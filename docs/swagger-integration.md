# Plan: Add Swagger API Documentation Endpoint

## TL;DR

Add swaggo/swag annotation-based Swagger docs to the Go backend. Annotate all 40+ handler functions, generate the spec at build time via `swag init`, and serve Swagger UI at `/api/docs/` using gin-swagger.

## Steps

### Phase 1 — Dependencies & Scaffolding

1. Install Go dependencies: `github.com/swaggo/swag/v2/cmd/swag`, `github.com/swaggo/gin-swagger`, `github.com/swaggo/files`
2. Add general API info annotation block to `backend/cmd/server/main.go` (title, description, version, base path, security definitions for Bearer JWT and API Key)
3. Run `swag init` from `backend/` to generate `backend/docs/` package (docs.go, swagger.json, swagger.yaml)

### Phase 2 — Annotate Handlers (can be parallelized across files)

4. Annotate `backend/internal/api/health.go` — GetHealth (1 endpoint)
5. Annotate `backend/internal/api/setup.go` — GetSetupStatus, PostSetup (2 endpoints)
6. Annotate `backend/internal/api/admin.go` — all auth + admin CRUD handlers (~30 endpoints)
7. Annotate `backend/internal/api/calls.go` — search, audio, upload endpoints (4 endpoints)
8. Annotate `backend/internal/api/share.go` — share CRUD + public shared call/audio (5 endpoints)
9. Annotate `backend/internal/api/webhooks.go` — webhook handler if public-facing

### Phase 3 — Route Registration & Build Integration

10. Register swagger route in `backend/internal/api/routes.go` — add `GET /api/docs/*any` using `ginSwagger.WrapHandler()` before the SPA fallback NoRoute handler
11. Import generated `backend/docs` package in routes.go (blank import)
12. Add `swag init` step to `backend/Makefile` — run before `go build`
13. Commit generated `backend/docs/` for reproducible builds

### Phase 4 — Verification

14. `swag init` succeeds without errors
15. `make build` passes
16. Swagger UI loads at `/api/docs/index.html` with all endpoints grouped by tag
17. "Try it out" on `GET /api/health` works
18. Auth-protected endpoints prompt for JWT in Swagger UI

## Relevant Files

- `backend/cmd/server/main.go` — general API info annotation
- `backend/internal/api/routes.go` — register `/api/docs/*any` route
- `backend/internal/api/admin.go` — annotate ~30 admin handlers
- `backend/internal/api/calls.go` — annotate call handlers
- `backend/internal/api/health.go` — annotate health check
- `backend/internal/api/setup.go` — annotate setup handlers
- `backend/internal/api/share.go` — annotate share handlers
- `backend/internal/api/webhooks.go` — annotate webhook handlers
- `backend/Makefile` — add `swag init` to build pipeline
- `backend/go.mod` — new dependencies

## Decisions

- **swaggo/swag** chosen over hand-written YAML — annotations live next to handler code
- Spec served at `/api/docs/` (Swagger UI) matching plan.md; raw spec at `/api/docs/doc.json`
- No auth on docs endpoints — plan says "always available"
- Define shared `ErrorResponse` struct for consistent swagger docs
- Generated `docs/` package committed to git for reproducible builds
