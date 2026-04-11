package api_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openscanner/openscanner/internal/api"
	"github.com/openscanner/openscanner/internal/auth"
	"github.com/openscanner/openscanner/internal/db"
	_ "modernc.org/sqlite"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestDB opens an in-memory SQLite database with all embedded migrations applied.
func newTestDB(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB, db.New(sqlDB)
}

// newTestEngine creates a Gin engine with all routes registered and a fresh
// in-memory database. Returns the engine and queries for seeding.
func newTestEngine(t *testing.T) (*gin.Engine, *db.Queries) {
	t.Helper()
	_, queries := newTestDB(t)

	router := gin.New()
	rl := auth.NewRateLimiter(context.Background())
	api.RegisterRoutes(router, api.Deps{
		Queries:     queries,
		RateLimiter: rl,
		Version:     "test",
	})
	return router, queries
}

// seedAdminUser creates an admin user in the database and marks setup as
// complete, returning the new user ID.
func seedAdminUser(t *testing.T, queries *db.Queries, username, password string) int64 {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().Unix()
	id, err := queries.CreateUser(context.Background(), db.CreateUserParams{
		Username:     username,
		PasswordHash: hash,
		Role:         auth.RoleAdmin,
		Disabled:     0,
		SystemsJson:  sql.NullString{},
		Expiration:   sql.NullInt64{},
		Limit:        sql.NullInt64{},
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	if err := queries.SetSetupComplete(context.Background(), 1); err != nil {
		t.Fatalf("set setup complete: %v", err)
	}
	return id
}
