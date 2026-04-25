// Package ws implements the OpenScanner WebSocket hub.
package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/openscanner/openscanner/internal/admin"
	"github.com/openscanner/openscanner/internal/db"
)

// Reloader triggers a service config reload (e.g. dirmonitor, downstream).
// Kept as a ws-local alias to admin.Reloader so external callers that
// reference ws.Reloader continue to compile.
type Reloader = admin.Reloader

// TranscriberReloader can hot-reload the transcription subsystem.
type TranscriberReloader = admin.TranscriberReloader

// HubDeps holds optional dependencies injected into the Hub for admin WS
// operations. It is an alias for admin.Deps so callers can keep using
// ws.HubDeps{...} while the underlying fields live in the admin package.
type HubDeps = admin.Deps

// StartTime is the process start time, used for uptime calculations.
var StartTime = time.Now()

// Hub manages all WebSocket client connections and broadcasts messages.
type Hub struct {
	queries *db.Queries
	version string
	admin   *admin.Operations // transport-agnostic admin ops

	mu      sync.RWMutex
	clients map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg
	done       chan struct{}

	// lscTimer is the debounce timer for LSC broadcasts (max once per 3s).
	lscTimer *time.Timer
	lscMu    sync.Mutex
}

const lscDebounceDuration = 3 * time.Second

type broadcastMsg struct {
	data   []byte
	filter func(*Client) bool
}

// NewHub creates a new Hub. Pass the queries for settings lookups and the
// server version string for VER messages.
func NewHub(queries *db.Queries, version string, deps ...HubDeps) *Hub {
	var d HubDeps
	if len(deps) > 0 {
		d = deps[0]
	}
	h := &Hub{
		queries:    queries,
		version:    version,
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg, 256),
		done:       make(chan struct{}),
	}
	h.admin = admin.New(queries, d, h)
	return h
}

// Run starts the hub's event loop. It blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	defer h.closeAll()
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			slog.Debug("ws: client registered", "user_id", c.userID, "is_admin", c.isAdmin)
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
			if !c.isAdmin {
				h.debounceLSC()
			}
		case c := <-h.unregister:
			slog.Debug("ws: client unregistered", "user_id", c.userID, "is_admin", c.isAdmin)
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				c.closeSend()
			}
			h.mu.Unlock()
			if !c.isAdmin {
				h.debounceLSC()
			}
		case msg := <-h.broadcast:
			slog.Debug("ws: broadcasting message", "size", len(msg.data), "has_filter", msg.filter != nil)
			h.mu.RLock()
			for c := range h.clients {
				if msg.filter != nil && !msg.filter(c) {
					continue
				}
				c.trySend(msg.data)
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a text message to all clients matching the filter.
// If filter is nil, sends to all clients. Non-blocking (drops if hub is busy).
func (h *Hub) Broadcast(data []byte, filter func(*Client) bool) {
	select {
	case h.broadcast <- broadcastMsg{data: data, filter: filter}:
	default:
		slog.Warn("ws: broadcast channel full, dropping message")
	}
}

// BroadcastCAL sends a metadata-only CAL message to matching clients.
// Audio bytes are no longer embedded — clients fetch them on demand from
// GET /api/calls/:id/audio. Also notifies admin clients so the activity
// dashboard can refresh.
func (h *Hub) BroadcastCAL(calMsg []byte, filter func(*Client) bool) {
	h.Broadcast(calMsg, filter)
	h.BroadcastAdminEvent("activity.updated", nil)
}

// BroadcastCFG rebuilds the CFG message from the database and sends it to
// all connected clients. Call this when systems or talkgroups are added or
// modified so that connected scanners see updated names and labels.
// Safe to call on a nil hub (no-op).
func (h *Hub) BroadcastCFG(ctx context.Context) {
	if h == nil {
		return
	}
	slog.Debug("ws: rebuilding and broadcasting CFG")
	cfgMsg, err := buildCFGMessage(ctx, h.queries)
	if err != nil {
		slog.Error("ws: failed to build CFG for broadcast", "error", err)
		return
	}
	h.Broadcast(cfgMsg, nil)
	slog.Debug("ws: cfg broadcast complete", "clients", h.ClientCount())
}

// BroadcastAdminEvent sends an ADM_EVT to all connected admin clients.
func (h *Hub) BroadcastAdminEvent(topic string, data any) {
	msg, err := NewADMEVTMessage(topic, data)
	if err != nil {
		slog.Error("ws: failed to build admin event", "topic", topic, "error", err)
		return
	}
	h.Broadcast(msg, func(c *Client) bool {
		return c.isAdmin
	})
}

// BroadcastTRN sends a transcript-ready message to all connected listener
// clients. segments may be nil when diarization is disabled.
func (h *Hub) BroadcastTRN(callID int64, text string, segments any) {
	if h == nil {
		return
	}
	msg, err := NewTRNMessage(callID, text, segments)
	if err != nil {
		slog.Error("ws: failed to build TRN message", "call_id", callID, "error", err)
		return
	}
	h.Broadcast(msg, nil)
}

// ClientCount returns the number of non-admin (listener) clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for c := range h.clients {
		if !c.isAdmin {
			count++
		}
	}
	return count
}

