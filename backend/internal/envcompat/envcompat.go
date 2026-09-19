// Package envcompat reads Squelch environment variables while still
// honouring the pre-rename OPENSCANNER_* names.
//
// Squelch was previously called OpenScanner, and its configuration was
// read from OPENSCANNER_*. Renaming the prefix outright would silently
// drop an operator's entire configuration on upgrade — the process would
// start with defaults rather than fail — so the old names keep working
// for one minor cycle and every use is reported at startup.
//
// Precedence is SQUELCH_* over OPENSCANNER_*, so an operator can set the
// new name on a host that still exports the old one and know which wins.
package envcompat

import (
	"os"
	"sort"
	"sync"
)

const (
	// Prefix is the current environment variable prefix.
	Prefix = "SQUELCH_"
	// LegacyPrefix is the pre-rename prefix, honoured with a warning.
	LegacyPrefix = "OPENSCANNER_"
)

var (
	mu   sync.Mutex
	used = map[string]string{}
)

// Lookup returns the value of SQUELCH_<suffix>, falling back to
// OPENSCANNER_<suffix>. An empty variable counts as unset, matching how
// the configuration layer has always treated these.
func Lookup(suffix string) string {
	if v := os.Getenv(Prefix + suffix); v != "" {
		return v
	}
	v := os.Getenv(LegacyPrefix + suffix)
	if v == "" {
		return ""
	}
	mu.Lock()
	used[LegacyPrefix+suffix] = Prefix + suffix
	mu.Unlock()
	return v
}

// Used lists the legacy variables that have been honoured so far, as
// "OPENSCANNER_X -> SQUELCH_X" pairs sorted for stable output. Callers
// use it to warn once at startup rather than per variable.
func Used() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(used))
	for old, new := range used {
		out = append(out, old+" -> "+new)
	}
	sort.Strings(out)
	return out
}

// Reset clears the recorded legacy usage. For tests.
func Reset() {
	mu.Lock()
	used = map[string]string{}
	mu.Unlock()
}
