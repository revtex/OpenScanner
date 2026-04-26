package ws

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCmd    string
		wantPayld  bool // expect non-nil payload
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:      "valid CAL message",
			input:     `["CAL", {"id": 1, "systemID": 10}]`,
			wantCmd:   "CAL",
			wantPayld: true,
		},
		{
			name:      "valid PIN message",
			input:     `["PIN", "my-access-code"]`,
			wantCmd:   "PIN",
			wantPayld: true,
		},
		{
			name:      "command only, no payload",
			input:     `["XPR"]`,
			wantCmd:   "XPR",
			wantPayld: false,
		},
		{
			name:       "empty array",
			input:      `[]`,
			wantErr:    true,
			wantErrMsg: "empty message",
		},
		{
			name:    "invalid JSON",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:    "missing command — first element is number",
			input:   `[123, "payload"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, payload, err := ParseCommand([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrMsg != "" && err.Error() != tt.wantErrMsg {
					t.Errorf("error = %q, want %q", err.Error(), tt.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if tt.wantPayld && payload == nil {
				t.Error("expected non-nil payload")
			}
			if !tt.wantPayld && payload != nil {
				t.Errorf("expected nil payload, got %s", payload)
			}
		})
	}
}

func TestNewCALMessage(t *testing.T) {
	payload := map[string]any{
		"id":       float64(42),
		"systemID": float64(10),
	}
	data, err := NewCALMessage(payload)
	if err != nil {
		t.Fatalf("NewCALMessage error: %v", err)
	}

	// Strong guard: marshalled bytes must not contain an "audio" key.
	// Embedded base64 audio was removed; clients fetch audio over HTTP.
	if bytes.Contains(data, []byte(`"audio":`)) {
		t.Errorf("CAL message must not contain an \"audio\" field, got: %s", data)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if cmd != "CAL" {
		t.Errorf("command = %q, want %q", cmd, "CAL")
	}

	var body map[string]any
	if err := json.Unmarshal(arr[1], &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if body["id"] != float64(42) {
		t.Errorf("payload id = %v, want 42", body["id"])
	}
	if _, ok := body["audio"]; ok {
		t.Error("payload must not contain an 'audio' key")
	}
}

func TestNewVERMessage(t *testing.T) {
	data, err := NewVERMessage("1.2.3", "OpenScanner", "admin@example.com")
	if err != nil {
		t.Fatalf("NewVERMessage error: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if cmd != "VER" {
		t.Errorf("command = %q, want %q", cmd, "VER")
	}

	var body map[string]string
	if err := json.Unmarshal(arr[1], &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if body["version"] != "1.2.3" {
		t.Errorf("version = %q, want %q", body["version"], "1.2.3")
	}
	if body["branding"] != "OpenScanner" {
		t.Errorf("branding = %q, want %q", body["branding"], "OpenScanner")
	}
	if body["email"] != "admin@example.com" {
		t.Errorf("email = %q, want %q", body["email"], "admin@example.com")
	}
}

func TestNewLSCMessage(t *testing.T) {
	data, err := NewLSCMessage(7)
	if err != nil {
		t.Fatalf("NewLSCMessage error: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if cmd != "LSC" {
		t.Errorf("command = %q, want %q", cmd, "LSC")
	}

	var count float64
	if err := json.Unmarshal(arr[1], &count); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	if count != 7 {
		t.Errorf("count = %v, want 7", count)
	}
}

func TestNewXPRMessage(t *testing.T) {
	data, err := NewXPRMessage()
	if err != nil {
		t.Fatalf("NewXPRMessage error: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if cmd != "XPR" {
		t.Errorf("command = %q, want %q", cmd, "XPR")
	}
}

func TestNewMAXMessage(t *testing.T) {
	data, err := NewMAXMessage()
	if err != nil {
		t.Fatalf("NewMAXMessage error: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element, got %d", len(arr))
	}

	var cmd string
	if err := json.Unmarshal(arr[0], &cmd); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if cmd != "MAX" {
		t.Errorf("command = %q, want %q", cmd, "MAX")
	}
}

// TestLegacyWireFormat_ByteEqual is a Phase N-0 contract-freeze guard for the
// rdio-scanner-shaped legacy WebSocket protocol. Each row pins the exact JSON
// bytes produced by a server-emitted constructor. Any future change to the
// array-framed wire shape (element order, key order, opcode, type
// representation) breaks this test loudly so the upcoming /api/v1/* native API
// work cannot accidentally drift the legacy surface.
//
// Plan reference: docs/plans/native-api-design-plan.md §4.2 (WebSocket
// surface) — the tabled "Legacy command" column is what these bytes pin.
func TestLegacyWireFormat_ByteEqual(t *testing.T) {
	tests := []struct {
		name string
		got  func(t *testing.T) []byte
		want []byte
	}{
		{
			// CAL — payload is metadata only; audio bytes are fetched over HTTP.
			name: "CAL with simple payload",
			got: func(t *testing.T) []byte {
				// json.Marshal of map[string]any sorts keys alphabetically, so
				// the byte output is deterministic for a fixed input.
				b, err := NewCALMessage(map[string]any{
					"id":          float64(42),
					"systemId":    float64(10),
					"talkgroupId": float64(100),
				})
				if err != nil {
					t.Fatalf("NewCALMessage: %v", err)
				}
				return b
			},
			want: []byte(`["CAL",{"id":42,"systemId":10,"talkgroupId":100}]`),
		},
		{
			// CFG — wraps an opaque config payload.
			name: "CFG with config payload",
			got: func(t *testing.T) []byte {
				b, err := NewCFGMessage(map[string]any{
					"systems": []any{},
				})
				if err != nil {
					t.Fatalf("NewCFGMessage: %v", err)
				}
				return b
			},
			want: []byte(`["CFG",{"systems":[]}]`),
		},
		{
			// VER — fixed 3-string struct in fixed key order.
			name: "VER welcome",
			got: func(t *testing.T) []byte {
				b, err := NewVERMessage("1.2.3", "OpenScanner", "admin@example.com")
				if err != nil {
					t.Fatalf("NewVERMessage: %v", err)
				}
				return b
			},
			want: []byte(`["VER",{"branding":"OpenScanner","email":"admin@example.com","version":"1.2.3"}]`),
		},
		{
			name: "LSC listener count",
			got: func(t *testing.T) []byte {
				b, err := NewLSCMessage(7)
				if err != nil {
					t.Fatalf("NewLSCMessage: %v", err)
				}
				return b
			},
			want: []byte(`["LSC",7]`),
		},
		{
			name: "XPR session expired",
			got: func(t *testing.T) []byte {
				b, err := NewXPRMessage()
				if err != nil {
					t.Fatalf("NewXPRMessage: %v", err)
				}
				return b
			},
			want: []byte(`["XPR"]`),
		},
		{
			name: "MAX max clients reached",
			got: func(t *testing.T) []byte {
				b, err := NewMAXMessage()
				if err != nil {
					t.Fatalf("NewMAXMessage: %v", err)
				}
				return b
			},
			want: []byte(`["MAX"]`),
		},
		{
			name: "LFM live feed map",
			got: func(t *testing.T) []byte {
				b, err := NewLFMMessage(map[string]any{
					"1": map[string]any{"100": true},
				})
				if err != nil {
					t.Fatalf("NewLFMMessage: %v", err)
				}
				return b
			},
			want: []byte(`["LFM",{"1":{"100":true}}]`),
		},
		{
			name: "TRN transcript without segments",
			got: func(t *testing.T) []byte {
				b, err := NewTRNMessage(42, "hello world", nil)
				if err != nil {
					t.Fatalf("NewTRNMessage: %v", err)
				}
				return b
			},
			want: []byte(`["TRN",{"callId":42,"text":"hello world"}]`),
		},
		{
			name: "TRN transcript with segments",
			got: func(t *testing.T) []byte {
				b, err := NewTRNMessage(7, "two", []any{
					map[string]any{"start": float64(0), "end": float64(1), "text": "two"},
				})
				if err != nil {
					t.Fatalf("NewTRNMessage: %v", err)
				}
				return b
			},
			want: []byte(`["TRN",{"callId":7,"segments":[{"end":1,"start":0,"text":"two"}],"text":"two"}]`),
		},
		{
			name: "ADM_RES success response",
			got: func(t *testing.T) []byte {
				b, err := NewADMRESMessage("req-1", map[string]any{"ok": true})
				if err != nil {
					t.Fatalf("NewADMRESMessage: %v", err)
				}
				return b
			},
			want: []byte(`["ADM_RES",{"data":{"ok":true},"ok":true,"reqId":"req-1"}]`),
		},
		{
			name: "ADM_RES error response",
			got: func(t *testing.T) []byte {
				b, err := NewADMRESErrorMessage("req-2", "boom")
				if err != nil {
					t.Fatalf("NewADMRESErrorMessage: %v", err)
				}
				return b
			},
			want: []byte(`["ADM_RES",{"error":"boom","ok":false,"reqId":"req-2"}]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.got(t)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("legacy wire shape drift\n got:  %s\n want: %s", got, tt.want)
			}
		})
	}
}

