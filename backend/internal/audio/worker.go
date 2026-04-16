// Package audio — bounded FFmpeg worker pool using a channel-based job queue.
package audio

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"
)

// ConversionMode controls how FFmpeg processes audio (normalization filter).
type ConversionMode int

const (
	ConversionDisabled ConversionMode = 0 // keep original
	ConversionEnabled  ConversionMode = 1 // encode only, no filter
	ConversionNorm     ConversionMode = 2 // encode + acompressor
	ConversionLoudNorm ConversionMode = 3 // encode + loudnorm
)

// EncodingPreset selects the codec/bitrate combination used during conversion.
type EncodingPreset string

const (
	PresetAACLC32k  EncodingPreset = "aac_lc_32k"  // AAC-LC 32 kbps (default)
	PresetAACLC24k  EncodingPreset = "aac_lc_24k"  // AAC-LC 24 kbps
	PresetAACLC16k  EncodingPreset = "aac_lc_16k"  // AAC-LC 16 kbps
	PresetHEAAC12k  EncodingPreset = "he_aac_12k"  // HE-AAC 12 kbps
	PresetHEAAC8k   EncodingPreset = "he_aac_8k"   // HE-AAC  8 kbps
)

// validPresets is the set of accepted EncodingPreset values.
var validPresets = map[EncodingPreset]bool{
	PresetAACLC32k: true,
	PresetAACLC24k: true,
	PresetAACLC16k: true,
	PresetHEAAC12k: true,
	PresetHEAAC8k:  true,
}

// IsValidEncodingPreset reports whether s is a known preset value.
func IsValidEncodingPreset(s string) bool {
	return validPresets[EncodingPreset(s)]
}

// ParseEncodingPreset returns the preset for s, falling back to the default
// if s is empty or unrecognised.
func ParseEncodingPreset(s string) EncodingPreset {
	p := EncodingPreset(s)
	if validPresets[p] {
		return p
	}
	return PresetAACLC32k
}

// presetCodecArgs returns the FFmpeg codec/bitrate/channel args for the preset.
func presetCodecArgs(preset EncodingPreset) []string {
	switch preset {
	case PresetAACLC24k:
		return []string{"-c:a", "aac", "-b:a", "24k", "-ac", "1"}
	case PresetAACLC16k:
		return []string{"-c:a", "aac", "-b:a", "16k", "-ac", "1"}
	case PresetHEAAC12k:
		return []string{"-c:a", "libfdk_aac", "-profile:a", "aac_he", "-b:a", "12k", "-ac", "1"}
	case PresetHEAAC8k:
		return []string{"-c:a", "libfdk_aac", "-profile:a", "aac_he", "-b:a", "8k", "-ac", "1"}
	default: // PresetAACLC32k
		return []string{"-c:a", "aac", "-b:a", "32k", "-ac", "1"}
	}
}

// ConversionJob represents a single FFmpeg conversion task.
type ConversionJob struct {
	InputPath  string
	OutputPath string
	Mode       ConversionMode
	Preset     EncodingPreset
	Done       chan error // written when job completes
}

// WorkerPool runs FFmpeg conversion jobs with bounded concurrency.
type WorkerPool struct {
	jobs chan ConversionJob
}

// NewWorkerPool creates a pool with runtime.NumCPU() workers (minimum 1).
// Workers stop when ctx is cancelled.
func NewWorkerPool(ctx context.Context) *WorkerPool {
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	p := &WorkerPool{
		jobs: make(chan ConversionJob, numWorkers*4),
	}
	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-p.jobs:
					if !ok {
						return
					}
					runJob(ctx, job)
				}
			}
		}()
	}
	return p
}

// Submit enqueues a conversion job. Returns ctx.Err() if the context is
// cancelled before the job can be queued. Blocks only when the buffer is full.
func (p *WorkerPool) Submit(ctx context.Context, job ConversionJob) error {
	slog.Debug("audio: submitting conversion job", "input", job.InputPath, "output", job.OutputPath, "mode", job.Mode, "preset", job.Preset)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.jobs <- job:
		return nil
	}
}

// runJob executes a single FFmpeg conversion using exec.CommandContext.
func runJob(ctx context.Context, job ConversionJob) {
	args := ffmpegArgs(job.InputPath, job.OutputPath, job.Mode, job.Preset)
	if len(args) == 0 {
		job.Done <- nil
		return
	}
	slog.Debug("audio: starting ffmpeg", "input", job.InputPath, "output", job.OutputPath, "mode", job.Mode, "preset", job.Preset)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		slog.Error("ffmpeg conversion failed", "input", job.InputPath, "output", job.OutputPath, "preset", job.Preset, "error", err)
		job.Done <- err
		return
	}
	slog.Debug("audio: ffmpeg completed", "output", job.OutputPath)
	job.Done <- nil
}

// ffmpegArgs returns the correct arg slice for the given mode and preset.
// input and output must be absolute paths.
func ffmpegArgs(input, output string, mode ConversionMode, preset EncodingPreset) []string {
	if mode == ConversionDisabled {
		return nil
	}
	// Resolve preset; fall back to default if unset.
	if preset == "" {
		preset = PresetAACLC32k
	}
	codec := presetCodecArgs(preset)

	base := append([]string{"ffmpeg", "-y", "-i", input}, codec...)

	switch mode {
	case ConversionNorm:
		base = append(base, "-af", "acompressor")
	case ConversionLoudNorm:
		base = append(base, "-af", "loudnorm")
	}

	return append(base, output)
}

// CheckFFmpeg reports whether ffmpeg is available on PATH.
func CheckFFmpeg() bool {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg not found on PATH — audio conversion will fail if enabled")
		return false
	}
	return true
}
