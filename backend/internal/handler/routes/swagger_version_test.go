package routes_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/openscanner/openscanner/docs"
	"github.com/openscanner/openscanner/internal/handler/routes"
)

// The @version annotation in main.go is a build-time literal, so it goes
// stale as soon as a release is cut (it read "1.0" through v1.3.3). The
// served docs must report the binary's real version instead.
func TestSwaggerVersionReportsBuildVersion(t *testing.T) {
	original := docs.SwaggerInfo.Version
	t.Cleanup(func() { docs.SwaggerInfo.Version = original })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.RegisterRoutes(router, routes.Deps{Version: "9.9.9"})

	if got := docs.SwaggerInfo.Version; got != "9.9.9" {
		t.Errorf("SwaggerInfo.Version = %q, want %q", got, "9.9.9")
	}
}

// A zero Deps (as several tests pass) must not blank out the version.
func TestSwaggerVersionLeftAloneWhenUnset(t *testing.T) {
	original := docs.SwaggerInfo.Version
	t.Cleanup(func() { docs.SwaggerInfo.Version = original })

	docs.SwaggerInfo.Version = "sentinel"
	gin.SetMode(gin.TestMode)
	routes.RegisterRoutes(gin.New(), routes.Deps{})

	if got := docs.SwaggerInfo.Version; got != "sentinel" {
		t.Errorf("SwaggerInfo.Version = %q, want it unchanged", got)
	}
}
