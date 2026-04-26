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
	// data is the legacy 3-letter array-framed payload, sent to every
	// matching client whose protocolVersion is the legacy default.
	data []byte
	// v1 is the optional native JSON-object framed payload, sent to every
	// matching client whose protocolVersion == "v1". When nil, v1 clients
	// receive the legacy bytes (used by callers that have not been
	// migrated to dual-encoding yet).
	v1     []byte
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
			slog.Debug("ws: broadcasting message", "size", len(msg.data), "has_filter", msg.filter != nil, "has_v1", msg.v1 != nil)
			h.mu.RLock()
			for c := range h.clients {
				if msg.filter != nil && !msg.filter(c) {
					continue
				}
				data := msg.data
				if c.protocolVersion == protocolV1 && msg.v1 != nil {
					data = msg.v1
				}
				c.trySend(data)
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

// broadcastBoth enqueues a broadcast carrying both the legacy and the
// native (v1) encoding of the same logical event. Each client receives the
// frame matching its negotiated protocol version. Non-blocking.
func (h *Hub) broadcastBoth(legacy, v1 []byte, filter func(*Client) bool) {
	select {
	case h.broadcast <- broadcastMsg{data: legacy, v1: v1, filter: filter}:
	default:
		slog.Warn("ws: broadcast channel full, dropping message")
	}
}

// BroadcastCAL fans out a new-call event to matching clients in both the
// legacy and native (v1) wire formats. The payload map is the same one
// already produced by the upload and dirmonitor handlers — its camelCase
// fields (id, audioName, audioType, dateTime, systemId, talkgroupId,
// frequency, duration, source, sources, frequencies, errorCount,
// spikeCount, talkerAlias, site, channel, decoder) are reused verbatim
// inside the native call.new envelope. Also notifies admin clients so the
// activity dashboard can refresh.
func (h *Hub) BroadcastCAL(payload map[string]any, filter func(*Client) bool) {
	legacy, err := NewCALMessage(payload)
	if err != nil {
		slog.Error("ws: failed to build legacy CAL", "error", err)
		return
	}
	v1, err := NewCallNewV1(payload)
	if err != nil {
		slog.Error("ws: failed to build native call.new", "error", err)
		// Fall back to legacy-only — v1 clients will receive the legacy
		// bytes, which is wrong but fails closed rather than dropping the
		// call entirely.
		h.Broadcast(legacy, filter)
		h.BroadcastAdminEvent("activity.updated", nil)
		return
	}
	h.broadcastBoth(legacy, v1, filter)
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
	legacy, v1, err := buildCFGFrames(ctx, h.queries)
	if err != nil {
		slog.Error("ws: failed to build CFG for broadcast", "error", err)
		return
	}
	h.broadcastBoth(legacy, v1, nil)
	slog.Debug("ws: cfg broadcast complete", "clients", h.ClientCount())
}

// BroadcastAdminEvent sends an admin event (legacy ADM_EVT / native
// admin.event) to all connected admin clients in both wire formats.
func (h *Hub) BroadcastAdminEvent(topic string, data any) {
	legacy, err := NewADMEVTMessage(topic, data)
	if err != nil {
		slog.Error("ws: failed to build admin event", "topic", topic, "error", err)
		return
	}
	v1, err := NewAdminEventV1(topic, data)
	if err != nil {
		slog.Error("ws: failed to build native admin.event", "topic", topic, "error", err)
		h.Broadcast(legacy, func(c *Client) bool { return c.isAdmin })
		return
	}
	h.broadcastBoth(legacy, v1, func(c *Client) bool { return c.isAdmin })
}

// BroadcastTRN sends a transcript-ready message (legacy TRN / native
// call.transcript) to all connected listener clients in both wire formats.
// segments may be nil when diarization is disabled.
func (h *Hub) BroadcastTRN(callID int64, text string, segments any) {
	if h == nil {
		return
	}
	legacy, err := NewTRNMessage(callID, text, segments)
	if err != nil {
		slog.Error("ws: failed to build TRN message", "call_id", callID, "error", err)
		return
	}
	v1, err := NewCallTranscriptV1(callID, text, segments)
	if err != nil {
		slog.Error("ws: failed to build native call.transcript", "call_id", callID, "error", err)
		h.Broadcast(legacy, nil)
		return
	}
	h.broadcastBoth(legacy, v1, nil)
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
		c.trySend(c.encodeSessionExpired())
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
		target.trySend(target.encodeSessionExpired())
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
		legacy, err := NewLSCMessage(count)
		if err != nil {
			slog.Error("ws: failed to build LSC message", "error", err)
			return
		}
		v1, err := NewListenerCountV1(count)
		if err != nil {
			slog.Error("ws: failed to build native listener.count", "error", err)
			h.Broadcast(legacy, nil)
			return
		}
		h.broadcastBoth(legacy, v1, nil)
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
