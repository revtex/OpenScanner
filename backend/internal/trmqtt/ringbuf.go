package trmqtt

import "sync"

// Ring is a bounded, concurrency-safe ring buffer of T. Push overwrites the
// oldest entry when full. Snapshot returns a copy of the buffer in insertion
// order (oldest first).
type Ring[T any] struct {
	mu    sync.Mutex
	buf   []T
	cap   int
	head  int  // next write index
	count int  // valid entries
}

// NewRing creates an empty ring with capacity capN. Capacity <= 0 is clamped
// to 1 to avoid divide-by-zero.
func NewRing[T any](capN int) *Ring[T] {
	if capN < 1 {
		capN = 1
	}
	return &Ring[T]{buf: make([]T, capN), cap: capN}
}

// Push inserts v, dropping the oldest entry when at capacity.
func (r *Ring[T]) Push(v T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.head] = v
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

// Snapshot returns the contents in insertion order (oldest first).
func (r *Ring[T]) Snapshot() []T {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]T, r.count)
	if r.count == 0 {
		return out
	}
	start := (r.head - r.count + r.cap) % r.cap
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(start+i)%r.cap]
	}
	return out
}

// Len returns the current number of stored entries.
func (r *Ring[T]) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}
