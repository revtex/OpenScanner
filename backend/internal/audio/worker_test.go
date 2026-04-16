package audio_test

import (
	"strings"
	"testing"

	"github.com/openscanner/openscanner/internal/audio"
)

func TestFfmpegArgs_Disabled(t *testing.T) {
	for _, preset := range []audio.EncodingPreset{audio.PresetAACLC32k, audio.PresetHEAAC8k, ""} {
		args := audio.FfmpegArgs("/in.wav", "/out.m4a", audio.ConversionDisabled, preset)
		if args != nil {
			t.Errorf("preset %q: expected nil args for ConversionDisabled, got %v", preset, args)
		}
	}
}

func TestFfmpegArgs_DefaultPreset(t *testing.T) {
	// Empty preset should fall back to aac_lc_32k behaviour.
	args := audio.FfmpegArgs("/in.wav", "/out.m4a", audio.ConversionEnabled, "")
	if !containsAll(args, "aac", "32k") {
		t.Errorf("empty preset: expected aac/32k in args, got %v", args)
	}
}

func TestFfmpegArgs_AACLC_Presets(t *testing.T) {
	cases := []struct {
		preset audio.EncodingPreset
		bitrate string
	}{
		{audio.PresetAACLC32k, "32k"},
		{audio.PresetAACLC24k, "24k"},
		{audio.PresetAACLC16k, "16k"},
	}
	for _, tc := range cases {
		args := audio.FfmpegArgs("/in.wav", "/out.m4a", audio.ConversionEnabled, tc.preset)
		if !containsAll(args, "aac", tc.bitrate) {
			t.Errorf("preset %q: expected aac/%s in args, got %v", tc.preset, tc.bitrate, args)
		}
		if containsAny(args, "libfdk_aac", "aac_he") {
			t.Errorf("preset %q: unexpected HE-AAC args in LC preset: %v", tc.preset, args)
		}
	}
}

func TestFfmpegArgs_HEAAC_Presets(t *testing.T) {
	cases := []struct {
		preset  audio.EncodingPreset
		bitrate string
	}{
		{audio.PresetHEAAC12k, "12k"},
		{audio.PresetHEAAC8k, "8k"},
	}
	for _, tc := range cases {
		args := audio.FfmpegArgs("/in.wav", "/out.m4a", audio.ConversionEnabled, tc.preset)
		if !containsAll(args, "libfdk_aac", "aac_he", tc.bitrate) {
			t.Errorf("preset %q: expected libfdk_aac/aac_he/%s in args, got %v", tc.preset, tc.bitrate, args)
		}
	}
}

func TestFfmpegArgs_NormModes(t *testing.T) {
	modes := []struct {
		mode   audio.ConversionMode
		filter string
	}{
		{audio.ConversionNorm, "acompressor"},
		{audio.ConversionLoudNorm, "loudnorm"},
	}
	for _, tc := range modes {
		args := audio.FfmpegArgs("/in.wav", "/out.m4a", tc.mode, audio.PresetAACLC32k)
		if !containsAll(args, "-af", tc.filter) {
			t.Errorf("mode %d: expected -af %s in args, got %v", tc.mode, tc.filter, args)
		}
	}
}

func TestFfmpegArgs_OutputIsLast(t *testing.T) {
	out := "/recordings/out.m4a"
	args := audio.FfmpegArgs("/in.wav", out, audio.ConversionEnabled, audio.PresetAACLC32k)
	if len(args) == 0 || args[len(args)-1] != out {
		t.Errorf("expected output path as last arg, got %v", args)
	}
}

func TestIsValidEncodingPreset(t *testing.T) {
	valid := []string{"aac_lc_32k", "aac_lc_24k", "aac_lc_16k", "he_aac_12k", "he_aac_8k"}
	for _, v := range valid {
		if !audio.IsValidEncodingPreset(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"", "aac", "he_aac_v2", "mp3_128k", "AACLC32K"}
	for _, v := range invalid {
		if audio.IsValidEncodingPreset(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestParseEncodingPreset_Fallback(t *testing.T) {
	if got := audio.ParseEncodingPreset(""); got != audio.PresetAACLC32k {
		t.Errorf("empty string should fall back to PresetAACLC32k, got %q", got)
	}
	if got := audio.ParseEncodingPreset("garbage"); got != audio.PresetAACLC32k {
		t.Errorf("unknown string should fall back to PresetAACLC32k, got %q", got)
	}
	if got := audio.ParseEncodingPreset("he_aac_8k"); got != audio.PresetHEAAC8k {
		t.Errorf("expected PresetHEAAC8k, got %q", got)
	}
}

// containsAll returns true if all needles appear in haystack.
func containsAll(haystack []string, needles ...string) bool {
	joined := strings.Join(haystack, " ")
	for _, n := range needles {
		if !strings.Contains(joined, n) {
			return false
		}
	}
	return true
}

// containsAny returns true if any needle appears in haystack.
func containsAny(haystack []string, needles ...string) bool {
	joined := strings.Join(haystack, " ")
	for _, n := range needles {
		if strings.Contains(joined, n) {
			return true
		}
	}
	return false
}
