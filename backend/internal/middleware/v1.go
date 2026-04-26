// Phase N-1 — middleware specific to the native /api/v1/* surface.
//
// V1Marker tags the gin context so version-aware middleware (currently
// APIKeyAuth) can branch without doing URL-prefix matching.
//
// V1ErrorEnvelope post-processes responses written by handlers that still
// use the legacy `{"error": "<string>"}` shape (notably JWTAuth, RequireAdmin,
// and routes shared between legacy and v1) so that v1 callers always observe
// the canonical {error: {code, message, details}} envelope.
package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/openscanner/openscanner/internal/handler/shared"
)

// V1Marker tags the gin context as part of the /api/v1/* surface.
//
// Other middleware (notably APIKeyAuth) reads c.GetString("apiVersion") to
// branch behaviour by API version without resorting to URL-prefix matching.
func V1Marker() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("apiVersion", "v1")
		c.Next()
	}
}

// envelopeRewriter is a gin.ResponseWriter that buffers the body so it can be
// optionally rewritten before being flushed to the client.
type envelopeRewriter struct {
	gin.ResponseWriter
	buf    bytes.Buffer
	status int
}

func (w *envelopeRewriter) WriteHeader(status int) {
	w.status = status
	// Defer to embedded writer when we flush. Don't propagate yet.
}

func (w *envelopeRewriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *envelopeRewriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

// V1ErrorEnvelope rewrites legacy `{"error":"<string>"}` 4xx/5xx response
// bodies into the native v1 envelope. Bodies already in the native shape
// (`{"error":{"code":...}}`) are passed through untouched, so handlers that
// emit the native envelope directly (the v1 upload handler, etc.) are not
// double-wrapped.
//
// 2xx responses are never rewritten.
func V1ErrorEnvelope() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Wrap the writer so we can inspect/rewrite the body after the
		// handler chain runs.
		orig := c.Writer
		rw := &envelopeRewriter{ResponseWriter: orig, status: 0}
		c.Writer = rw
		defer func() {
			c.Writer = orig
		}()

		c.Next()

		status := rw.status
		if status == 0 {
			status = orig.Status()
		}
		if status == 0 {
			status = http.StatusOK
		}

		body := rw.buf.Bytes()

		// Pass through 2xx untouched.
		if status < 400 {
			orig.WriteHeader(status)
			if len(body) > 0 {
				_, _ = orig.Write(body)
			}
			return
		}

		// Try to detect the legacy `{"error":"<string>"}` shape. If anything
		// else (already native, plain text, empty), pass through unchanged.
		rewritten, ok := rewriteLegacyError(body, status)
		if ok {
			orig.Header().Set("Content-Type", "application/json; charset=utf-8")
			orig.WriteHeader(status)
			_, _ = orig.Write(rewritten)
			return
		}

		orig.WriteHeader(status)
		if len(body) > 0 {
			_, _ = orig.Write(body)
		}
	}
}

// rewriteLegacyError attempts to translate a legacy `{"error":"<string>"}`
// body into the v1 envelope. Returns (newBody, true) on success, (nil,false)
// when the body is not a legacy error shape and should be left alone.
func rewriteLegacyError(body []byte, status int) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, false
	}
	errVal, ok := raw["error"]
	if !ok {
		return nil, false
	}
	// Already-native shape: error is an object with a "code" field. Don't
	// touch.
	if len(errVal) > 0 && errVal[0] == '{' {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(errVal, &inner); err == nil {
			if _, hasCode := inner["code"]; hasCode {
				return nil, false
			}
		}
	}
	// Legacy shape: error is a JSON string.
	var msg string
	if err := json.Unmarshal(errVal, &msg); err != nil {
		return nil, false
	}
	env := shared.APIErrorResponse{
		Error: shared.APIError{
			Code:    shared.DefaultCodeForStatus(status),
			Message: msg,
		},
	}
	out, err := json.Marshal(env)
	if err != nil {
		return nil, false
	}
	return out, true
}
