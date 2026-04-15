package audio

import (
	"context"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeDuration uses ffprobe to extract the duration of an audio file in seconds.
// Returns 0 if ffprobe is unavailable, the file cannot be probed, or the
// duration is not positive.
func ProbeDuration(ctx context.Context, filePath string) int64 {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int64(math.Round(f))
}
