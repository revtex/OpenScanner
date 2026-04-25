package shared

import (
	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/db"
)

// GetSettingValue fetches a setting value from the DB, returning "" on error.
func GetSettingValue(c *gin.Context, queries *db.Queries, key string) string {
	s, err := queries.GetSetting(c.Request.Context(), key)
	if err != nil {
		return ""
	}
	return s.Value
}

// MaxImportRows is the CSV import safety limit.
const MaxImportRows = 100_000
