package audio_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openscanner/openscanner/internal/audio"
)

// makeFileHeader builds a *multipart.FileHeader with the given filename and content.
func makeFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("audio", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fw.Write(content)
	w.Close()

	mr := multipart.NewReader(&body, w.Boundary())
	form, err := mr.ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["audio"][0]
}

// newTestProcessor creates a Processor backed by t.TempDir() with a
// background worker pool that is cancelled when the test ends.
func newTestProcessor(t *testing.T) (*audio.Processor, string) {
	t.Helper()
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	pool := audio.NewWorkerPool(ctx)
	return audio.NewProcessor(tmpDir, pool), tmpDir
}

// TestStore_PathSanitisation verifies that directory components and dangerous
// filename patterns are stripped or rejected before writing to disk.
func TestStore_PathSanitisation(t *testing.T) {
	fakeWAV := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")

	cases := []struct {
		name     string
		filename string
		wantErr  bool
		// wantBase is the expected filepath.Base of the returned relPath (only
		// checked when wantErr=false).
		wantBase string
	}{
		// Path traversal components are stripped by filepath.Base; no error is
		// returned, but the file lands safely inside the base directory.
		{"traversal stripped to basename", "../../../etc/passwd", false, "passwd"},
		{"subdir stripped to basename", "subdir/audio.wav", false, "audio.wav"},
		{"simple filename", "audio.wav", false, "audio.wav"},
		// The following filenames remain dangerous after filepath.Base and must
		// be rejected with an error.
		{"dotdot literal", "..", true, ""},
		{"dot literal", ".", true, ""},
		{"embedded dotdot in name", "bad..file.wav", true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proc, baseDir := newTestProcessor(t)
			fh := makeFileHeader(t, tc.filename, fakeWAV)

			relPath, err := proc.Store(context.Background(), fh, audio.ConversionDisabled)

			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for filename %q, got nil (relPath=%q)", tc.filename, relPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for filename %q: %v", tc.filename, err)
			}

			// relPath must never contain ".." (path traversal prevented).
			if strings.Contains(relPath, "..") {
				t.Errorf("relPath %q contains '..'; path traversal not prevented", relPath)
			}

			// The base filename component must match expected.
			if got := filepath.Base(relPath); got != tc.wantBase {
				t.Errorf("filepath.Base(relPath) = %q, want %q", got, tc.wantBase)
			}

			// File must actually exist on disk inside the base directory.
			absPath := filepath.Join(baseDir, relPath)
			if _, statErr := os.Stat(absPath); statErr != nil {
				t.Errorf("file not found at %s: %v", absPath, statErr)
			}
		})
	}
}

// TestStore_ConversionDisabled verifies that with ConversionDisabled the file
// is written to a dated subdirectory with its original extension preserved.
func TestStore_ConversionDisabled(t *testing.T) {
	proc, baseDir := newTestProcessor(t)
	content := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	fh := makeFileHeader(t, "recording.wav", content)

	relPath, err := proc.Store(context.Background(), fh, audio.ConversionDisabled)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	// relPath must be relative (not an absolute path).
	if filepath.IsAbs(relPath) {
		t.Errorf("relPath is absolute: %s", relPath)
	}

	// Original extension must be preserved (.wav, not .m4a).
	if ext := filepath.Ext(relPath); ext != ".wav" {
		t.Errorf("extension = %q, want .wav", ext)
	}

	// File must exist at baseDir/relPath.
	absPath := filepath.Join(baseDir, relPath)
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		t.Fatalf("stored file not found at %s: %v", absPath, statErr)
	}
	if info.Size() == 0 {
		t.Errorf("stored file is empty; content was not written")
	}
}

// TestFfmpegArgs verifies the exact argument slices produced for each
// ConversionMode, including structural invariants.
func TestFfmpegArgs(t *testing.T) {
	const in = "/tmp/in.wav"
	const out = "/tmp/out.m4a"

	cases := []struct {
		name     string
		mode     audio.ConversionMode
		wantNil  bool
		wantArgs []string
	}{
		{
			name:    "disabled returns nil",
			mode:    audio.ConversionDisabled,
			wantNil: true,
		},
		{
			name:     "enabled — plain aac 32k",
			mode:     audio.ConversionEnabled,
			wantArgs: []string{"ffmpeg", "-y", "-i", in, "-c:a", "aac", "-b:a", "32k", out},
		},
		{
			name:     "norm — aac 32k + acompressor",
			mode:     audio.ConversionNorm,
			wantArgs: []string{"ffmpeg", "-y", "-i", in, "-c:a", "aac", "-b:a", "32k", "-af", "acompressor", out},
		},
		{
			name:     "loudnorm — aac 32k + loudnorm filter",
			mode:     audio.ConversionLoudNorm,
			wantArgs: []string{"ffmpeg", "-y", "-i", in, "-c:a", "aac", "-b:a", "32k", "-af", "loudnorm", out},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := audio.FfmpegArgs(in, out, tc.mode)

			if tc.wantNil {
				if got != nil {
					t.Errorf("FfmpegArgs mode=%d: expected nil, got %v", tc.mode, got)
				}
				return
			}

			if len(got) != len(tc.wantArgs) {
				t.Fatalf("len(args) = %d, want %d\n  got:  %v\n  want: %v",
					len(got), len(tc.wantArgs), got, tc.wantArgs)
			}
			for i, want := range tc.wantArgs {
				if got[i] != want {
					t.Errorf("args[%d] = %q, want %q", i, got[i], want)
				}
			}

			// Structural invariants regardless of mode.
			if len(got) < 2 {
				t.Fatal("args slice too short")
			}
			if got[1] != "-y" {
				t.Errorf("args[1] = %q, want \"-y\" (overwrite flag)", got[1])
			}
			if got[len(got)-1] != out {
				t.Errorf("last arg = %q, want output path %q", got[len(got)-1], out)
			}
		})
	}
}
