// Package ws — native (v1) WebSocket message constructors.
//
// These produce the JSON-object framed messages described in
// docs/plans/native-api-design-plan.md §4.2. They live alongside the
// legacy 3-letter array-framed constructors in messages.go; the per-client
// encoder selection happens at connect time based on which handler accepted
// the upgrade (legacy /ws, /api/ws, /api/admin/ws vs native
// /api/v1/ws/listener, /api/v1/ws/admin).
package ws

import (
	"encoding/json"
	"time"
)

// Native message type discriminators.
const (
	TypeWelcome         = "connection.welcome"
	TypeRejected        = "connection.rejected"
	TypeScannerConfig   = "scanner.config"
	TypeCallNew         = "call.new"
	TypeCallTranscript  = "call.transcript"
	TypeSessionExpired  = "session.expired"
	TypeListenerCount   = "listener.count"
	TypeFeedMapSnapshot = "listener.feedMap.snapshot"
	TypeFeedMapUpdate   = "listener.feedMap.update"
	TypeAdminEvent      = "admin.event"
	TypeAdminRequest    = "admin.request"
	TypeAdminResponse   = "admin.response"
)

// Native admin error codes used inside admin.response.error.code.
// These mirror the REST error envelope code vocabulary (§7) so the frontend
// can share a single discriminator across REST and WS error surfaces.
const (
	NativeErrCodeValidation = "validation_failed"
	NativeErrCodeUnknownOp  = "unknown_op"
	NativeErrCodeInternal   = "internal_error"
)

// connectionWelcomeV1 is the v1 welcome envelope sent on connect.
type connectionWelcomeV1 struct {
	Type     string `json:"type"`
	Version  string `json:"version"`
	Branding string `json:"branding"`
	Email    string `json:"email"`
}

// NewWelcomeV1 builds a connection.welcome JSON-object frame.
func NewWelcomeV1(version, branding, email string) ([]byte, error) {
	return json.Marshal(connectionWelcomeV1{
		Type:     TypeWelcome,
		Version:  version,
		Branding: branding,
		Email:    email,
	})
}

// NewScannerConfigV1 builds a scanner.config JSON-object frame.
func NewScannerConfigV1(config any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":   TypeScannerConfig,
		"config": config,
	})
}

// NewCallNewV1 builds a call.new JSON-object frame. The call object carries
// the same camelCase metadata fields as the legacy CAL payload (id,
// systemId, talkgroupId, dateTime, audioName, audioType, frequency,
// duration, source, sources, frequencies, errorCount, spikeCount,
// talkerAlias, site, channel, decoder).
func NewCallNewV1(call any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type": TypeCallNew,
		"call": call,
	})
}

// NewSessionExpiredV1 builds a session.expired JSON-object frame.
func NewSessionExpiredV1() ([]byte, error) {
	return json.Marshal(map[string]string{"type": TypeSessionExpired})
}

// NewListenerCountV1 builds a listener.count JSON-object frame.
func NewListenerCountV1(count int) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":  TypeListenerCount,
		"count": count,
	})
}

// NewFeedMapSnapshotV1 builds a listener.feedMap.snapshot JSON-object frame
// (server → client). The client → server counterpart is parsed inline as
// listener.feedMap.update.
func NewFeedMapSnapshotV1(feedMap map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":    TypeFeedMapSnapshot,
		"feedMap": feedMap,
	})
}

// NewRejectedV1 builds a connection.rejected JSON-object frame, sent
// immediately before the server closes the connection (e.g. max clients).
func NewRejectedV1(reason string) ([]byte, error) {
	return json.Marshal(map[string]string{
		"type":   TypeRejected,
		"reason": reason,
	})
}

// NewCallTranscriptV1 builds a call.transcript JSON-object frame. segments
// is omitted when nil to mirror the legacy TRN behaviour.
func NewCallTranscriptV1(callID int64, text string, segments any) ([]byte, error) {
	payload := map[string]any{
		"type":   TypeCallTranscript,
		"callId": callID,
		"text":   text,
	}
	if segments != nil {
		payload["segments"] = segments
	}
	return json.Marshal(payload)
}

// NewAdminEventV1 builds an admin.event JSON-object frame.
func NewAdminEventV1(topic string, data any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":  TypeAdminEvent,
		"topic": topic,
		"at":    time.Now().Unix(),
		"data":  data,
	})
}

// NewAdminResponseV1 builds an admin.response JSON-object frame for a
// successful op.
func NewAdminResponseV1(reqID string, data any) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":  TypeAdminResponse,
		"reqId": reqID,
		"ok":    true,
		"data":  data,
	})
}

// NativeErrorEnvelope mirrors the REST error envelope's error object so the
// frontend can share a single discriminator across REST and WS error
// surfaces (plan §7).
type NativeErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// NewAdminResponseErrorV1 builds an admin.response error JSON-object frame.
// The error object mirrors the REST error envelope shape.
func NewAdminResponseErrorV1(reqID, code, message string, details any) ([]byte, error) {
	env := NativeErrorEnvelope{Code: code, Message: message, Details: details}
	return json.Marshal(map[string]any{
		"type":  TypeAdminResponse,
		"reqId": reqID,
		"ok":    false,
		"error": env,
	})
}

// nativeAdminRequest is the inbound shape for v1 admin request frames.
type nativeAdminRequest struct {
	Type   string          `json:"type"`
	ReqID  string          `json:"reqId"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params,omitempty"`
}

// nativeFeedMapUpdate is the inbound shape for v1 listener.feedMap.update
// frames.
type nativeFeedMapUpdate struct {
	Type    string         `json:"type"`
	FeedMap map[string]any `json:"feedMap"`
}

// nativeEnvelope sniffs the type discriminator on inbound v1 frames.
type nativeEnvelope struct {
	Type string `json:"type"`
}
