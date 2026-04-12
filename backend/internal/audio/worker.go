// Package audio — bounded FFmpeg worker pool using a channel-based job queue.
package audio

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"
)

// ConversionMode controls how FFmpeg converts audio.
type ConversionMode int

const (
	ConversionDisabled ConversionMode = 0 // keep original
	ConversionEnabled  ConversionMode = 1 // aac 32k
	ConversionNorm     ConversionMode = 2 // aac 32k + acompressor
	ConversionLoudNorm ConversionMode = 3 // aac 32k + loudnorm
)

// ConversionJob represents a single FFmpeg conversion task.
type ConversionJob struct {
	InputPath  string
	OutputPath string
	Mode       ConversionMode
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.jobs <- job:
		return nil
	}
}

// runJob executes a single FFmpeg conversion using exec.CommandContext.
func runJob(ctx context.Context, job ConversionJob) {
	args := ffmpegArgs(job.InputPath, job.OutputPath, job.Mode)
	if len(args) == 0 {
		job.Done <- nil
		return
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		slog.Error("ffmpeg conversion failed", "input", job.InputPath, "output", job.OutputPath, "error", err)
		job.Done <- err
		return
	}
	job.Done <- nil
}

// ffmpegArgs returns the correct arg slice for the given mode.
// input and output must be absolute paths.
func ffmpegArgs(input, output string, mode ConversionMode) []string {
	switch mode {
	case ConversionEnabled:
		return []string{"ffmpeg", "-y", "-i", input, "-c:a", "aac", "-b:a", "32k", output}
	case ConversionNorm:
		return []string{"ffmpeg", "-y", "-i", input, "-c:a", "aac", "-b:a", "32k", "-af", "acompressor", output}
	case ConversionLoudNorm:
		return []string{"ffmpeg", "-y", "-i", input, "-c:a", "aac", "-b:a", "32k", "-af", "loudnorm", output}
	default:
		return nil
	}
}

// CheckFFmpeg logs a warning at startup if ffmpeg is not found on PATH.
func CheckFFmpeg() {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		slog.Warn("ffmpeg not found on PATH — audio conversion will fail if enabled")
	}
}
