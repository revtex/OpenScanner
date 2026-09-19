package trmqtt

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecode_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		into    func() any
		check   func(t *testing.T, v any)
	}{
		{
			name:    "rates",
			payload: `{"type":"rates","instance_id":"site-a","timestamp":1700000000,"rates":[{"sys_num":0,"control_channel":855262500,"decoderate":35.5}]}`,
			into:    func() any { return new(RatesFrame) },
			check: func(t *testing.T, v any) {
				f := v.(*RatesFrame)
				if f.InstanceID != "site-a" {
					t.Fatalf("instance_id: %q", f.InstanceID)
				}
				if len(f.Rates) == 0 {
					t.Fatalf("rates payload empty")
				}
			},
		},
		{
			name:    "config",
			payload: `{"type":"config","instance_id":"site-a","config":{"shortname":"hcrcs","control_channels":[855262500]}}`,
			into:    func() any { return new(ConfigFrame) },
			check: func(t *testing.T, v any) {
				f := v.(*ConfigFrame)
				if f.InstanceID != "site-a" {
					t.Fatalf("instance_id: %q", f.InstanceID)
				}
			},
		},
		{
			name:    "call_start",
			payload: `{"type":"call_start","instance_id":"site-a","call":{"id":"1700_500","talkgroup":500,"unit":12345,"freq":855262500}}`,
			into:    func() any { return new(CallStartFrame) },
			check:   func(t *testing.T, v any) {},
		},
		{
			name:    "message",
			payload: `{"type":"message","instance_id":"site-a","message":{"sys_num":0,"sys_name":"hcrcs","trunk_msg":0,"trunk_msg_type":"GRANT","opcode":"30","opcode_type":"SYNC_BCST","opcode_desc":"Sync Broadcast / Patch"},"timestamp":1700000000}`,
			into:    func() any { return new(MessageFrame) },
			check: func(t *testing.T, v any) {
				f := v.(*MessageFrame)
				if f.InstanceID != "site-a" {
					t.Fatalf("instance_id: %q", f.InstanceID)
				}
				if !bytes.Contains(f.Message, []byte(`"trunk_msg_type":"GRANT"`)) {
					t.Fatalf("message body: %s", string(f.Message))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.into()
			if err := decode([]byte(tc.payload), v); err != nil {
				t.Fatalf("decode: %v", err)
			}
			tc.check(t, v)
		})
	}
}

func TestDecode_UnknownFieldsTolerated(t *testing.T) {
	payload := `{"type":"config","instance_id":"x","new_field_we_dont_know":42,"config":{}}`
	var f ConfigFrame
	if err := decode([]byte(payload), &f); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.InstanceID != "x" {
		t.Fatalf("instance_id: %q", f.InstanceID)
	}
}

func TestDecode_OversizeRejected(t *testing.T) {
	big := bytes.Repeat([]byte("a"), MaxPayloadBytes+1)
	if err := decode(big, &RatesFrame{}); err != ErrOversize {
		t.Fatalf("want ErrOversize, got %v", err)
	}
}

func TestDecode_BadJSON(t *testing.T) {
	if err := decode([]byte(`{"not json`), &RatesFrame{}); err == nil {
		t.Fatalf("want decode error")
	} else if !strings.Contains(err.Error(), "trmqtt decode") {
		t.Fatalf("error wrap missing: %v", err)
	}
}
