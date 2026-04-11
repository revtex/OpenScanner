// Package auth — login rate limiter (3 failed attempts → 10-minute lockout).
package auth

import (
	"context"
	"sync"
	"time"
)

const (
	maxFailures     = 3
	lockoutDuration = 10 * time.Minute
	cleanupInterval = 15 * time.Minute
)

type loginEntry struct {
	failures    int
	lockedUntil time.Time
	lastFailure time.Time
}

// RateLimiter is an in-memory per-IP login rate limiter.
// After maxFailures failures the IP is locked out for lockoutDuration.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginEntry
}

// NewRateLimiter creates a new RateLimiter and starts its background cleanup goroutine.
// The goroutine is bound to ctx and exits when ctx is cancelled, preventing goroutine leaks.
func NewRateLimiter(ctx context.Context) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*loginEntry),
	}
	go rl.cleanup(ctx)
	return rl
}

// RecordFailure records a failed login attempt for the given IP.
// Returns true if the IP is now locked out.
func (r *RateLimiter) RecordFailure(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[ip]
	if !ok {
		e = &loginEntry{}
		r.entries[ip] = e
	}

	now := time.Now()

	// If already locked out, just reconfirm.
	if now.Before(e.lockedUntil) {
		return true
	}

	// If lockout has expired, reset the counter to give fresh attempts.
	if e.failures >= maxFailures && now.After(e.lockedUntil) {
		e.failures = 0
	}

	e.failures++
	e.lastFailure = now
	if e.failures >= maxFailures {
		e.lockedUntil = now.Add(lockoutDuration)
		return true
	}
	return false
}

// IsLockedOut returns true if the IP is currently in lockout.
func (r *RateLimiter) IsLockedOut(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	e, ok := r.entries[ip]
	if !ok {
		return false
	}
	return time.Now().Before(e.lockedUntil)
}

// Reset clears the failure record for an IP (call on successful login).
func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, ip)
}

// cleanup periodically removes stale entries to bound memory usage.
func (r *RateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for ip, e := range r.entries {
				if e.failures >= maxFailures {
					// Remove once lockout has expired.
					if now.After(e.lockedUntil) {
						delete(r.entries, ip)
					}
				} else {
					// Remove partial-failure entries older than the lockout window.
					if now.Sub(e.lastFailure) > lockoutDuration {
						delete(r.entries, ip)
					}
				}
			}
			r.mu.Unlock()
		}
	}
}
