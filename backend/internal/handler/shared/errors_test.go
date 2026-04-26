package shared

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteAPIError_TableDriven(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		status      int
		code        string
		message     string
		details     map[string]any
		injectReqID string
		wantReqID   bool
	}{
		{"400 validation_failed", http.StatusBadRequest, CodeValidationFailed, "talkgroupId required", map[string]any{"field": "talkgroupId"}, "", false},
		{"401 invalid_credentials", http.StatusUnauthorized, CodeInvalidCredentials, "bearer missing", nil, "", false},
		{"403 forbidden", http.StatusForbidden, CodeForbidden, "admin required", nil, "", false},
		{"404 call_not_found", http.StatusNotFound, CodeCallNotFound, "no call with id 1", map[string]any{"id": 1}, "", false},
		{"409 duplicate_call", http.StatusConflict, CodeDuplicateCall, "dup", map[string]any{"existingId": 9001}, "", false},
		{"422 unprocessable", http.StatusUnprocessableEntity, CodeSystemNotFound, "system 502 not configured", map[string]any{"systemId": 502}, "", false},
		{"429 rate_limited", http.StatusTooManyRequests, CodeRateLimited, "slow down", map[string]any{"retryAfterSeconds": 30}, "", false},
		{"500 internal_error injects requestId", http.StatusInternalServerError, CodeInternalError, "boom", nil, "req-xyz", true},
		{"500 keeps existing requestId in details", http.StatusInternalServerError, CodeInternalError, "boom", map[string]any{"requestId": "preset"}, "req-xyz", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if tc.injectReqID != "" {
				c.Set("requestID", tc.injectReqID)
			}
			WriteAPIError(c, tc.status, tc.code, tc.message, tc.details)

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			var env APIErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode: %v\nbody: %s", err, w.Body.String())
			}
			if env.Error.Code != tc.code {
				t.Errorf("code = %q, want %q", env.Error.Code, tc.code)
			}
			if env.Error.Message != tc.message {
				t.Errorf("message = %q, want %q", env.Error.Message, tc.message)
			}
			if tc.wantReqID {
				rid, ok := env.Error.Details["requestId"].(string)
				if !ok || rid == "" {
					t.Errorf("expected details.requestId to be set; got %v", env.Error.Details)
				}
				// Existing preset values must be preserved (not overwritten).
				if pre, ok := tc.details["requestId"].(string); ok && pre != "" {
					if rid != pre {
						t.Errorf("requestId overwrote preset value: got %q, want %q", rid, pre)
					}
				}
			}
		})
	}
}

func TestDefaultCodeForStatus(t *testing.T) {
	cases := map[int]string{
		http.StatusBadRequest:          CodeValidationFailed,
		http.StatusUnauthorized:        CodeInvalidCredentials,
		http.StatusForbidden:           CodeForbidden,
		http.StatusNotFound:            CodeNotFound,
		http.StatusConflict:            CodeConflict,
		http.StatusUnprocessableEntity: CodeUnprocessable,
		http.StatusTooManyRequests:     CodeRateLimited,
		http.StatusInternalServerError: CodeInternalError,
		http.StatusServiceUnavailable:  CodeInternalError,
		418:                            CodeValidationFailed, // unknown 4xx → validation_failed default
	}
	for status, want := range cases {
		if got := DefaultCodeForStatus(status); got != want {
			t.Errorf("DefaultCodeForStatus(%d) = %q, want %q", status, got, want)
		}
	}
}
