package api

import (
	"database/sql"
	"strings"

	"github.com/openscanner/openscanner/internal/db"
	"github.com/openscanner/openscanner/internal/ws"
)

const maxImportRows = 100_000 // CSV import safety limit.

// AdminHandler handles admin CRUD endpoints.
type AdminHandler struct {
	queries          *db.Queries
	hub              *ws.Hub
	sqlDB            *sql.DB
	dwReload         DirMonitorReloader
	dsReload         DownstreamReloader
	recordingsDir    string
	ffmpegAvailable  bool
	fdkAACAvailable  bool
	whisperAvailable bool
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(queries *db.Queries, hub *ws.Hub, sqlDB *sql.DB, dwReload DirMonitorReloader, dsReload DownstreamReloader, recordingsDir ...string) *AdminHandler {
	rd := "."
	if len(recordingsDir) > 0 && strings.TrimSpace(recordingsDir[0]) != "" {
		rd = recordingsDir[0]
	}
	return &AdminHandler{queries: queries, hub: hub, sqlDB: sqlDB, dwReload: dwReload, dsReload: dsReload, recordingsDir: rd}
}
