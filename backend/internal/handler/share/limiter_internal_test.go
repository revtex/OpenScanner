package share

import (
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// TestShareLimiter_CleansUpStaleEntries verifies that getShareLimiter
// sweeps stale entries once the map grows past the threshold (>100).
func TestShareLimiter_CleansUpStaleEntries(t *testing.T) {
	h := New(&db.Queries{}, nil)

	staleStart := time.Now().Add(-3 * rateWindowDuration)
	for i := int64(1); i <= 101; i++ {
		h.limiters[i] = &shareLimiter{
			windowStart: staleStart,
			rateLimit:   shareRatePerMin,
		}
	}

	_ = h.getShareLimiter(9999)

	if got := len(h.limiters); got != 1 {
		t.Fatalf("post-sweep limiters size = %d, want 1", got)
	}
}
