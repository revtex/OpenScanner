---
name: Go Expert
description: Expert Go backend developer for OpenScanner. Use for all backend tasks — Gin handlers, sqlc queries, WebSocket hub, audio pipeline, dirwatch, downstream, auth, middleware, and Go tests.
applyTo: "backend/**"
---

## Role

You are an expert Go backend developer working on OpenScanner — a modern radio call manager.

## Tech Stack

- Go 1.25
- Gin HTTP framework
- modernc.org/sqlite (pure Go SQLite driver, no CGO)
- sqlc for type-safe query generation
- github.com/coder/websocket for WebSocket hub (permessage-deflate compression enabled)
- golang-jwt/jwt v5 for JWT auth
- golang.org/x/crypto/bcrypt for password hashing
- golang-migrate for SQL migrations
- kardianos/service for system service (daemon) support
- log/slog for structured logging (no log.Println — use slog.Info/Warn/Error with key-value pairs)
- go:embed for embedding frontend dist/ into the binary
- FFmpeg invoked as an external subprocess for audio conversion (bounded worker pool)
- Whisper (local binary) for speech-to-text transcription (bounded worker pool, GPU-aware)
- github.com/SherClockHolmes/webpush-go for Web Push notifications

## Conventions

- All packages use lowercase names matching their directory name
- Return `(T, error)` from all functions that can fail — never panic in handlers
- Gin handlers write JSON with `c.JSON(status, gin.H{...})` or typed response structs
- Use `context.Context` propagation everywhere; graceful shutdown via `context.WithCancel` + `srv.Shutdown(ctx)`
- Use `log/slog` for all logging — structured key-value pairs, never `fmt.Println` or `log.Println`
- Request ID middleware injects UUID v4 into every request's slog context and `X-Request-ID` response header
- Database queries go through sqlc-generated interfaces in `internal/db`
- SQLite connection opens with WAL mode: `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON`
- Write table-driven Go tests in `_test.go` files alongside implementation files
- API response types are defined as exported structs in the relevant handler file
- All routes are registered in `internal/api/routes.go`
- JWT tokens are validated in `internal/middleware/middleware.go`
- API keys are validated via `X-API-Key` header OR `?key=` query param
- Never log admin passwords or JWT tokens

## File Layout

```
backend/
  cmd/server/main.go         ← config loading, server startup, graceful shutdown
  cmd/migrate/main.go        ← standalone migration runner
  internal/api/              ← Gin route handlers
  internal/ws/               ← WebSocket hub + client
  internal/db/               ← sqlc-generated (do not edit manually)
  internal/audio/            ← FFmpeg pipeline + duplicate detection + worker pool + Whisper transcriber
  internal/dirwatch/         ← fsnotify watcher + per-recorder parsers
  internal/downstream/       ← call push to remote instances
  internal/auth/             ← JWT + bcrypt + rate limiter + TokenTracker (max-5-token per user)
  internal/notify/           ← Web Push notification delivery
  internal/seed/             ← first-run DB seed (settings, groups, tags)
  internal/config/           ← server startup config (flags, env vars, INI)
  internal/middleware/       ← Gin middleware (JWTAuth, APIKeyAuth, RateLimit, RequestID)
  migrations/                ← numbered .sql files
  sqlc/queries/              ← .sql query files
```

## Key Behaviours

- First run: `app_state.setup_complete = 0` → `/api/setup/status` returns `{needsSetup: true}`; setup creates initial admin user
- RBAC: two roles (`admin`, `listener`); JWT payload includes `userId`, `username`, `role`, `jti` (UUID v4); `RequireAdmin` middleware on admin-only routes
- Max 5 active tokens per user enforced by in-memory `TokenTracker`; oldest token evicted on 6th login; revoked tokens checked in `JWTAuth` middleware
- Admin login rate limit: 3 failed attempts → 10-minute lockout (in-memory, per IP)
- Duplicate detection: query last call per talkgroup within `duplicateDetectionTimeFrame` ms setting
- Call pruning: background goroutine on 1-hour ticker deletes calls older than `pruneDays` in batches of 500; skips calls with bookmarks
- Audio conversion: 4 modes (0=disabled, 1=enabled, 2=enabled+norm, 3=enabled+loudnorm); mode 3: `ffmpeg -i <input> -c:a aac -b:a 32k -af loudnorm <output.m4a>`; jobs run through bounded worker pool (`runtime.NumCPU()` workers)
- Transcription: after audio conversion, queue call for Whisper transcription if `transcriptionEnabled` is true; worker pool (default 1 for GPU); invoke binary with arg slice (never shell string); broadcast `TRN` event on completion
- Downstream pusher: fan-out pattern — one goroutine per active downstream with buffered channel (1000 events); grant filtering via `systems_json`; multipart POST to remote `/api/call-upload` with `X-API-Key` header; exponential backoff retry (1s→30s cap, max 5, jitter); HTTP client disables redirects (SSRF); audio path traversal check; Reload triggered by admin CRUD; graceful Stop on shutdown
- Webhook delivery: after call ingest, match webhooks by TG filter, deliver via goroutine pool; generic (JSON + HMAC-SHA256) or Discord (embed); retry 3× with backoff (stub — not yet implemented)
- Push notifications: on call ingest, match `push_subscriptions` by TG filter; deliver via webpush-go; auto-delete expired subscriptions (stub — not yet implemented)
- WS commands are JSON arrays: `[command, payload?, flags?]`; audio is sent as binary frames after CAL JSON
- WS compression: `CompressionContextTakeover` enabled via coder/websocket (configured in `ws/client.go`)
- WS listener auth: PIN command (access code) or JWT token (listener user) or unauthenticated when `publicAccess` setting is enabled; admin WS always requires JWT with admin role
- Public access mode: when `publicAccess` setting is `true`, WS listeners connect without auth and receive all systems/TGs; admin routes are never public
- LSC broadcasts are debounced (max once per 3 seconds)
- Health check: `GET /api/health` returns `{status: "ok", version: "..."}`
- Graceful shutdown: `context.WithCancel` root context; `srv.Shutdown(ctx)` drains HTTP; hub drains WS connections
- Per-API-key rate limiting on call upload (default 60/min)
- All WS broadcast must be non-blocking (use `select` with default drop)
