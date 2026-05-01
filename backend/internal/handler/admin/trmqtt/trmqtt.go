// Package trmqtt provides the admin REST endpoints for managing
// trunk-recorder MQTT instances at /api/admin/tr/* and /api/v1/admin/tr/*.
//
// All endpoints return 404 when the trMqttEnabled setting is not "true" or
// when the Manager dependency is nil (feature disabled). Broker passwords
// are AES-256-GCM encrypted on write via auth.EncryptString and never
// echoed back in responses or logged.
package trmqtt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	trmqttsvc "github.com/openscanner/openscanner/internal/trmqtt"
)

// Handler serves the /api/{,v1/}admin/tr/* endpoints.
type Handler struct {
	queries       *db.Queries
	manager       *trmqttsvc.Manager
	encryptionKey string
}

// New constructs a Handler. A nil manager disables every endpoint (404).
func New(queries *db.Queries, manager *trmqttsvc.Manager, encryptionKey string) *Handler {
	return &Handler{queries: queries, manager: manager, encryptionKey: encryptionKey}
}

// instanceResponse is the JSON shape returned for a single tr_instances row.
// password_enc is intentionally never included; HasPassword is the only
// signal callers receive.
type instanceResponse struct {
	ID            int64  `json:"id"`
	Label         string `json:"label"`
	InstanceID    string `json:"instanceId"`
	BrokerURL     string `json:"brokerUrl"`
	BaseTopic     string `json:"baseTopic"`
	UnitTopic     string `json:"unitTopic,omitempty"`
	MessageTopic  string `json:"messageTopic,omitempty"`
	Username      string `json:"username,omitempty"`
	HasPassword   bool   `json:"hasPassword"`
	TLSSkipVerify bool   `json:"tlsSkipVerify"`
	QoS           int    `json:"qos"`
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status"`
	LastSeenAt    *int64 `json:"lastSeenAt,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
} // @name TRInstance

// createRequest is the JSON body for POST /admin/tr/instances.
type createRequest struct {
	Label         string `json:"label" binding:"required"`
	InstanceID    string `json:"instanceId" binding:"required"`
	BrokerURL     string `json:"brokerUrl" binding:"required"`
	BaseTopic     string `json:"baseTopic" binding:"required"`
	UnitTopic     string `json:"unitTopic,omitempty"`
	MessageTopic  string `json:"messageTopic,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	TLSSkipVerify bool   `json:"tlsSkipVerify"`
	QoS           int    `json:"qos"`
	Enabled       *bool  `json:"enabled,omitempty"`
} // @name TRInstanceCreateRequest

