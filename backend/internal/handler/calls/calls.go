// Package calls — call upload (POST /api/call-upload, /api/trunk-recorder-call-upload)
// plus the authenticated call search, audio, and transcript endpoints.
package calls

import (
	"sync"
	"time"

	"github.com/openscanner/openscanner/internal/audio"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/downstream"
	"github.com/openscanner/openscanner/internal/ws"
)

const (
	defaultCallRatePerMin = 60
	maxCallRatePerMin     = 600
	rateWindowDuration    = time.Minute
)

// DownstreamNotifier sends call events to downstream pushers.
type DownstreamNotifier interface {
	Notify(event downstream.CallEvent)
}

// apiKeyLimiter is a per-API-key sliding-window rate limiter.
type apiKeyLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	rateLimit   int
}

func (l *apiKeyLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.windowStart) >= rateWindowDuration {
		l.windowStart = now
		l.count = 0
	}
	if l.count >= l.rateLimit {
		return false
	}
	l.count++
	return true
}

// Handler handles call upload and archive endpoints.
type Handler struct {
	queries     *db.Queries
	processor   *audio.Processor
	hub         *ws.Hub
	dsNotifier  DownstreamNotifier
	transcriber audio.Transcriber // nil when transcription is disabled

	mu       sync.Mutex
	limiters map[int64]*apiKeyLimiter
}

// New creates a call Handler.
func New(queries *db.Queries, processor *audio.Processor, hub *ws.Hub, dsNotifier DownstreamNotifier, transcriber audio.Transcriber) *Handler {
	return &Handler{
		queries:     queries,
		processor:   processor,
		hub:         hub,
		dsNotifier:  dsNotifier,
		transcriber: transcriber,
		limiters:    make(map[int64]*apiKeyLimiter),
	}
}

func (h *Handler) getLimiter(apiKeyID int64, rateLimit int) *apiKeyLimiter {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Periodic cleanup: remove limiters whose window has expired to bound
	// memory growth (one entry per unique API key ID).
	if len(h.limiters) > 100 {
		now := time.Now()
		for id, l := range h.limiters {
			l.mu.Lock()
			stale := now.Sub(l.windowStart) >= 2*rateWindowDuration
			l.mu.Unlock()
			if stale {
				delete(h.limiters, id)
			}
		}
	}

	l, ok := h.limiters[apiKeyID]
	if !ok {
		l = &apiKeyLimiter{
			windowStart: time.Now(),
			rateLimit:   rateLimit,
		}
		h.limiters[apiKeyID] = l
	} else {
		l.mu.Lock()
		l.rateLimit = rateLimit
		l.mu.Unlock()
	}
	return l
}
