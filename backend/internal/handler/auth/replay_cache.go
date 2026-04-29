package auth

import (
	"sync"
	"time"
)

// RefreshReplayGrace is the window during which a recently-rotated refresh
// token may be presented again (e.g. by a parallel tab, a service-worker
// retry, or a reload mid-rotation) and receive the same successor tokens
// that were issued on the original rotation, instead of triggering family
// revocation.
//
// This implements the "small leeway period" recommended by the OAuth 2.0
// Security Best Current Practice (draft-ietf-oauth-security-topics §4.13)
// to absorb harmless duplicate refresh requests caused by client retries
// without weakening the replay-detection signal for actual token theft.
//
// Tokens replayed AFTER this window still revoke the entire family.
const RefreshReplayGrace = 30 * time.Second

// replayCacheEntry holds the response returned on a successful refresh,
// keyed by the SHA-256 hash of the refresh token that was rotated. When
// the same hash is presented again within the grace window, the cached
// response is replayed verbatim so both racing clients converge on the
// same access JWT and refresh cookie.
type replayCacheEntry struct {
	accessToken string // JWT to return in the JSON body
	refreshRaw  string // raw refresh token to set in the cookie
	userID      int64
	username    string
	role        string
	familyID    string
	expiresAt   time.Time // entry TTL
}

// replayCache is a small in-memory map of recently rotated refresh-token
// hashes to the response that was issued, with a fixed TTL. Concurrent
// access is guarded by a mutex; the map is swept lazily on every access
// and proactively by the maintenance goroutine in cmd/server.
type replayCache struct {
	mu    sync.Mutex
	items map[string]replayCacheEntry
	ttl   time.Duration
}

func newReplayCache(ttl time.Duration) *replayCache {
	return &replayCache{
		items: make(map[string]replayCacheEntry),
		ttl:   ttl,
	}
}

// put records the response issued when the refresh token with the given
// hash was rotated. Stored entries auto-expire after the cache TTL.
func (c *replayCache) put(hash string, e replayCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.expiresAt = time.Now().Add(c.ttl)
	c.items[hash] = e
	c.sweepLocked()
}

// get returns the entry for hash if present and unexpired. Expired entries
// are removed.
func (c *replayCache) get(hash string) (replayCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[hash]
	if !ok {
		return replayCacheEntry{}, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.items, hash)
		return replayCacheEntry{}, false
	}
	return e, true
}

// sweepLocked drops expired entries; caller must hold c.mu.
func (c *replayCache) sweepLocked() {
	now := time.Now()
	for h, e := range c.items {
		if now.After(e.expiresAt) {
			delete(c.items, h)
		}
	}
}

// flush removes every entry. Used by tests to simulate the grace window
// having elapsed without sleeping.
func (c *replayCache) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]replayCacheEntry)
}
