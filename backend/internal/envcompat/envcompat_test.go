package envcompat_test

import (
	"strings"
	"testing"

	"github.com/revtex/squelch/internal/envcompat"
)

func TestPrefersCurrentName(t *testing.T) {
	envcompat.Reset()
	t.Setenv("SQUELCH_DB_FILE", "/new.db")
	t.Setenv("OPENSCANNER_DB_FILE", "/old.db")

	if got := envcompat.Lookup("DB_FILE"); got != "/new.db" {
		t.Errorf("got %q, want the SQUELCH_ value", got)
	}
	if len(envcompat.Used()) != 0 {
		t.Error("the legacy name was not used, so nothing should be reported")
	}
}

func TestFallsBackToLegacyName(t *testing.T) {
	envcompat.Reset()
	t.Setenv("OPENSCANNER_DB_FILE", "/old.db")

	// Dropping this silently would start the server on defaults and look
	// like the operator's entire configuration had vanished.
	if got := envcompat.Lookup("DB_FILE"); got != "/old.db" {
		t.Errorf("got %q, want the OPENSCANNER_ value", got)
	}
	used := envcompat.Used()
	if len(used) != 1 || !strings.Contains(used[0], "OPENSCANNER_DB_FILE") {
		t.Errorf("legacy use must be reported for the startup warning, got %v", used)
	}
}

func TestUnsetIsEmpty(t *testing.T) {
	envcompat.Reset()
	if got := envcompat.Lookup("NOT_SET_ANYWHERE"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if len(envcompat.Used()) != 0 {
		t.Error("nothing was honoured, so nothing should be reported")
	}
}

func TestEmptyCurrentFallsThrough(t *testing.T) {
	envcompat.Reset()
	t.Setenv("SQUELCH_LISTEN", "")
	t.Setenv("OPENSCANNER_LISTEN", ":3022")

	if got := envcompat.Lookup("LISTEN"); got != ":3022" {
		t.Errorf("got %q; an empty SQUELCH_ value must not mask the legacy one", got)
	}
}
