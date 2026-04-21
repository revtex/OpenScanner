// Package ws — WebSocket client connection (listener + admin).
package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/logging"
)

const (
	writeWait      = 10 * time.Second
	pingPeriod     = 30 * time.Second
	sendBufSize    = 256
	authTimeout    = 10 * time.Second
	maxMessageSize = 4096
)

// systemGrant represents a system-level grant with optional talkgroup filtering.
type systemGrant struct {
	ID         int64   `json:"id"`
	Talkgroups []int64 `json:"talkgroups,omitempty"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	grants  []systemGrant // nil/empty = receive all
	isAdmin bool
	userID  int64
}

// adminRequest is the envelope for admin WS request messages.
type adminRequest struct {
	ReqID  string          `json:"reqId"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params,omitempty"`
}

// adminOpHandler is the function signature for all admin WS operation handlers.
type adminOpHandler func(ctx context.Context, params json.RawMessage) (any, error)

// userError is returned by op handlers for validation errors that should be
// shown verbatim to the client. Other errors are treated as internal.
type userError string

func (e userError) Error() string { return string(e) }

// CanReceive reports whether this client is authorized to receive a call for
// the given system and talkgroup. If grants is nil/empty, everything is allowed.
func (c *Client) CanReceive(systemID, talkgroupID int64) bool {
	if len(c.grants) == 0 {
		slog.Debug("ws: grant check (no grants, allow all)", "system", systemID, "talkgroup", talkgroupID)
		return true
	}
	for _, g := range c.grants {
		if g.ID != systemID {
			continue
		}
		// No TG filter → all TGs in this system.
		if len(g.Talkgroups) == 0 {
			slog.Debug("ws: grant check allowed (system match, no tg filter)", "system", systemID, "talkgroup", talkgroupID)
			return true
		}
		for _, tg := range g.Talkgroups {
			if tg == talkgroupID {
				slog.Debug("ws: grant check allowed", "system", systemID, "talkgroup", talkgroupID)
				return true
			}
		}
	}
	slog.Debug("ws: grant check denied", "system", systemID, "talkgroup", talkgroupID)
	return false
}

// parseGrants parses systems_json into a slice of systemGrant.
func parseGrants(systemsJSON sql.NullString) []systemGrant {
	if !systemsJSON.Valid || systemsJSON.String == "" {
		return nil
	}
	var grants []systemGrant
	if err := json.Unmarshal([]byte(systemsJSON.String), &grants); err != nil {
		slog.Warn("ws: failed to parse systems_json", "error", err)
		return nil
	}
	return grants
}

func wsAcceptOptions(r *http.Request) *websocket.AcceptOptions {
	patterns := []string{r.Host}

	// Allow localhost dev frontend origins (e.g. :5173) when backend runs on localhost.
	hostname := strings.ToLower(r.URL.Hostname())
	if hostname == "" {
		if u, err := url.Parse("http://" + r.Host); err == nil {
			hostname = strings.ToLower(u.Hostname())
		}
	}
	if hostname == "localhost" || hostname == "127.0.0.1" {
		patterns = append(patterns,
			"localhost:*",
			"127.0.0.1:*",
		)
	}

	return &websocket.AcceptOptions{
		OriginPatterns:  patterns,
		CompressionMode: websocket.CompressionContextTakeover,
	}
}

