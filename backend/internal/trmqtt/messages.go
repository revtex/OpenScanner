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

// MessageFrame — trunking-control-channel message (debug pane).
type MessageFrame struct {
	frameCommon
	System      json.Number     `json:"sys_num,omitempty"`
	Shortname   string          `json:"shortname,omitempty"`
	MessageType string          `json:"message_type,omitempty"`
	Opcode      json.Number     `json:"opcode,omitempty"`
	OpcodeDesc  string          `json:"opcode_desc,omitempty"`
	Meta        json.RawMessage `json:"meta,omitempty"`
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
// field carries which unit topic delivered it. Unknown fields are preserved
// in Extra.
type UnitFrame struct {
	frameCommon
	Kind        UnitEventKind   `json:"-"`
	Shortname   string          `json:"shortname,omitempty"`
	UnitID      json.Number     `json:"unit_id,omitempty"`
	UnitAlpha   string          `json:"unit_alpha,omitempty"`
	TalkgroupID json.Number     `json:"talkgroup,omitempty"`
	TGAlpha     string          `json:"talkgroup_alpha_tag,omitempty"`
	Patches     json.RawMessage `json:"patches,omitempty"`
	Extra       json.RawMessage `json:"extra,omitempty"`
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
