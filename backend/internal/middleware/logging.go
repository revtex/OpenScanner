package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func requestLogLevel(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "error"
	case status >= http.StatusBadRequest:
		return "warn"
	default:
		return "info"
	}
}

// RequestID adds a UUID v4 X-Request-ID response header and stores it in the
// Gin context under the key "requestID".
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("requestID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger emits a structured slog line for every request including method, path,
// status code, latency, request ID, and client IP.
// Health check probes and CORS preflight requests are logged at Debug only so
// they don't drown out real traffic in normal operation.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		level := requestLogLevel(status)

		var slogLevel slog.Level
		switch level {
		case "error":
			slogLevel = slog.LevelError
		case "warn":
			slogLevel = slog.LevelWarn
		default:
			slogLevel = slog.LevelInfo
		}

		// Demote noisy low-signal endpoints to Debug when they succeed.
		path := c.Request.URL.Path
		if slogLevel == slog.LevelInfo &&
			(path == "/api/health" || c.Request.Method == http.MethodOptions) {
			slogLevel = slog.LevelDebug
		}

		requestID, _ := c.Get("requestID")
		slog.Log(c.Request.Context(), slogLevel, "request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"request_id", requestID,
			"ip", c.ClientIP(),
		)
	}
}