// HandleListenerWS upgrades the HTTP connection for a listener WebSocket.
func HandleListenerWS(hub *Hub, queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, wsAcceptOptions(r))
		if err != nil {
			slog.Error("ws: failed to accept listener connection",
				"error", err,
				"origin", r.Header.Get("Origin"),
				"host", r.Host,
			)
			return
		}

		slog.Debug("ws: listener connection accepted", "ip", r.RemoteAddr)

		ctx := r.Context()

		// Check maxClients setting.
		if maxStr, err := queries.GetSetting(ctx, "maxClients"); err == nil {
			if max, err := strconv.Atoi(maxStr.Value); err == nil && max > 0 {
				if hub.ClientCount() >= max {
					msg, _ := NewMAXMessage()
					_ = conn.Write(ctx, websocket.MessageText, msg)
					conn.Close(websocket.StatusNormalClosure, "max clients reached")
					return
				}
			}
		}

		// Check publicAccess setting.
		publicAccess := false
		if s, err := queries.GetSetting(ctx, "publicAccess"); err == nil {
			publicAccess = s.Value == "true"
		}

		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, sendBufSize),
		}

		if publicAccess {
			slog.Debug("ws: listener authenticated via public access")
			// Public access — no auth required, receive all.
			if err := sendWelcome(ctx, conn, hub, queries); err != nil {
				slog.Error("ws: failed to send welcome", "error", err)
				conn.Close(websocket.StatusInternalError, "")
				return
			}
			hub.Register(client)
			go client.writePump(ctx)
			client.readPump(ctx)
			return
		}

		// Wait for auth message with timeout.
		authCtx, cancel := context.WithTimeout(ctx, authTimeout)
		defer cancel()

		typ, data, err := conn.Read(authCtx)
		if err != nil {
			slog.Info("ws: listener auth timeout or read error", "error", err)
			conn.Close(websocket.StatusPolicyViolation, "auth timeout")
			return
		}
		if typ != websocket.MessageText {
			conn.Close(websocket.StatusPolicyViolation, "expected text message")
			return
		}

		cmd, payload, err := ParseCommand(data)
		if err != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid message")
			return
		}

		// Try as JWT token string (the entire message may be just a token).
		// First try to parse as a JSON string from payload, otherwise use raw cmd.
		tokenStr := cmd
		if payload != nil {
			// The message might be ["<token>"] where cmd is the token.
			tokenStr = cmd
		}
		// Also handle case where client sends raw token as first array element.
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			slog.Info("ws: invalid JWT on listener WS")
			sendExpiredAndClose(ctx, conn)
			return
		}
		if auth.Tokens.IsRevoked(claims.ID) {
			slog.Info("ws: revoked JWT on listener WS", "jti", claims.ID)
			sendExpiredAndClose(ctx, conn)
			return
		}
		if claims.Role != auth.RoleListener && claims.Role != auth.RoleAdmin {
			slog.Info("ws: invalid role on listener WS", "role", claims.Role)
			sendExpiredAndClose(ctx, conn)
			return
		}
		// Load user grants.
		user, err := queries.GetUser(ctx, claims.UserID)
		if err != nil || user.Disabled != 0 {
			slog.Info("ws: user not found or disabled on listener WS", "user_id", claims.UserID)
			sendExpiredAndClose(ctx, conn)
			return
		}
		// Enforce account expiration on WS connections.
		if user.Expiration.Valid && user.Expiration.Int64 > 0 {
			if time.Now().Unix() > user.Expiration.Int64 {
				slog.Info("ws: expired user on listener WS", "user_id", claims.UserID)
				sendExpiredAndClose(ctx, conn)
				return
			}
		}
		// Check user connection limit.
		if user.Limit.Valid && user.Limit.Int64 > 0 {
			if int64(hub.countByUser(user.ID)) >= user.Limit.Int64 {
				msg, _ := NewMAXMessage()
				_ = conn.Write(ctx, websocket.MessageText, msg)
				conn.Close(websocket.StatusNormalClosure, "connection limit")
				return
			}
		}
		client.userID = user.ID
		client.grants = parseGrants(user.SystemsJson)
		slog.Debug("ws: listener authenticated via jwt", "user_id", user.ID, "grants", len(client.grants))

		if err := sendWelcome(ctx, conn, hub, queries); err != nil {
			slog.Error("ws: failed to send welcome", "error", err)
			conn.Close(websocket.StatusInternalError, "")
			return
		}

		hub.Register(client)
		go client.writePump(ctx)
		client.readPump(ctx)
	}
}

// HandleAdminWS upgrades the HTTP connection for an admin WebSocket.
func HandleAdminWS(hub *Hub, queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Admin WS auth via query param since WebSocket can't send headers.
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, "token required", http.StatusUnauthorized)
			return
		}

		// Strip the token from the URL immediately to prevent it from being
		// logged in access logs, error reports, or proxy logs (OWASP A09).
		r.URL.RawQuery = ""

		claims, err := auth.ParseToken(tokenStr)
		if err != nil || auth.Tokens.IsRevoked(claims.ID) {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if claims.Role != auth.RoleAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}

		conn, err := websocket.Accept(w, r, wsAcceptOptions(r))
		if err != nil {
			slog.Error("ws: failed to accept admin connection",
				"error", err,
				"origin", r.Header.Get("Origin"),
				"host", r.Host,
			)
			return
		}

		slog.Debug("ws: admin connection accepted", "user_id", claims.UserID)

		ctx := r.Context()

		client := &Client{
			hub:     hub,
			conn:    conn,
			send:    make(chan []byte, sendBufSize),
			isAdmin: true,
			userID:  claims.UserID,
		}

		hub.Register(client)
		go client.writePump(ctx)
		client.readPump(ctx)
	}
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMessageSize)

	for {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) &&
				websocket.CloseStatus(err) == -1 {
				slog.Info("ws: read error", "error", err)
			}
			return
		}
		if typ != websocket.MessageText {
			continue
		}

		cmd, payload, err := ParseCommand(data)
		if err != nil {
			continue
		}

		switch cmd {
		case CmdLFM:
			slog.Debug("ws: received command", "cmd", cmd)
			// Client updating live feed map — echo it back.
			if payload != nil {
				var fm map[string]any
				if err := json.Unmarshal(payload, &fm); err == nil {
					msg, err := NewLFMMessage(fm)
					if err == nil {
						select {
						case c.send <- msg:
						default:
						}
					}
				}
			}
		case CmdADMREQ:
			if !c.isAdmin {
				slog.Warn("ws: non-admin client sent ADM_REQ")
				continue
			}
			if payload == nil {
				slog.Warn("ws: ADM_REQ with nil payload")
				continue
			}
			var req adminRequest
			if err := json.Unmarshal(payload, &req); err != nil {
				slog.Warn("ws: failed to parse ADM_REQ payload", "error", err)
				continue
			}
			if req.ReqID == "" {
				slog.Warn("ws: ADM_REQ missing reqId")
				continue
			}
			c.handleAdminRequest(ctx, req)
		default:
			slog.Warn("ws: received unknown command", "cmd", cmd)
		}
	}
}

