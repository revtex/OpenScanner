// Package trmqtt implements an in-process MQTT subscriber for the
// trunk-recorder MQTT status plugin. One Client per tr_instances row,
// supervised reconnect via eclipse/paho.golang/autopaho.
package trmqtt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// MaxPayloadBytes is the hard cap on inbound MQTT payload size. Anything
// larger is dropped, counted via metrics.dropOversize, and logged at warn
// (without payload contents).
const MaxPayloadBytes = 256 * 1024

// ErrOversize is returned by decode helpers when a payload exceeds MaxPayloadBytes.
var ErrOversize = errors.New("payload exceeds 256 KB cap")

// frameCommon contains the fields every TR plugin frame carries.
// Decoders ignore unknown extra fields; json.Number preserves precision for
// large integer IDs (e.g. unit IDs).
type frameCommon struct {
	Type       string      `json:"type,omitempty"`
	InstanceID string      `json:"instance_id,omitempty"`
	Timestamp  json.Number `json:"timestamp,omitempty"`
}

// RatesFrame — per-system control-channel decode rates.
type RatesFrame struct {
	frameCommon
	Rates json.RawMessage `json:"rates,omitempty"`
}

// RecordersFrame — recorder pool snapshot (idle / available / recording).
type RecordersFrame struct {
	frameCommon
	Recorders json.RawMessage `json:"recorders,omitempty"`
}

// CallsActiveFrame — currently in-flight calls across all systems.
type CallsActiveFrame struct {
	frameCommon
	Calls json.RawMessage `json:"calls,omitempty"`
}

// CallStartFrame — emitted when TR starts recording a call.
type CallStartFrame struct {
	frameCommon
	Call json.RawMessage `json:"call,omitempty"`
}

// CallEndFrame — emitted when TR finishes recording a call.
type CallEndFrame struct {
	frameCommon
	Call json.RawMessage `json:"call,omitempty"`
}

// SystemFrame — single-system status update.
type SystemFrame struct {
	frameCommon
	System json.RawMessage `json:"system,omitempty"`
}

// SystemsFrame — full systems list (retained).
type SystemsFrame struct {
	frameCommon
	Systems json.RawMessage `json:"systems,omitempty"`
}

// ConfigFrame — TR config snapshot (retained).
type ConfigFrame struct {
	frameCommon
	Config json.RawMessage `json:"config,omitempty"`
}

// PluginStatusFrame — TR plugin lifecycle (start/stop).
type PluginStatusFrame struct {
	frameCommon
	Status string `json:"status,omitempty"`
}

// MessageFrame — trunking-control-channel message (debug pane). The TR
// plugin nests the message body under a "message" key alongside the standard
// frameCommon fields (`{type, message:{...}, timestamp, instance_id}`), so the
// inner blob is captured as a RawMessage and forwarded to the frontend
// untouched. Frontend extracts trunk_msg_type/opcode/opcode_desc/sys_name from
// the nested object.
type MessageFrame struct {
	frameCommon
	Message json.RawMessage `json:"message,omitempty"`
}

// UnitEventKind enumerates the per-unit-topic event kinds.
type UnitEventKind string

const (
	UnitEventOn       UnitEventKind = "on"
	UnitEventOff      UnitEventKind = "off"
	UnitEventJoin     UnitEventKind = "join"
	UnitEventLocation UnitEventKind = "location"
	UnitEventCall     UnitEventKind = "call"
	UnitEventEnd      UnitEventKind = "end"
	UnitEventData     UnitEventKind = "data"
	UnitEventAckResp  UnitEventKind = "ackresp"
	UnitEventAnsReq   UnitEventKind = "ans_req"
)

// UnitFrame is the canonical decoded form of any unit-topic frame. The Kind
// field carries which unit topic delivered it.
//
// The TR plugin envelopes unit payloads as `{type, <kind>:{body}, timestamp,
// instance_id}` where <kind> is the unit event name ("on", "off", "call",
// "join", "data", "location", "end", "ackresp", "ans_req"). Use
// decodeUnit() to flatten that envelope into UnitFrame.
//
// Field names mirror the plugin's get_unit_json / get_unit_tg_json output:
// `unit` (not `unit_id`), `unit_alpha_tag` (not `unit_alpha`), `sys_name`
// (not `shortname`). The full nested body is preserved in Body for any
// extra fields the frontend wants to surface.
type UnitFrame struct {
	frameCommon
	Kind        UnitEventKind   `json:"kind,omitempty"`
	SysNum      json.Number     `json:"sys_num,omitempty"`
	Shortname   string          `json:"sys_name,omitempty"`
	UnitID      json.Number     `json:"unit,omitempty"`
	UnitAlpha   string          `json:"unit_alpha_tag,omitempty"`
	TalkgroupID json.Number     `json:"talkgroup,omitempty"`
	TGAlpha     string          `json:"talkgroup_alpha_tag,omitempty"`
	TGGroup     string          `json:"talkgroup_group,omitempty"`
	TGTag       string          `json:"talkgroup_tag,omitempty"`
	TGPatches   string          `json:"talkgroup_patches,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
}

// decodeUnit flattens a nested unit payload `{type, <kind>:{body}, ...}`
// into a UnitFrame, copying frameCommon fields and decoding the kind body
// into the inner fields. Returns false on size/decode error.
func decodeUnit(payload []byte, kind UnitEventKind, f *UnitFrame) error {
	if len(payload) > MaxPayloadBytes {
		return ErrOversize
	}
	var env map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&env); err != nil {
		return fmt.Errorf("trmqtt decode unit: %w", err)
	}
	if raw, ok := env["type"]; ok {
		_ = json.Unmarshal(raw, &f.Type)
	}
	if raw, ok := env["instance_id"]; ok {
		_ = json.Unmarshal(raw, &f.InstanceID)
	}
	if raw, ok := env["timestamp"]; ok {
		f.Timestamp = json.Number(bytes.TrimSpace(raw))
	}
	body, ok := env[string(kind)]
	if ok && len(body) > 0 {
		bodyDec := json.NewDecoder(bytes.NewReader(body))
		bodyDec.UseNumber()
		if err := bodyDec.Decode(f); err != nil {
			return fmt.Errorf("trmqtt decode unit body: %w", err)
		}
		f.Body = append(f.Body[:0], body...)
	}
	f.Kind = kind
	return nil
}

// decode unmarshals payload into v using a json.Decoder configured with
// UseNumber so large ints don't lose precision. Returns ErrOversize when the
// payload exceeds MaxPayloadBytes.
func decode(payload []byte, v any) error {
	if len(payload) > MaxPayloadBytes {
		return ErrOversize
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("trmqtt decode: %w", err)
	}
	return nil
}
