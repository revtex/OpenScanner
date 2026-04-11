// Package audio handles FFmpeg-based audio conversion and storage.
package audio

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Processor handles storing uploaded audio files and queuing FFmpeg conversion.
type Processor struct {
	baseDir string
	pool    *WorkerPool
}

// NewProcessor creates a Processor for the given base directory.
func NewProcessor(baseDir string, pool *WorkerPool) *Processor {
	return &Processor{baseDir: baseDir, pool: pool}
}

// Store saves the uploaded file to disk under baseDir/<YYYY>/<MM>/<DD>/<filename>
// and returns the relative path (relative to baseDir).
// SECURITY: sanitises the filename via filepath.Base — strips directory components,
// rejects paths containing "..".
// If conversion is enabled, submits an FFmpeg job, waits for completion, removes
// the original, and returns the .m4a relative path.
func (p *Processor) Store(ctx context.Context, fh *multipart.FileHeader, mode ConversionMode) (string, error) {
	safeName := filepath.Base(fh.Filename)
	if safeName == "" || safeName == "." || safeName == ".." || strings.Contains(safeName, "..") {
		return "", fmt.Errorf("invalid filename")
	}

	now := time.Now().UTC()
	dayDir := filepath.Join(p.baseDir, now.Format("2006"), now.Format("01"), now.Format("02"))
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

	relPath, err := filepath.Rel(p.baseDir, destPath)
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
		// Non-fatal.
		_ = err
	}

	relOut, err := filepath.Rel(p.baseDir, outPath)
	if err != nil {
		return "", fmt.Errorf("compute relative output path: %w", err)
	}
	return relOut, nil
}