// handleAdminRequest dispatches an admin WS request to the appropriate handler.
func (c *Client) handleAdminRequest(ctx context.Context, req adminRequest) {
	slog.Debug("ws: handling admin request", "op", req.Op, "reqId", req.ReqID)

	handlers := c.adminOpHandlers()
	handler, ok := handlers[req.Op]
	if !ok {
		msg, _ := NewADMRESErrorMessage(req.ReqID, "unknown op: "+req.Op)
		select {
		case c.send <- msg:
		default:
		}
		return
	}

	data, err := handler(ctx, req.Params)
	var msg []byte
	if err != nil {
		if _, ok := err.(userError); ok {
			msg, _ = NewADMRESErrorMessage(req.ReqID, err.Error())
		} else {
			slog.Error("ws: admin op failed", "op", req.Op, "reqId", req.ReqID, "error", err)
			msg, _ = NewADMRESErrorMessage(req.ReqID, "internal error")
		}
	} else {
		msg, _ = NewADMRESMessage(req.ReqID, data)
	}
	select {
	case c.send <- msg:
	default:
	}
}

func (c *Client) opActivityStats(ctx context.Context, _ json.RawMessage) (any, error) {
	now := time.Now()
	y, m, d := now.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location()).Unix()

	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	weekStart := time.Date(y, m, d-int(weekday-time.Monday), 0, 0, 0, 0, now.Location()).Unix()

	stats, err := c.hub.queries.GetActivityStats(ctx, db.GetActivityStatsParams{
		TodayStart: todayStart,
		WeekStart:  weekStart,
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"callsToday":      stats.CallsToday,
		"callsThisWeek":   stats.CallsThisWeek,
		"callsTotal":      stats.CallsTotal,
		"activeListeners": c.hub.ClientCount(),
		"uptime":          int64(time.Since(StartTime).Seconds()),
	}, nil
}

func (c *Client) opActivityChart(ctx context.Context, _ json.RawMessage) (any, error) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	rows, err := c.hub.queries.GetCallsPerHour(ctx, cutoff)
	if err != nil {
		return nil, err
	}

	buckets := make([]map[string]int64, len(rows))
	for i, r := range rows {
		buckets[i] = map[string]int64{"hour": r.HourBucket, "count": r.CallCount}
	}
	return map[string]any{"buckets": buckets}, nil
}

func (c *Client) opTopTalkgroups(ctx context.Context, _ json.RawMessage) (any, error) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	rows, err := c.hub.queries.GetTopTalkgroups(ctx, db.GetTopTalkgroupsParams{
		DateTime: cutoff,
		Limit:    10,
	})
	if err != nil {
		return nil, err
	}

	tgs := make([]map[string]any, len(rows))
	for i, r := range rows {
		tgs[i] = map[string]any{
			"talkgroupId":    r.TalkgroupID.Int64,
			"talkgroupLabel": r.TalkgroupLabel.String,
			"talkgroupName":  r.TalkgroupName.String,
			"systemLabel":    r.SystemLabel.String,
			"callCount":      r.CallCount,
		}
	}
	return map[string]any{"talkgroups": tgs}, nil
}

func (c *Client) opLogsQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Level string `json:"level"`
		From  int64  `json:"from"`
		To    int64  `json:"to"`
		Query string `json:"q"`
		Limit int    `json:"limit"`
	}
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
	}
	if p.Limit <= 0 || p.Limit > 10_000 {
		p.Limit = 500
	}

	entries := logging.QueryEntries(p.Level, p.From, p.To, p.Query, p.Limit)
	resp := make([]map[string]any, len(entries))
	for i, e := range entries {
		resp[i] = map[string]any{
			"dateTime": e.Time.Unix(),
			"level":    e.Level,
			"message":  e.Message,
			"attrs":    e.Attrs,
		}
	}
	return resp, nil
}

