// Package audio handles FFmpeg-based audio conversion and storage.
package audio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Processor handles storing uploaded audio files and queuing FFmpeg conversion.
type Processor struct {
	recordingsDir string
	pool          *WorkerPool
}

// NewProcessor creates a Processor for the given recordings directory.
func NewProcessor(recordingsDir string, pool *WorkerPool) *Processor {
	return &Processor{recordingsDir: recordingsDir, pool: pool}
}

// RecordingsDir returns the directory used for audio file storage.
func (p *Processor) RecordingsDir() string {
	return p.recordingsDir
}

// Store saves the uploaded file to disk under recordingsDir/<YYYY>/<MM>/<DD>/<filename>
// and returns the relative path (relative to recordingsDir).
// SECURITY: sanitises the filename via filepath.Base — strips directory components,
// rejects paths containing "..".
// If conversion is enabled, submits an FFmpeg job, waits for completion, removes
// the original, and returns the .m4a relative path.
func (p *Processor) Store(ctx context.Context, fh *multipart.FileHeader, mode ConversionMode, preset EncodingPreset) (string, error) {
	slog.Debug("audio: storing uploaded file", "filename", fh.Filename, "size", fh.Size, "mode", mode, "preset", preset)
	safeName := filepath.Base(fh.Filename)
	if safeName == "" || safeName == "." || safeName == ".." || strings.Contains(safeName, "..") {
		return "", fmt.Errorf("invalid filename")
	}

	now := time.Now().UTC()
	dayDir := filepath.Join(p.recordingsDir, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dayDir, 0755); err != nil {
		return "", fmt.Errorf("create audio dir: %w", err)
	}

	destPath := filepath.Join(dayDir, safeName)

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(destPath) //nolint:errcheck
		return "", fmt.Errorf("write audio file: %w", err)
	}
	dst.Close()
	slog.Debug("audio: file written", "path", destPath, "size_bytes", fh.Size)

	relPath, err := filepath.Rel(p.recordingsDir, destPath)
	if err != nil {
		return "", fmt.Errorf("compute relative path: %w", err)
	}

	if mode == ConversionDisabled {
		return relPath, nil
	}

	// Validate mode — treat unknown values as disabled to avoid silent data
	// loss (ffmpegArgs returns nil for unknown modes, so no output file would
	// be created but the original would be deleted).
	if mode != ConversionEnabled && mode != ConversionNorm && mode != ConversionLoudNorm {
		return relPath, nil
	}

	// Build .m4a output path.
	ext := filepath.Ext(safeName)
	outName := strings.TrimSuffix(safeName, ext) + ".m4a"
	outPath := filepath.Join(dayDir, outName)

	done := make(chan error, 1)
	if err := p.pool.Submit(ctx, ConversionJob{
		InputPath:  destPath,
		OutputPath: outPath,
		Mode:       mode,
		Preset:     preset,
		Done:       done,
	}); err != nil {
		os.Remove(destPath) //nolint:errcheck
		return "", fmt.Errorf("submit conversion job: %w", err)
	}

	select {
	case <-ctx.Done():
		os.Remove(destPath) //nolint:errcheck
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			os.Remove(destPath) //nolint:errcheck
			return "", fmt.Errorf("audio conversion: %w", err)
		}
	}

	// Remove original after successful conversion.
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("audio: failed to remove original after conversion", "path", destPath, "error", err)
	}

	relOut, err := filepath.Rel(p.recordingsDir, outPath)
	if err != nil {
		return "", fmt.Errorf("compute relative output path: %w", err)
	}
	slog.Debug("audio: conversion complete", "input", safeName, "output", relOut)
	return relOut, nil
}

// StoreFile stores a local file (by path) identically to Store, but reads
// directly from the filesystem rather than from a multipart upload.
// SECURITY: the filename is sanitised via filepath.Base — strips directory
// components and rejects names containing "..".
func (p *Processor) StoreFile(ctx context.Context, srcPath string, mode ConversionMode, preset EncodingPreset) (string, error) {
	slog.Debug("audio: storing local file", "src", srcPath, "mode", mode, "preset", preset)
	// filepath.Base strips all directory components; the == ".." guard catches
	// the only remaining traversal case.  No further Contains check is needed.
	safeName := filepath.Base(srcPath)
	if safeName == "" || safeName == "." || safeName == ".." {
		return "", fmt.Errorf("invalid filename")
	}

	now := time.Now().UTC()
	dayDir := filepath.Join(p.recordingsDir, now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dayDir, 0755); err != nil {
		return "", fmt.Errorf("create audio dir: %w", err)
	}

	destPath := filepath.Join(dayDir, safeName)
	if err := copyFile(srcPath, destPath); err != nil {
		return "", fmt.Errorf("copy audio file: %w", err)
	}
	slog.Debug("audio: file written", "path", destPath)

	relPath, err := filepath.Rel(p.recordingsDir, destPath)
	if err != nil {
		return "", fmt.Errorf("compute relative path: %w", err)
	}

	if mode == ConversionDisabled {
		return relPath, nil
	}

	if mode != ConversionEnabled && mode != ConversionNorm && mode != ConversionLoudNorm {
		return relPath, nil
	}

	ext := filepath.Ext(safeName)
	outName := strings.TrimSuffix(safeName, ext) + ".m4a"
	outPath := filepath.Join(dayDir, outName)

	done := make(chan error, 1)
	if err := p.pool.Submit(ctx, ConversionJob{
		InputPath:  destPath,
		OutputPath: outPath,
		Mode:       mode,
		Preset:     preset,
		Done:       done,
	}); err != nil {
		os.Remove(destPath) //nolint:errcheck
		return "", fmt.Errorf("submit conversion job: %w", err)
	}

	select {
	case <-ctx.Done():
		os.Remove(destPath) //nolint:errcheck
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			os.Remove(destPath) //nolint:errcheck
			return "", fmt.Errorf("audio conversion: %w", err)
		}
	}

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("audio: failed to remove original after conversion", "path", destPath, "error", err)
	}

	relOut, err := filepath.Rel(p.recordingsDir, outPath)
	if err != nil {
		return "", fmt.Errorf("compute relative output path: %w", err)
	}
	slog.Debug("audio: conversion complete", "input", safeName, "output", relOut)
	return relOut, nil
}

// copyFile copies the file at src to dst, creating dst if it does not exist.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst) //nolint:errcheck
		return err
	}
	return out.Close()
}
