// Package ws — WebSocket client connection (listener + admin).
package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
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
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	sendMu   sync.Mutex
	grants   []systemGrant // nil/empty = receive all
	isAdmin  bool
	userID   int64
	accessID int64
}

// CanReceive reports whether this client is authorized to receive a call for
// the given system and talkgroup. If grants is nil/empty, everything is allowed.
func (c *Client) CanReceive(systemID, talkgroupID int64) bool {
	if len(c.grants) == 0 {
		return true
	}
	for _, g := range c.grants {
		if g.ID != systemID {
			continue
		}
		// No TG filter → all TGs in this system.
		if len(g.Talkgroups) == 0 {
			return true
		}
		for _, tg := range g.Talkgroups {
			if tg == talkgroupID {
				return true
			}
		}
	}
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

// HandleListenerWS upgrades the HTTP connection for a listener WebSocket.
func HandleListenerWS(hub *Hub, queries *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			slog.Error("ws: failed to accept listener connection", "error", err)
			return
		}

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

		switch cmd {
		case CmdPIN:
			// Access code authentication.
			var code string
			if err := json.Unmarshal(payload, &code); err != nil {
				slog.Info("ws: invalid PIN format")
				sendExpiredAndClose(ctx, conn)
				return
			}
			access, err := queries.GetAccessByCode(ctx, code)
			if err != nil {
				slog.Info("ws: invalid access code")
				sendExpiredAndClose(ctx, conn)
				return
			}
			// Check expiration.
			if access.Expiration.Valid && access.Expiration.Int64 > 0 {
				if time.Now().Unix() > access.Expiration.Int64 {
					slog.Info("ws: expired access code", "access_id", access.ID)
					sendExpiredAndClose(ctx, conn)
					return
				}
			}
			// Check connection limit.
			if access.Limit.Valid && access.Limit.Int64 > 0 {
				if int64(hub.countByAccess(access.ID)) >= access.Limit.Int64 {
					msg, _ := NewMAXMessage()
					_ = conn.Write(ctx, websocket.MessageText, msg)
					conn.Close(websocket.StatusNormalClosure, "connection limit")
					return
				}
			}
			client.accessID = access.ID
			client.grants = parseGrants(access.SystemsJson)

		default:
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
			if claims.Role != auth.RoleListener {
				slog.Info("ws: non-listener role on listener WS", "role", claims.Role)
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
		}

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

		claims, err := auth.ParseToken(tokenStr)
		if err != nil || auth.Tokens.IsRevoked(claims.ID) {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if claims.Role != auth.RoleAdmin {
			http.Error(w, "admin access required", http.StatusForbidden)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			slog.Error("ws: failed to accept admin connection", "error", err)
			return
		}

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
		case CmdPIN:
			// Re-auth is not supported after initial connection — ignore.
		}
	}
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
			// Determine message type: if it doesn't start with '[' or '{',
			// treat as binary (audio data). Otherwise text.
			msgType := websocket.MessageText
			if len(msg) > 0 && msg[0] != '[' && msg[0] != '{' {
				msgType = websocket.MessageBinary
			}
			err := c.conn.Write(writeCtx, msgType, msg)
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

	// Build CFG message with systems and talkgroups.
	systems, err := queries.ListSystems(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	type tgCfg struct {
		ID          int64  `json:"id"`
		TalkgroupID int64  `json:"talkgroupId"`
		Label       string `json:"label,omitempty"`
		Name        string `json:"name,omitempty"`
		GroupID     int64  `json:"groupId,omitempty"`
		TagID       int64  `json:"tagId,omitempty"`
	}
	type sysCfg struct {
		ID         int64   `json:"id"`
		SystemID   int64   `json:"systemId"`
		Label      string  `json:"label"`
		Talkgroups []tgCfg `json:"talkgroups"`
	}
	var sysCfgs []sysCfg
	for _, s := range systems {
		sc := sysCfg{ID: s.ID, SystemID: s.SystemID, Label: s.Label}
		tgs, err := queries.ListTalkgroupsBySystem(ctx, s.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
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
				t.GroupID = tg.GroupID.Int64
			}
			if tg.TagID.Valid {
				t.TagID = tg.TagID.Int64
			}
			sc.Talkgroups = append(sc.Talkgroups, t)
		}
		sysCfgs = append(sysCfgs, sc)
	}

	// Gather groups and tags for the config.
	groups, _ := queries.ListGroups(ctx)
	tags, _ := queries.ListTags(ctx)

	cfgPayload := map[string]any{
		"systems": sysCfgs,
		"groups":  groups,
		"tags":    tags,
	}
	cfgMsg, err := NewCFGMessage(cfgPayload)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, cfgMsg)
}
