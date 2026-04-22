package auth

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
)

// SystemGrant is a system-level listener grant with optional talkgroup filter.
// Shared shape used across WS auth, downstream filtering, and API grant checks.
type SystemGrant struct {
	ID         int64   `json:"id"`
	Talkgroups []int64 `json:"talkgroups,omitempty"`
}

// ParseSystemGrants decodes a NULL-able systems_json column into grants.
// Returns nil when the column is NULL / empty (meaning "all access").
func ParseSystemGrants(sj sql.NullString) []SystemGrant {
	if !sj.Valid || strings.TrimSpace(sj.String) == "" {
		return nil
	}
	var grants []SystemGrant
	if err := json.Unmarshal([]byte(sj.String), &grants); err != nil {
		slog.Warn("auth: failed to parse systems_json", "error", err)
		return nil
	}
	return grants
}

// HasSystemAccess reports whether the given grants permit access to a call on
// (systemID, talkgroupID). Nil/empty grants mean "allow all".
func HasSystemAccess(grants []SystemGrant, systemID, talkgroupID int64) bool {
	if len(grants) == 0 {
		return true
	}
	for _, g := range grants {
		if g.ID != systemID {
			continue
		}
		if len(g.Talkgroups) == 0 {
			return true
		}
		for _, tg := range g.Talkgroups {
			if tg == talkgroupID {
				return true
			}
		}
	}
	return false
}
