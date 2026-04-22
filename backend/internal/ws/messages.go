// Package ws — WebSocket command and message type definitions.
package ws

import (
	"encoding/json"
	"time"
)

// Command constants for WebSocket messages.
// All messages are JSON arrays: [command, payload?, flags?]
const (
	CmdCAL = "CAL" // Server → client: new call data
	CmdCFG = "CFG" // Server → client: full config broadcast
	CmdXPR = "XPR" // Server → client: session expired
	CmdLCL = "LCL" // Server → client: paginated call list
	CmdLSC = "LSC" // Server → client: active listeners count
	CmdLFM = "LFM" // Bidirectional: live feed map update
	CmdMAX = "MAX" // Server → client: max clients reached
	CmdVER = "VER" // Server → client: server version + branding + email
	CmdTRN = "TRN" // Server → client: transcript ready for a call

	// Admin WebSocket protocol.
	CmdADMEVT = "ADM_EVT" // Server → admin client: event notification
	CmdADMREQ = "ADM_REQ" // Admin client → server: request
	CmdADMRES = "ADM_RES" // Server → admin client: response
)

// NewCALMessage builds a CAL JSON text frame for a call event.
// When audioData is non-nil, it is added to the payload as "audio" and
// Go's json.Marshal encodes it as a base64 string automatically.
func NewCALMessage(payload map[string]any, audioData []byte) ([]byte, error) {
	if len(audioData) > 0 {
		payload["audio"] = audioData
	}
	return json.Marshal([]any{CmdCAL, payload})
}

// NewCFGMessage builds a CFG message with the full config payload.
func NewCFGMessage(config any) ([]byte, error) {
	return json.Marshal([]any{CmdCFG, config})
}

// NewVERMessage builds a VER message with server version, branding, and email.
func NewVERMessage(version, branding, email string) ([]byte, error) {
	return json.Marshal([]any{CmdVER, map[string]string{
		"version":  version,
		"branding": branding,
		"email":    email,
	}})
}

// NewLSCMessage builds an LSC message with the current listener count.
func NewLSCMessage(count int) ([]byte, error) {
	return json.Marshal([]any{CmdLSC, count})
}

// NewXPRMessage builds an XPR (session expired) message.
func NewXPRMessage() ([]byte, error) {
	return json.Marshal([]any{CmdXPR})
}

// NewMAXMessage builds a MAX (max clients reached) message.
func NewMAXMessage() ([]byte, error) {
	return json.Marshal([]any{CmdMAX})
}

// NewLFMMessage builds an LFM (live feed map) message.
func NewLFMMessage(feedMap map[string]any) ([]byte, error) {
	return json.Marshal([]any{CmdLFM, feedMap})
}

// NewLCLMessage builds an LCL (call list) message.
func NewLCLMessage(calls any, total int64) ([]byte, error) {
	return json.Marshal([]any{CmdLCL, map[string]any{
		"calls": calls,
		"total": total,
	}})
}

// NewTRNMessage builds a TRN (transcript ready) message.
// When segments is non-nil, they are included for diarization-aware clients.
func NewTRNMessage(callID int64, text string, segments any) ([]byte, error) {
	payload := map[string]any{
		"callId": callID,
		"text":   text,
	}
	if segments != nil {
		payload["segments"] = segments
	}
	return json.Marshal([]any{CmdTRN, payload})
}

// ParseCommand extracts the command string from a JSON array message.
// Returns the command and the raw payload (second element) if present.
func ParseCommand(data []byte) (cmd string, payload json.RawMessage, err error) {
	var arr []json.RawMessage
	if err = json.Unmarshal(data, &arr); err != nil {
		return "", nil, err
	}
	if len(arr) == 0 {
		return "", nil, ErrEmptyMessage
	}
	if err = json.Unmarshal(arr[0], &cmd); err != nil {
		return "", nil, err
	}
	if len(arr) > 1 {
		payload = arr[1]
	}
	return cmd, payload, nil
}

// ErrEmptyMessage is returned when a WebSocket message has no elements.
var ErrEmptyMessage = &wsError{"empty message"}

type wsError struct{ msg string }

func (e *wsError) Error() string { return e.msg }

// NewADMEVTMessage builds an admin event message.
// ["ADM_EVT", {"topic": "...", "at": unix_seconds, "data": ...}]
func NewADMEVTMessage(topic string, data any) ([]byte, error) {
	return json.Marshal([]any{CmdADMEVT, map[string]any{
		"topic": topic,
		"at":    time.Now().Unix(),
		"data":  data,
	}})
}

// NewADMRESMessage builds an admin response message.
// ["ADM_RES", {"reqId": "...", "ok": true, "data": ...}]
func NewADMRESMessage(reqID string, data any) ([]byte, error) {
	return json.Marshal([]any{CmdADMRES, map[string]any{
		"reqId": reqID,
		"ok":    true,
		"data":  data,
	}})
}

// NewADMRESErrorMessage builds an admin error response.
// ["ADM_RES", {"reqId": "...", "ok": false, "error": "..."}]
func NewADMRESErrorMessage(reqID string, errMsg string) ([]byte, error) {
	return json.Marshal([]any{CmdADMRES, map[string]any{
		"reqId": reqID,
		"ok":    false,
		"error": errMsg,
	}})
}
