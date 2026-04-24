package api_test

import (
	"testing"
)

// TestListenerWSAlias verifies that both /ws (legacy compat alias) and /api/ws
// (canonical) are registered and point at the same handler.
func TestListenerWSAlias(t *testing.T) {
	router, _ := newTestEngine(t)

	var legacyHandler, canonicalHandler string
	for _, rt := range router.Routes() {
		if rt.Method != "GET" {
			continue
		}
		switch rt.Path {
		case "/ws":
			legacyHandler = rt.Handler
		case "/api/ws":
			canonicalHandler = rt.Handler
		}
	}

	if legacyHandler == "" {
		t.Error("GET /ws route is not registered")
	}
	if canonicalHandler == "" {
		t.Error("GET /api/ws route is not registered")
	}
	if legacyHandler != "" && canonicalHandler != "" && legacyHandler != canonicalHandler {
		t.Errorf("listener WS handlers differ: /ws=%q /api/ws=%q",
			legacyHandler, canonicalHandler)
	}
}