// TestLegacyWireFormat_ADMEVT_ByteEqual pins the ADM_EVT shape but excludes
// the volatile "at" timestamp. The constructor stamps time.Now().Unix() into
// the payload, so we round-trip through the parser and assert the stable
// keys byte-for-byte while only requiring "at" to be a positive integer.
//
// Plan reference: docs/plans/native-api-design-plan.md §4.2 — ADM_EVT row.
func TestLegacyWireFormat_ADMEVT_ByteEqual(t *testing.T) {
	data, err := NewADMEVTMessage("systems.updated", map[string]any{"id": float64(1)})
	if err != nil {
		t.Fatalf("NewADMEVTMessage: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if !bytes.Equal(arr[0], []byte(`"ADM_EVT"`)) {
		t.Errorf("opcode = %s, want \"ADM_EVT\"", arr[0])
	}

	var body struct {
		Topic string         `json:"topic"`
		At    int64          `json:"at"`
		Data  map[string]any `json:"data"`
	}
	if err := json.Unmarshal(arr[1], &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if body.Topic != "systems.updated" {
		t.Errorf("topic = %q, want %q", body.Topic, "systems.updated")
	}
	if body.At <= 0 {
		t.Errorf("at = %d, want positive unix seconds", body.At)
	}
	if body.Data["id"] != float64(1) {
		t.Errorf("data.id = %v, want 1", body.Data["id"])
	}
}
