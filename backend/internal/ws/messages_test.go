package ws

import (
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
	data, err := NewCALMessage(payload, nil)
	if err != nil {
		t.Fatalf("NewCALMessage error: %v", err)
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
	// Audio should be absent when nil is passed.
	if _, ok := body["audio"]; ok {
		t.Error("expected no 'audio' key when audioData is nil")
	}
}

func TestNewCALMessage_WithAudio(t *testing.T) {
	payload := map[string]any{
		"id": float64(1),
	}
	audio := []byte{0xFF, 0xFB, 0x90, 0x00} // fake MP3 header
	data, err := NewCALMessage(payload, audio)
	if err != nil {
		t.Fatalf("NewCALMessage error: %v", err)
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("not valid JSON array: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(arr[1], &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	audioB64, ok := body["audio"].(string)
	if !ok || audioB64 == "" {
		t.Fatal("expected base64 'audio' field in payload")
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
