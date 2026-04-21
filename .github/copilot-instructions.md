# OpenScanner — Copilot Instructions

## Project Overview

OpenScanner is a modern web-based radio call manager — a reimplementation of rdio-scanner using Go + React.

## Tech Stack

- **Backend:** Go 1.25, Gin, coder/websocket, modernc.org/sqlite, sqlc, golang-jwt, bcrypt, golang-migrate, kardianos/service, log/slog, go:embed, webpush-go
- **Frontend:** React 18, TypeScript (strict), Vite, Tailwind CSS 4 (@tailwindcss/vite), DaisyUI 5 (dark/light themes), Redux Toolkit, RTK Query, @tanstack/react-virtual, Service Worker (PWA + push notifications)
- **Database:** SQLite (WAL mode) — all application configuration is stored in the DB
- **Server config:** CLI flags, environment variables, or optional INI file (for listen address, DB path, TLS)
- **Audio:** Files stored on filesystem, FFmpeg for conversion (4 modes: disabled/enabled/norm/loudnorm), bounded worker pool
- **Transcription:** go-whisper HTTP API sidecar (whisper.cpp, CPU or GPU); supports diarization via tinydiarize model
- **Dev tooling:** air (Go hot-reload) + Vite proxy (single `make dev`)

## Project Structure

```
openscanner/
  backend/     ← Go backend
  frontend/    ← React frontend
  docs/        ← Documentation
  .github/agents/  ← Expert agents for each domain
```

## Coding Conventions

### Go

- All errors returned, never panicked in HTTP handlers
- Gin handlers use typed response structs
- SQL via sqlc only — no raw string queries
- JWT in `Authorization: Bearer` header; API keys in `X-API-Key` header
- Tests are table-driven, use `httptest` for API tests
- Use `log/slog` for all logging — never `log.Println` or `fmt.Println`
- Use `context.Context` propagation; graceful shutdown via `context.WithCancel`

### TypeScript / React

- Strict mode; no `any` types
- All imports use `@/` alias for `src/`
- All server data goes through RTK Query slices
- WS events dispatch to Redux

## Security Rules (OWASP Top 10 — always enforce)

1. All admin routes require valid JWT with admin role
2. Call upload routes require valid API key
3. No SQL string concatenation — sqlc only
4. bcrypt cost ≥ 12 for passwords
5. No secrets in logs or error responses
6. Audio file paths sanitised — no `../` traversal
7. FFmpeg invoked with arg slice, never shell string; go-whisper accessed via HTTP only (no subprocess)
8. Role-based access: listener JWT cannot access admin endpoints (403)
9. Public access mode (`publicAccess` setting) allows unauthenticated scanner listening; admin routes are never public
10. Webhook secrets use HMAC-SHA256 for payload signing
11. VAPID keys stored in settings table; push subscriptions validated before delivery

## Agent Assignment

- Go backend tasks → use **Go Expert** agent
- React/TypeScript tasks → use **React Expert** agent
- Schema/query tasks → use **Database Expert** agent
- Documentation tasks → use **Docs Expert** agent
- Code review/security → use **Reviewer** agent
- Test writing → use **Testing Expert** agent