// Register adds a client to the hub. Safe to call after hub shutdown.
func (h *Hub) Register(c *Client) {
	select {
	case h.register <- c:
	case <-h.done:
		c.closeSend()
	}
}

// Unregister removes a client from the hub. Safe to call after hub shutdown.
func (h *Hub) Unregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}

// countByUser returns the number of active clients for the given user ID.
func (h *Hub) countByUser(userID int64) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for c := range h.clients {
		if c.userID == userID {
			count++
		}
	}
	return count
}

// DisconnectByUser closes all WS connections for the given user ID.
// Sends an XPR message before closing so the client knows to re-authenticate.
func (h *Hub) DisconnectByUser(userID int64) {
	h.mu.RLock()
	var targets []*Client
	for c := range h.clients {
		if c.userID == userID {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		slog.Info("ws: disconnecting user session", "user_id", userID, "is_admin", c.isAdmin)
		msg, _ := NewXPRMessage()
		c.trySend(msg)
		h.Unregister(c)
	}
}

// DisconnectByJTI closes the WS connection associated with the given JWT ID.
func (h *Hub) DisconnectByJTI(jti string) {
	h.mu.RLock()
	var target *Client
	for c := range h.clients {
		if c.jti == jti {
			target = c
			break
		}
	}
	h.mu.RUnlock()

	if target != nil {
		slog.Info("ws: disconnecting session by JTI", "jti", jti, "user_id", target.userID)
		msg, _ := NewXPRMessage()
		target.trySend(msg)
		h.Unregister(target)
	}
}

// SetDirMonitorReloader sets the DirMonitor reloader after hub creation.
// This handles the circular dependency where dwService needs hub but hub
// needs dwService's Reloader.
func (h *Hub) SetDirMonitorReloader(r Reloader) {
	if h.admin != nil {
		h.admin.Deps.DirMonitorReload = r
	}
}

// debounceLSC schedules an LSC broadcast, resetting the timer if one is already
// pending. Ensures at most one LSC broadcast per lscDebounceDuration.
func (h *Hub) debounceLSC() {
	h.lscMu.Lock()
	defer h.lscMu.Unlock()
	if h.lscTimer != nil {
		h.lscTimer.Stop()
	}
	h.lscTimer = time.AfterFunc(lscDebounceDuration, func() {
		count := h.ClientCount()
		msg, err := NewLSCMessage(count)
		if err != nil {
			slog.Error("ws: failed to build LSC message", "error", err)
			return
		}
		h.Broadcast(msg, nil)
	})
}

// closeAll closes all connected clients during shutdown.
func (h *Hub) closeAll() {
	close(h.done)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		c.closeSend()
		delete(h.clients, c)
	}
	h.lscMu.Lock()
	if h.lscTimer != nil {
		h.lscTimer.Stop()
	}
	h.lscMu.Unlock()
}
