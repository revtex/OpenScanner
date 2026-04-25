package shared

import (
	"encoding/json"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
)

// SystemGrant mirrors ws.systemGrant for grant-based filtering in REST handlers.
type SystemGrant struct {
	ID         int64   `json:"id"`
	Talkgroups []int64 `json:"talkgroups,omitempty"`
}

// LoadUserGrants returns the parsed grants for the authenticated user. Returns
// nil (allow-all) for admins, unauthenticated users, or users with no grants.
func LoadUserGrants(c *gin.Context, queries *db.Queries) []SystemGrant {
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if roleStr == auth.RoleAdmin {
		return nil
	}
	userIDVal, exists := c.Get("userID")
	if !exists {
		return nil
	}
	uid, _ := userIDVal.(int64)
	user, err := queries.GetUser(c.Request.Context(), uid)
	if err != nil {
		return nil
	}
	if !user.SystemsJson.Valid || user.SystemsJson.String == "" {
		return nil
	}
	var grants []SystemGrant
	if err := json.Unmarshal([]byte(user.SystemsJson.String), &grants); err != nil {
		slog.Warn("failed to parse user grants", "user_id", uid, "error", err)
		return nil
	}
	if len(grants) == 0 {
		return nil
	}
	return grants
}

// IsGranted checks whether a call with the given system/talkgroup passes the
// grant filter. A nil grant list means everything is allowed.
func IsGranted(grants []SystemGrant, systemID, talkgroupID int64) bool {
	if grants == nil {
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