// updateRequest is the JSON body for PATCH /admin/tr/instances/:id. All
// fields are optional; pointer semantics distinguish "omitted" from "set to
// zero value". Password has tri-state semantics (see UpdateInstance).
type updateRequest struct {
	Label         *string `json:"label,omitempty"`
	InstanceID    *string `json:"instanceId,omitempty"`
	BrokerURL     *string `json:"brokerUrl,omitempty"`
	BaseTopic     *string `json:"baseTopic,omitempty"`
	UnitTopic     *string `json:"unitTopic,omitempty"`
	MessageTopic  *string `json:"messageTopic,omitempty"`
	Username      *string `json:"username,omitempty"`
	Password      *string `json:"password,omitempty"`
	TLSSkipVerify *bool   `json:"tlsSkipVerify,omitempty"`
	QoS           *int    `json:"qos,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
} // @name TRInstanceUpdateRequest

// testResponse is the JSON shape returned by POST /admin/tr/instances/:id/test.
type testResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
} // @name TRInstanceTestResponse

// validBrokerSchemes are the URL schemes accepted for broker_url.
var validBrokerSchemes = map[string]struct{}{
	"mqtt":  {},
	"mqtts": {},
	"tcp":   {},
	"ssl":   {},
	"tls":   {},
	"ws":    {},
	"wss":   {},
}

// enabled reports whether the feature is currently active. Returns false
// when the Manager is nil or the trMqttEnabled setting is not "true".
func (h *Handler) enabled(ctx context.Context) bool {
	if h == nil || h.manager == nil {
		return false
	}
	s, err := h.queries.GetSetting(ctx, "trMqttEnabled")
	if err != nil {
		return false
	}
	return s.Value == "true"
}

// disabled writes a 404 and returns true when the feature is off.
func (h *Handler) disabled(c *gin.Context) bool {
	if !h.enabled(c.Request.Context()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "trMqtt disabled"})
		return true
	}
	return false
}

// toResponse converts a DB row + manager snapshot into the JSON response.
func (h *Handler) toResponse(row db.TrInstance) instanceResponse {
	resp := instanceResponse{
		ID:            row.ID,
		Label:         row.Label,
		InstanceID:    row.InstanceID,
		BrokerURL:     row.BrokerUrl,
		BaseTopic:     row.BaseTopic,
		UnitTopic:     row.UnitTopic.String,
		MessageTopic:  row.MessageTopic.String,
		Username:      row.Username.String,
		HasPassword:   row.PasswordEnc.Valid && row.PasswordEnc.String != "",
		TLSSkipVerify: row.TlsSkipVerify == 1,
		QoS:           int(row.Qos),
		Enabled:       row.Enabled == 1,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	if row.LastSeenAt.Valid {
		v := row.LastSeenAt.Int64
		resp.LastSeenAt = &v
	}
	resp.Status = h.statusFor(row)
	return resp
}

// statusFor derives the externally-visible status string for a row.
func (h *Handler) statusFor(row db.TrInstance) string {
	if row.Enabled != 1 {
		return "disabled"
	}
	if h.manager == nil {
		return "disconnected"
	}
	view, ok := h.manager.Snapshot(row.ID)
	if !ok {
		return "disconnected"
	}
	if view.Connection.LastError != "" && !view.Connection.Connected {
		return "error"
	}
	if !view.Connection.Connected {
		return "disconnected"
	}
	return "connected"
}

// validateBrokerURL ensures broker_url parses and uses a supported scheme.
func validateBrokerURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid brokerUrl: %w", err)
	}
	if u.Scheme == "" {
		return errors.New("brokerUrl missing scheme")
	}
	if _, ok := validBrokerSchemes[strings.ToLower(u.Scheme)]; !ok {
		return fmt.Errorf("unsupported brokerUrl scheme %q (want one of mqtt, mqtts, tcp, ssl, tls, ws, wss)", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("brokerUrl missing host")
	}
	return nil
}

// ListInstances handles GET /admin/tr/instances.
//
//	@Summary		List trunk-recorder MQTT instances
//	@Description	Returns every configured tr_instances row with derived live status. Passwords are never included.
//	@Tags			Admin,v1-Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		instanceResponse
//	@Failure		404	{object}	shared.ErrorResponse
//	@Failure		500	{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances [get]
//	@Router			/v1/admin/tr/instances [get]
func (h *Handler) ListInstances(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	rows, err := h.queries.ListTRInstances(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "trmqtt: list instances", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	out := make([]instanceResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, h.toResponse(row))
	}
	c.JSON(http.StatusOK, out)
}

// CreateInstance handles POST /admin/tr/instances.
//
//	@Summary		Create a trunk-recorder MQTT instance
//	@Description	Creates a new tr_instances row. The password is encrypted at rest with AES-256-GCM and never echoed back.
//	@Tags			Admin,v1-Admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createRequest	true	"Instance payload"
//	@Success		201		{object}	instanceResponse
//	@Failure		400		{object}	shared.ErrorResponse
//	@Failure		404		{object}	shared.ErrorResponse
//	@Failure		409		{object}	shared.ErrorResponse
//	@Failure		500		{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances [post]
//	@Router			/v1/admin/tr/instances [post]
func (h *Handler) CreateInstance(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	ctx := c.Request.Context()

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.Label = strings.TrimSpace(req.Label)
	req.InstanceID = strings.TrimSpace(req.InstanceID)
	req.BrokerURL = strings.TrimSpace(req.BrokerURL)
	req.BaseTopic = strings.TrimSpace(req.BaseTopic)

	if req.Label == "" || len(req.Label) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label is required and must be ≤128 chars"})
		return
	}
	if req.InstanceID == "" || len(req.InstanceID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required and must be ≤64 chars"})
		return
	}
	if err := validateBrokerURL(req.BrokerURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.BaseTopic == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseTopic is required"})
		return
	}
	if req.QoS < 0 || req.QoS > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "qos must be 0, 1, or 2"})
		return
	}

	// Uniqueness on label.
	if existing, err := h.queries.GetTRInstanceByLabel(ctx, req.Label); err == nil {
		_ = existing
		c.JSON(http.StatusConflict, gin.H{"error": "label already exists"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.ErrorContext(ctx, "trmqtt: lookup by label", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Encrypt password if provided. When no encryption key is configured we
	// store plaintext (matches downstream API-key behavior; the startup banner
	// warns operators about encryption-at-rest being disabled).
	var passwordEnc sql.NullString
	if req.Password != "" {
		stored := req.Password
		if h.encryptionKey != "" {
			enc, err := auth.EncryptString(req.Password, h.encryptionKey)
			if err != nil {
				slog.ErrorContext(ctx, "trmqtt: encrypt password", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			stored = enc
		}
		passwordEnc = sql.NullString{String: stored, Valid: true}
	}

	enabled := int64(1)
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}
	now := time.Now().Unix()

	row, err := h.queries.CreateTRInstance(ctx, db.CreateTRInstanceParams{
		Label:         req.Label,
		InstanceID:    req.InstanceID,
		BrokerUrl:     req.BrokerURL,
		BaseTopic:     req.BaseTopic,
		UnitTopic:     nullStringFrom(req.UnitTopic),
		MessageTopic:  nullStringFrom(req.MessageTopic),
		Username:      nullStringFrom(req.Username),
		PasswordEnc:   passwordEnc,
		TlsSkipVerify: boolToInt64(req.TLSSkipVerify),
		Qos:           int64(req.QoS),
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		slog.ErrorContext(ctx, "trmqtt: create instance", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if h.manager != nil && enabled == 1 {
		if addErr := h.manager.Add(ctx, row.ID); addErr != nil {
			slog.WarnContext(ctx, "trmqtt: manager.Add after create returned error",
				"instance_id", row.ID, "error", addErr)
		}
	}

	c.JSON(http.StatusCreated, h.toResponse(row))
}

// UpdateInstance handles PATCH /admin/tr/instances/:id.
//
//	@Summary		Update a trunk-recorder MQTT instance
//	@Description	Patch any subset of fields. Omit "password" to keep the existing encrypted value; pass an empty string to clear it; pass a new value to re-encrypt.
//	@Tags			Admin,v1-Admin
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"Instance ID"
//	@Param			body	body		updateRequest	true	"Patch payload"
//	@Success		200		{object}	instanceResponse
//	@Failure		400		{object}	shared.ErrorResponse
//	@Failure		404		{object}	shared.ErrorResponse
//	@Failure		409		{object}	shared.ErrorResponse
//	@Failure		500		{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances/{id} [patch]
//	@Router			/v1/admin/tr/instances/{id} [patch]
func (h *Handler) UpdateInstance(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	ctx := c.Request.Context()

	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	row, err := h.queries.GetTRInstance(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		slog.ErrorContext(ctx, "trmqtt: get instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Apply patches.
	if req.Label != nil {
		v := strings.TrimSpace(*req.Label)
		if v == "" || len(v) > 128 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "label must be 1..128 chars"})
			return
		}
		if v != row.Label {
			if _, err := h.queries.GetTRInstanceByLabel(ctx, v); err == nil {
				c.JSON(http.StatusConflict, gin.H{"error": "label already exists"})
				return
			} else if !errors.Is(err, sql.ErrNoRows) {
				slog.ErrorContext(ctx, "trmqtt: lookup by label", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
		}
		row.Label = v
	}
	if req.InstanceID != nil {
		v := strings.TrimSpace(*req.InstanceID)
		if v == "" || len(v) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "instanceId must be 1..64 chars"})
			return
		}
		row.InstanceID = v
	}
	if req.BrokerURL != nil {
		v := strings.TrimSpace(*req.BrokerURL)
		if err := validateBrokerURL(v); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row.BrokerUrl = v
	}
	if req.BaseTopic != nil {
		v := strings.TrimSpace(*req.BaseTopic)
		if v == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "baseTopic must not be empty"})
			return
		}
		row.BaseTopic = v
	}
	if req.UnitTopic != nil {
		row.UnitTopic = nullStringFrom(*req.UnitTopic)
	}
	if req.MessageTopic != nil {
		row.MessageTopic = nullStringFrom(*req.MessageTopic)
	}
	if req.Username != nil {
		row.Username = nullStringFrom(*req.Username)
	}
	if req.TLSSkipVerify != nil {
		row.TlsSkipVerify = boolToInt64(*req.TLSSkipVerify)
	}
	if req.QoS != nil {
		if *req.QoS < 0 || *req.QoS > 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "qos must be 0, 1, or 2"})
			return
		}
		row.Qos = int64(*req.QoS)
	}
	if req.Enabled != nil {
		row.Enabled = boolToInt64(*req.Enabled)
	}

	// Password tri-state:
	//   nil   → keep existing.
	//   ""    → clear.
	//   other → encrypt and replace.
	if req.Password != nil {
		switch *req.Password {
		case "":
			row.PasswordEnc = sql.NullString{}
		default:
			stored := *req.Password
			if h.encryptionKey != "" {
				enc, err := auth.EncryptString(*req.Password, h.encryptionKey)
				if err != nil {
					slog.ErrorContext(ctx, "trmqtt: encrypt password", "error", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
					return
				}
				stored = enc
			}
			row.PasswordEnc = sql.NullString{String: stored, Valid: true}
		}
	}

	updated, err := h.queries.UpdateTRInstance(ctx, db.UpdateTRInstanceParams{
		Label:         row.Label,
		InstanceID:    row.InstanceID,
		BrokerUrl:     row.BrokerUrl,
		BaseTopic:     row.BaseTopic,
		UnitTopic:     row.UnitTopic,
		MessageTopic:  row.MessageTopic,
		Username:      row.Username,
		PasswordEnc:   row.PasswordEnc,
		TlsSkipVerify: row.TlsSkipVerify,
		Qos:           row.Qos,
		Enabled:       row.Enabled,
		UpdatedAt:     time.Now().Unix(),
		ID:            row.ID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "trmqtt: update instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if h.manager != nil {
		if uErr := h.manager.Update(ctx, updated.ID); uErr != nil {
			slog.WarnContext(ctx, "trmqtt: manager.Update returned error",
				"instance_id", updated.ID, "error", uErr)
		}
	}

	c.JSON(http.StatusOK, h.toResponse(updated))
}

// DeleteInstance handles DELETE /admin/tr/instances/:id.
//
//	@Summary		Delete a trunk-recorder MQTT instance
//	@Description	Disconnects the live client (if any) and removes the row.
//	@Tags			Admin,v1-Admin
//	@Security		BearerAuth
//	@Param			id	path	int	true	"Instance ID"
//	@Success		204
//	@Failure		400	{object}	shared.ErrorResponse
//	@Failure		404	{object}	shared.ErrorResponse
//	@Failure		500	{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances/{id} [delete]
//	@Router			/v1/admin/tr/instances/{id} [delete]
func (h *Handler) DeleteInstance(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	ctx := c.Request.Context()

	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.queries.GetTRInstance(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		slog.ErrorContext(ctx, "trmqtt: get instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if h.manager != nil {
		if rErr := h.manager.Remove(ctx, id); rErr != nil {
			slog.WarnContext(ctx, "trmqtt: manager.Remove returned error",
				"instance_id", id, "error", rErr)
		}
	}

	if err := h.queries.DeleteTRInstance(ctx, id); err != nil {
		slog.ErrorContext(ctx, "trmqtt: delete instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

// TestInstance handles POST /admin/tr/instances/:id/test.
//
//	@Summary		Test a trunk-recorder MQTT instance
//	@Description	One-shot connect and subscribe attempt with a 5-second deadline. Returns {ok:true} on a successful CONNACK or {ok:false,error:"..."} otherwise. The broker password is never included in the error message.
//	@Tags			Admin,v1-Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Instance ID"
//	@Success		200	{object}	testResponse
//	@Failure		400	{object}	shared.ErrorResponse
//	@Failure		404	{object}	shared.ErrorResponse
//	@Failure		500	{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances/{id}/test [post]
//	@Router			/v1/admin/tr/instances/{id}/test [post]
func (h *Handler) TestInstance(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	ctx := c.Request.Context()

	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	row, err := h.queries.GetTRInstance(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		slog.ErrorContext(ctx, "trmqtt: get instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	password, err := h.decryptRowPassword(row)
	if err != nil {
		// Never include the encrypted value or key in the response.
		slog.WarnContext(ctx, "trmqtt: test connect: decrypt password", "instance_id", id, "error", err)
		c.JSON(http.StatusOK, testResponse{OK: false, Error: "failed to decrypt stored password"})
		return
	}

	cfg := trmqttsvc.ClientConfig{
		InstanceID:       row.ID,
		Label:            row.Label,
		PluginInstanceID: row.InstanceID,
		BrokerURL:        row.BrokerUrl,
		BaseTopic:        row.BaseTopic,
		UnitTopic:        row.UnitTopic.String,
		MessageTopic:     row.MessageTopic.String,
		Username:         row.Username.String,
		Password:         password,
		TLSSkipVerify:    row.TlsSkipVerify == 1,
		QoS:              byte(row.Qos),
		ConnectTimeout:   5 * time.Second,
	}

	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := trmqttsvc.TestConnect(testCtx, cfg); err != nil {
		c.JSON(http.StatusOK, testResponse{OK: false, Error: redactErr(err)})
		return
	}
	c.JSON(http.StatusOK, testResponse{OK: true})
}

// ReconnectInstance handles POST /admin/tr/instances/:id/reconnect.
//
//	@Summary		Force-reconnect a trunk-recorder MQTT instance
//	@Tags			Admin,v1-Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	int	true	"Instance ID"
//	@Success		200	{object}	map[string]bool
//	@Failure		400	{object}	shared.ErrorResponse
//	@Failure		404	{object}	shared.ErrorResponse
//	@Failure		500	{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances/{id}/reconnect [post]
//	@Router			/v1/admin/tr/instances/{id}/reconnect [post]
func (h *Handler) ReconnectInstance(c *gin.Context) {
	if h.disabled(c) {
		return
	}
	ctx := c.Request.Context()

	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if _, err := h.queries.GetTRInstance(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}
		slog.ErrorContext(ctx, "trmqtt: get instance", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if err := h.manager.Reconnect(ctx, id); err != nil {
		slog.WarnContext(ctx, "trmqtt: reconnect", "instance_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reconnect failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSnapshot handles GET /admin/tr/instances/:id/snapshot.
//
//	@Summary		Get the live in-memory snapshot for a trunk-recorder MQTT instance
//	@Tags			Admin,v1-Admin
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	int	true	"Instance ID"
//	@Success		200	{object}	map[string]any
//	@Failure		400	{object}	shared.ErrorResponse
//	@Failure		404	{object}	shared.ErrorResponse
//	@Router			/admin/tr/instances/{id}/snapshot [get]
//	@Router			/v1/admin/tr/instances/{id}/snapshot [get]
func (h *Handler) GetSnapshot(c *gin.Context) {
	if h.disabled(c) {
		return
	}

	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	view, ok := h.manager.Snapshot(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not available"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// decryptRowPassword returns the plaintext broker password for row, or "" if
// none is stored. Never logs the value.
func (h *Handler) decryptRowPassword(row db.TrInstance) (string, error) {
	if !row.PasswordEnc.Valid || row.PasswordEnc.String == "" {
		return "", nil
	}
	v := row.PasswordEnc.String
	if !auth.IsEncrypted(v) {
		return v, nil
	}
	if h.encryptionKey == "" {
		return "", errors.New("encryption key not configured")
	}
	return auth.DecryptString(v, h.encryptionKey)
}

// parseID parses a path parameter as an int64.
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// nullStringFrom converts a Go string to sql.NullString, where empty becomes
// NULL.
func nullStringFrom(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// boolToInt64 maps Go bool to the int64 1/0 SQLite uses for booleans.
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// redactErr returns a short, password-free string for client display.
func redactErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	// Defensive: errors from net/url or autopaho should not embed credentials,
	// but trim any "userinfo@host" segment if present.
	if i := strings.Index(s, "@"); i > 0 {
		if j := strings.LastIndex(s[:i], "://"); j >= 0 {
			s = s[:j+3] + "***" + s[i:]
		}
	}
	if len(s) > 256 {
		s = s[:256] + "…"
	}
	return s
}