// writePump sends messages from the send channel to the WebSocket connection
// and sends periodic pings for keepalive.
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel.
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// sendExpiredAndClose sends an XPR message and closes the connection.
func sendExpiredAndClose(ctx context.Context, conn *websocket.Conn) {
	msg, _ := NewXPRMessage()
	_ = conn.Write(ctx, websocket.MessageText, msg)
	conn.Close(websocket.StatusPolicyViolation, "auth failed")
}

// sendWelcome sends VER and CFG messages to a newly authenticated client.
func sendWelcome(ctx context.Context, conn *websocket.Conn, hub *Hub, queries *db.Queries) error {
	slog.Debug("ws: sending welcome (VER+CFG)")
	// Build VER message.
	branding := ""
	if s, err := queries.GetSetting(ctx, "branding"); err == nil {
		branding = s.Value
	}
	email := ""
	if s, err := queries.GetSetting(ctx, "email"); err == nil {
		email = s.Value
	}
	verMsg, err := NewVERMessage(hub.version, branding, email)
	if err != nil {
		return err
	}
	if err := conn.Write(ctx, websocket.MessageText, verMsg); err != nil {
		return err
	}

	cfgMsg, err := buildCFGMessage(ctx, queries)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, cfgMsg)
}

// buildCFGMessage constructs the CFG WebSocket message from the current
// database state (systems, talkgroups, groups, tags, settings).
func buildCFGMessage(ctx context.Context, queries *db.Queries) ([]byte, error) {
	// Resolve group and tag labels first so talkgroups carry string labels,
	// matching the TalkgroupConfig type expected by the frontend.
	groups, _ := queries.ListGroups(ctx)
	tags, _ := queries.ListTags(ctx)

	groupLabels := make(map[int64]string, len(groups))
	for _, g := range groups {
		groupLabels[g.ID] = g.Label
	}
	tagLabels := make(map[int64]string, len(tags))
	for _, t := range tags {
		tagLabels[t.ID] = t.Label
	}

	systems, err := queries.ListSystems(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	type tgCfg struct {
		ID          int64  `json:"id"`
		TalkgroupID int64  `json:"talkgroupId"`
		Label       string `json:"label,omitempty"`
		Name        string `json:"name,omitempty"`
		Group       string `json:"group,omitempty"`
		Tag         string `json:"tag,omitempty"`
		LedColor    string `json:"ledColor,omitempty"`
		Frequency   *int64 `json:"frequency,omitempty"`
	}
	type sysCfg struct {
		ID         int64   `json:"id"`
		SystemID   int64   `json:"systemId"`
		Label      string  `json:"label"`
		Talkgroups []tgCfg `json:"talkgroups"`
	}
	sysCfgs := []sysCfg{} // never nil — serialises as [] not null
	for _, s := range systems {
		sc := sysCfg{ID: s.ID, SystemID: s.SystemID, Label: s.Label, Talkgroups: []tgCfg{}}
		tgs, err := queries.ListTalkgroupsBySystem(ctx, s.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		for _, tg := range tgs {
			t := tgCfg{ID: tg.ID, TalkgroupID: tg.TalkgroupID}
			if tg.Label.Valid {
				t.Label = tg.Label.String
			}
			if tg.Name.Valid {
				t.Name = tg.Name.String
			}
			if tg.GroupID.Valid {
				t.Group = groupLabels[tg.GroupID.Int64]
			}
			if tg.TagID.Valid {
				t.Tag = tagLabels[tg.TagID.Int64]
			}
			if tg.Led.Valid {
				t.LedColor = tg.Led.String
			}
			if tg.Frequency.Valid {
				freq := tg.Frequency.Int64
				t.Frequency = &freq
			}
			sc.Talkgroups = append(sc.Talkgroups, t)
		}
		sysCfgs = append(sysCfgs, sc)
	}

	cfgPayload := map[string]any{
		"systems": sysCfgs,
	}

	// Include scanner display settings in the config payload.
	if s, err := queries.GetSetting(ctx, "time12hFormat"); err == nil {
		cfgPayload["time12hFormat"] = s.Value == "true"
	}
	if s, err := queries.GetSetting(ctx, "showListenersCount"); err == nil {
		cfgPayload["showListenersCount"] = s.Value == "true"
	}
	if s, err := queries.GetSetting(ctx, "playbackGoesLive"); err == nil {
		cfgPayload["playbackGoesLive"] = s.Value == "true"
	}
	if s, err := queries.GetSetting(ctx, "keypadBeeps"); err == nil {
		cfgPayload["keypadBeeps"] = s.Value
	}
	if s, err := queries.GetSetting(ctx, "shareableLinks"); err == nil {
		cfgPayload["shareableLinks"] = s.Value == "true"
	}
	if s, err := queries.GetSetting(ctx, "transcriptionEnabled"); err == nil {
		cfgPayload["transcriptionEnabled"] = s.Value == "true"
	}

	return NewCFGMessage(cfgPayload)
}
