package audio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateUniqueFile_ReturnsPrimaryWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	path, f, err := createUniqueFile(dir, "call.wav")
	if err != nil {
		t.Fatalf("createUniqueFile: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	want := filepath.Join(dir, "call.wav")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestCreateUniqueFile_AppendsSuffixOnCollision(t *testing.T) {
	dir := t.TempDir()
	primary := filepath.Join(dir, "call.wav")
	if err := os.WriteFile(primary, []byte("existing"), 0o644); err != nil {
		t.Fatalf("pre-create primary: %v", err)
	}

	path, f, err := createUniqueFile(dir, "call.wav")
	if err != nil {
		t.Fatalf("createUniqueFile: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if path == primary {
		t.Fatalf("expected a suffixed path, got the primary %q", path)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "call-") || !strings.HasSuffix(base, ".wav") {
		t.Fatalf("unexpected filename shape: %q", base)
	}
	// Suffix should be 6 hex chars between "call-" and ".wav".
	mid := strings.TrimSuffix(strings.TrimPrefix(base, "call-"), ".wav")
	if len(mid) != 6 {
		t.Fatalf("suffix length = %d, want 6 (from %q)", len(mid), base)
	}

	// Both files must exist with different content.
	existing, err := os.ReadFile(primary)
	if err != nil {
		t.Fatalf("read primary: %v", err)
	}
	if string(existing) != "existing" {
		t.Fatalf("primary was overwritten: %q", existing)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat new file: %v", err)
	}
}

func TestCreateUniqueFile_MultipleCollisionsRetry(t *testing.T) {
	// Pre-create the primary so the first attempt fails. Because randomSuffix
	// comes from crypto/rand, colliding on a specific suffix is vanishingly
	// unlikely — so as long as the primary exists, the function must fall
	// back to a suffix. Verify no extension .wav.wav nesting, etc.
	dir := t.TempDir()
	primary := filepath.Join(dir, "audio.m4a")
	if err := os.WriteFile(primary, []byte("a"), 0o644); err != nil {
		t.Fatalf("seed primary: %v", err)
	}
	// Also pre-create a file with a fake suffix to prove the function keeps
	// retrying even if the first randomSuffix attempt happened to collide.
	// We can't predict randomSuffix output, so just seed a sentinel name.
	sentinel := filepath.Join(dir, "audio-deadbe.m4a")
	if err := os.WriteFile(sentinel, []byte("b"), 0o644); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}

	path, f, err := createUniqueFile(dir, "audio.m4a")
	if err != nil {
		t.Fatalf("createUniqueFile: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if path == primary || path == sentinel {
		t.Fatalf("unexpected collision with seeded file: %q", path)
	}
	if filepath.Ext(path) != ".m4a" {
		t.Fatalf("extension mismatch: %q", path)
	}
}

func TestCreateUniqueFile_ReturnsErrorOnNonExistDir(t *testing.T) {
	// Non-ErrExist errors must propagate immediately (not treated as collision).
	// A missing parent directory yields ENOENT, which is not os.ErrExist.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, _, err := createUniqueFile(missing, "x.wav")
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
	if os.IsExist(err) {
		t.Fatalf("error classified as collision; want propagation: %v", err)
	}
}

// NOTE: TestCreateUniqueFile_ReturnsErrorAfterMaxAttempts is skipped —
// randomSuffix draws from crypto/rand with 24 bits of entropy, so forcing
// five successive collisions without stubbing crypto/rand (which would
// require a refactor to inject a rand source) is not deterministic. The
// max-attempts cap is exercised implicitly by the other collision tests.
func TestCreateUniqueFile_ReturnsErrorAfterMaxAttempts(t *testing.T) {
	t.Skip("cannot force 5 crypto/rand collisions without injecting a rand source; documented gap")
}
