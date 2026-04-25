package calls

import (
	"testing"
	"time"

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

// TestCallHandler_Limiter_CleansUpStaleEntries verifies that getLimiter
// sweeps stale entries once the map grows past the threshold (>100).
//
// The limiter uses time.Now() with no injection point; instead of
// manipulating the clock, we fast-forward by rewriting each seeded limiter's
// windowStart to a point well in the past. Then adding a 101st entry
// triggers the sweep, which should purge every stale entry except the
// freshly-inserted one.
func TestCallHandler_Limiter_CleansUpStaleEntries(t *testing.T) {
	h := New(&db.Queries{}, (*audio.Processor)(nil), (*ws.Hub)(nil), nil, nil)

	// Seed 101 entries with a stale windowStart (> 2*rateWindowDuration ago).
	staleStart := time.Now().Add(-3 * rateWindowDuration)
	for i := int64(1); i <= 101; i++ {
		h.limiters[i] = &apiKeyLimiter{
			windowStart: staleStart,
			rateLimit:   defaultCallRatePerMin,
		}
	}
	if got := len(h.limiters); got != 101 {
		t.Fatalf("seeded size = %d, want 101", got)
	}

	// Triggering getLimiter with a new key ID must sweep stale entries,
	// then insert the new one.
	const freshID = int64(9999)
	l := h.getLimiter(freshID, defaultCallRatePerMin)
	if l == nil {
		t.Fatal("getLimiter returned nil")
	}

	// After sweep, only the fresh entry survives (all seeded ones were stale).
	if got := len(h.limiters); got != 1 {
		t.Fatalf("post-sweep size = %d, want 1 (only fresh entry should remain)", got)
	}
	if _, ok := h.limiters[freshID]; !ok {
		t.Fatal("fresh limiter missing from map")
	}
}

func TestCallHandler_Limiter_KeepsFreshEntriesDuringSweep(t *testing.T) {
	h := New(&db.Queries{}, (*audio.Processor)(nil), (*ws.Hub)(nil), nil, nil)

	now := time.Now()
	// 90 stale + 15 fresh = 105 total → > 100 triggers sweep.
	for i := int64(1); i <= 90; i++ {
		h.limiters[i] = &apiKeyLimiter{
			windowStart: now.Add(-3 * rateWindowDuration),
			rateLimit:   defaultCallRatePerMin,
		}
	}
	for i := int64(91); i <= 105; i++ {
		h.limiters[i] = &apiKeyLimiter{
			windowStart: now, // fresh
			rateLimit:   defaultCallRatePerMin,
		}
	}

	_ = h.getLimiter(12345, defaultCallRatePerMin)

	// All 15 fresh + 1 new = 16 should survive.
	if got := len(h.limiters); got != 16 {
		t.Fatalf("post-sweep size = %d, want 16 (15 fresh + 1 new)", got)
	}
}
