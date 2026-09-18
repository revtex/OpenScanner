package stream

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// canonicalArgs builds the FFmpeg argument list that turns any input into
// the canonical stream format on stdout.
//
// `-reservoir 0` is load-bearing, not a tuning knob: it disables LAME's bit
// reservoir so every frame stands alone and can be spliced between silence
// and other calls. `-write_xing 0` and `-id3v2_version 0` suppress the
// header frame and tags, which have no place mid-stream.
func canonicalArgs(input ...string) []string {
	args := []string{"ffmpeg", "-hide_banner", "-loglevel", "error"}
	args = append(args, input...)
	return append(args,
		"-vn",
		"-ac", fmt.Sprintf("%d", streamChannels),
		"-ar", fmt.Sprintf("%d", streamSampleRate),
		"-c:a", "libmp3lame",
		"-b:a", streamBitrate,
		"-reservoir", "0",
		"-write_xing", "0",
		"-id3v2_version", "0",
		"-f", "mp3",
		"-",
	)
}

// runFFmpeg executes FFmpeg with the given argument slice and returns its
// stdout. Arguments are always passed as a slice — never a shell string.
func runFFmpeg(ctx context.Context, args []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // G204: args are assembled by canonicalArgs from server-controlled values
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// encodeFile transcodes a stored call recording into canonical stream
// frames.
func encodeFile(ctx context.Context, path string) ([][]byte, error) {
	out, err := runFFmpeg(ctx, canonicalArgs("-i", path))
	if err != nil {
		return nil, err
	}
	frames := splitFrames(out)
	if len(frames) == 0 {
		return nil, fmt.Errorf("stream: no frames decoded from %q", path)
	}
	return frames, nil
}

// encodeSilence produces a run of silent frames used as the stream's idle
// filler. Generated once at startup and then cycled, so the cost is paid a
// single time regardless of how long a listener stays connected.
func encodeSilence(ctx context.Context, seconds int) ([][]byte, error) {
	src := fmt.Sprintf("anullsrc=r=%d:cl=mono", streamSampleRate)
	out, err := runFFmpeg(ctx, canonicalArgs(
		"-f", "lavfi", "-i", src, "-t", fmt.Sprintf("%d", seconds),
	))
	if err != nil {
		return nil, err
	}
	frames := splitFrames(out)
	if len(frames) == 0 {
		return nil, fmt.Errorf("stream: no silence frames produced")
	}
	return frames, nil
}
