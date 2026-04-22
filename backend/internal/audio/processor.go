// Package audio handles FFmpeg-based audio conversion and storage.
package audio

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
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

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	// Create the destination atomically with O_EXCL so colliding filenames
	// don't silently overwrite an existing recording. If the name is taken,
	// append a short random suffix and retry up to 5 times.
	destPath, dst, err := createUniqueFile(dayDir, safeName)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	// Update safeName to whatever filename we actually wrote so the
	// downstream conversion step uses the unique base name.
	safeName = filepath.Base(destPath)
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

	// Build output path with appropriate extension for the preset.
	ext := filepath.Ext(safeName)
	outExt := OutputExt(preset)
	outName := strings.TrimSuffix(safeName, ext) + outExt
	outPath := filepath.Join(dayDir, outName)

	// When the input already has the target extension, FFmpeg would try to
	// read and write the same file.  Write to a temp path then rename.
	sameFile := strings.EqualFold(ext, outExt)
	ffmpegOut := outPath
	if sameFile {
		ffmpegOut = outPath + ".tmp" + outExt
	}

	done := make(chan error, 1)
	if err := p.pool.Submit(ctx, ConversionJob{
		InputPath:  destPath,
		OutputPath: ffmpegOut,
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
		if sameFile {
			os.Remove(ffmpegOut) //nolint:errcheck
		}
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			os.Remove(destPath) //nolint:errcheck
			if sameFile {
				os.Remove(ffmpegOut) //nolint:errcheck
			}
			return "", fmt.Errorf("audio conversion: %w", err)
		}
	}

	// Rename temp file to final output path when input/output extensions match.
	if sameFile {
		if err := os.Rename(ffmpegOut, outPath); err != nil {
			os.Remove(ffmpegOut) //nolint:errcheck
			return "", fmt.Errorf("rename converted file: %w", err)
		}
	}

	// Remove original after successful conversion (skip if same path — already replaced by rename).
	if !sameFile {
		if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("audio: failed to remove original after conversion", "path", destPath, "error", err)
		}
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

	destPath, dst, err := createUniqueFile(dayDir, safeName)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	safeName = filepath.Base(destPath)
	if err := copyFileTo(srcPath, dst); err != nil {
		dst.Close()
		os.Remove(destPath) //nolint:errcheck
		return "", fmt.Errorf("copy audio file: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(destPath) //nolint:errcheck
		return "", fmt.Errorf("close audio file: %w", err)
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
	outExt := OutputExt(preset)
	outName := strings.TrimSuffix(safeName, ext) + outExt
	outPath := filepath.Join(dayDir, outName)

	// When the input already has the target extension, FFmpeg would try to
	// read and write the same file.  Write to a temp path then rename.
	sameFile := strings.EqualFold(ext, outExt)
	ffmpegOut := outPath
	if sameFile {
		ffmpegOut = outPath + ".tmp" + outExt
	}

	done := make(chan error, 1)
	if err := p.pool.Submit(ctx, ConversionJob{
		InputPath:  destPath,
		OutputPath: ffmpegOut,
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
		if sameFile {
			os.Remove(ffmpegOut) //nolint:errcheck
		}
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			os.Remove(destPath) //nolint:errcheck
			if sameFile {
				os.Remove(ffmpegOut) //nolint:errcheck
			}
			return "", fmt.Errorf("audio conversion: %w", err)
		}
	}

	if sameFile {
		if err := os.Rename(ffmpegOut, outPath); err != nil {
			os.Remove(ffmpegOut) //nolint:errcheck
			return "", fmt.Errorf("rename converted file: %w", err)
		}
	}

	if !sameFile {
		if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("audio: failed to remove original after conversion", "path", destPath, "error", err)
		}
	}

	relOut, err := filepath.Rel(p.recordingsDir, outPath)
	if err != nil {
		return "", fmt.Errorf("compute relative output path: %w", err)
	}
	slog.Debug("audio: conversion complete", "input", safeName, "output", relOut)
	return relOut, nil
}

// copyFileTo streams the file at src into an already-open destination handle.
// The caller is responsible for closing dst.
func copyFileTo(src string, dst *os.File) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(dst, in)
	return err
}

// createUniqueFile creates a new file under dir named filename, failing
// atomically (O_EXCL) if the name already exists. On collision it appends
// a short random suffix before the extension and retries up to 5 times.
// Returns the final path and an open write handle on success.
func createUniqueFile(dir, filename string) (string, *os.File, error) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	candidate := filepath.Join(dir, filename)
	const flags = os.O_CREATE | os.O_EXCL | os.O_WRONLY
	for attempt := 0; attempt < 5; attempt++ {
		f, err := os.OpenFile(candidate, flags, 0644)
		if err == nil {
			return candidate, f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
		suffix, sErr := randomSuffix()
		if sErr != nil {
			return "", nil, sErr
		}
		candidate = filepath.Join(dir, base+"-"+suffix+ext)
	}
	return "", nil, fmt.Errorf("audio: exhausted unique filename attempts for %q", filename)
}

// randomSuffix returns 6 lowercase hex characters from crypto/rand.
func randomSuffix() (string, error) {
	var b [3]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
