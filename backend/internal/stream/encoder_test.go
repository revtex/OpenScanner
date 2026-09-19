package stream

import (
	"context"
	"os/exec"
	"testing"
)

// requireFFmpeg skips when the host has no encoder. The container that runs
// OpenScanner always has one, so this exercises the real LAME output in CI
// and in the image while staying green on a bare dev box.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
}

func TestCanonicalArgs_DisablesTheBitReservoir(t *testing.T) {
	args := canonicalArgs("-i", "in.m4a")
	var sawReservoir bool
	for i, a := range args {
		if a == "-reservoir" && i+1 < len(args) && args[i+1] == "0" {
			sawReservoir = true
		}
	}
	// Without this every frame depends on its predecessors, and splicing
	// calls into the silence filler produces artefacts at each join.
	if !sawReservoir {
		t.Error("canonical args must pass -reservoir 0")
	}
	if args[len(args)-1] != "-" {
		t.Error("canonical args must write to stdout")
	}
}

func TestEncodeSilence_ProducesParsableCanonicalFrames(t *testing.T) {
	requireFFmpeg(t)

	frames, err := encodeSilence(context.Background(), 2)
	if err != nil {
		t.Fatalf("encodeSilence: %v", err)
	}
	if len(frames) == 0 {
		t.Fatal("no frames produced")
	}

	// Every frame must parse and share the canonical parameters, because
	// concatenation is only valid when they agree.
	for i, f := range frames {
		hdr, ok := parseFrameHeader(f)
		if !ok {
			t.Fatalf("frame %d did not parse", i)
		}
		if hdr.sampleRate != streamSampleRate || hdr.channels != streamChannels {
			t.Fatalf("frame %d: rate=%d channels=%d, want %d/%d",
				i, hdr.sampleRate, hdr.channels, streamSampleRate, streamChannels)
		}
		if hdr.length != len(f) {
			t.Fatalf("frame %d: header says %d bytes, slice is %d", i, hdr.length, len(f))
		}
	}

	// ~2 seconds of audio at 576 samples per frame.
	samples, rate := frameDuration(frames[0])
	seconds := float64(len(frames)*samples) / float64(rate)
	if seconds < 1.5 || seconds > 2.5 {
		t.Errorf("silence covers %.2fs, want ~2s", seconds)
	}
}
