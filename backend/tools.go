//go:build tools

package tools

// Blank imports to keep all planned dependencies in go.mod.
// These will be moved to actual imports as phases are implemented.
import (
	_ "github.com/SherClockHolmes/webpush-go"
	_ "github.com/coder/websocket"
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/kardianos/service"
	_ "modernc.org/sqlite"
)
