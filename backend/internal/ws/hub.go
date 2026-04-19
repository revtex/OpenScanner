// Package ws implements the OpenScanner WebSocket hub.
package ws

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// Reloader triggers a service config reload (e.g. dirmonitor, downstream).
type Reloader interface {
	Reload()
}

// HubDeps holds optional dependencies injected into the Hub for admin WS operations.
type HubDeps struct {
	SQLDB            *sql.DB
	DirMonitorReload Reloader
	DownstreamReload Reloader
	FFmpegAvailable  bool
	FDKAACAvailable  bool
	WhisperAvailable bool
	RecordingsDir    string
}

// StartTime is the process start time, used for uptime calculations.
var StartTime = time.Now()

// Hub manages all WebSocket client connections and broadcasts messages.
type Hub struct {
	queries *db.Queries
	version string
	deps    HubDeps

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
	return &Hub{
		queries:    queries,
		version:    version,
		deps:       d,
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg, 256),
		done:       make(chan struct{}),
	}
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
				close(c.send)
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
				select {
				case c.send <- msg.data:
				default:
					// Slow client — drop the message.
				}
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

// BroadcastCAL sends a CAL message (with embedded base64 audio) to matching
// clients. Audio is now part of the JSON text frame — no separate binary frame.
func (h *Hub) BroadcastCAL(calMsg []byte, filter func(*Client) bool) {
	h.Broadcast(calMsg, filter)
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
		close(c.send)
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

// SetDirMonitorReloader sets the DirMonitor reloader after hub creation.
// This handles the circular dependency where dwService needs hub but hub
// needs dwService's Reloader.
func (h *Hub) SetDirMonitorReloader(r Reloader) {
	h.deps.DirMonitorReload = r
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
		close(c.send)
		delete(h.clients, c)
	}
	h.lscMu.Lock()
	if h.lscTimer != nil {
		h.lscTimer.Stop()
	}
	h.lscMu.Unlock()
}
