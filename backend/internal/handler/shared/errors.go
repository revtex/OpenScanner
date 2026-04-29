// Phase N-1 — RFC-style error envelope used by every /api/v1/* handler.
//
// All native v1 error responses share the shape:
//
//	{
//	  "error": {
//	    "code":    "validation_failed",
//	    "message": "human-readable english",
//	    "details": { ... } // optional, endpoint-specific
//	  }
//	}
//
// See docs/plans/native-api-design-plan.md §7 for the full contract.
package shared

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Native v1 error codes. Not exhaustive — handlers may emit endpoint-specific
// codes — but these are the canonical ones used by the middleware-level
// envelope rewriter and the most common 4xx/5xx branches.
const (
	CodeValidationFailed   = "validation_failed"
	CodeInvalidCredentials = "invalid_credentials"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeCallNotFound       = "call_not_found"
	CodeConflict           = "conflict"
	CodeDuplicateCall      = "duplicate_call"
	CodeUnprocessable      = "unprocessable"
	CodeSystemNotFound     = "system_not_found"
	CodeTalkgroupNotFound  = "talkgroup_not_found"
	CodeRateLimited        = "rate_limited"
	CodeInternalError      = "internal_error"
)

// APIError is the inner object of the native v1 error envelope.
type APIError struct {
	Code    string         `json:"code"`              // stable machine identifier
	Message string         `json:"message"`           // human-readable English
	Details map[string]any `json:"details,omitempty"` // endpoint-specific
} // @name APIError

// APIErrorResponse is the outer JSON envelope: `{"error": {...}}`.
type APIErrorResponse struct {
	Error APIError `json:"error"`
} // @name APIErrorResponse

// WriteAPIError emits the native error envelope and aborts the gin context.
//
// For 5xx responses, the request id (set by middleware.RequestID) is auto-
// injected into details.requestId so operators can correlate the response
// with server logs without leaking other internals.
func WriteAPIError(c *gin.Context, status int, code, message string, details map[string]any) {
	if status >= http.StatusInternalServerError {
		if rid, ok := c.Get("requestID"); ok {
			if ridStr, ok := rid.(string); ok && ridStr != "" {
				if details == nil {
					details = map[string]any{}
				}
				if _, exists := details["requestId"]; !exists {
					details["requestId"] = ridStr
				}
			}
		}
	}
	c.AbortWithStatusJSON(status, APIErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// DefaultCodeForStatus returns the canonical v1 error code for a status code.
// Used by the envelope-rewriter middleware to translate legacy
// `{"error": "..."}` responses emitted by shared handlers into the native shape.
func DefaultCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeValidationFailed
	case http.StatusUnauthorized:
		return CodeInvalidCredentials
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusUnprocessableEntity:
		return CodeUnprocessable
	case http.StatusTooManyRequests:
		return CodeRateLimited
	default:
		if status >= 500 {
			return CodeInternalError
		}
		return CodeValidationFailed
	}
}
