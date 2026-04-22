package downstream

import (
	"os"
	"testing"
)

// TestMain opts into the safehttp allow-internal override so httptest
// loopback servers are reachable during tests.
func TestMain(m *testing.M) {
	_ = os.Setenv("OPENSCANNER_ALLOW_INTERNAL_HTTP", "1")
	os.Exit(m.Run())
}
