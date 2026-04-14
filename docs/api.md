# OpenScanner API Reference

OpenScanner exposes an HTTP API and WebSocket feeds for scanner clients and admin tools.

## Canonical contract source

Swagger is the canonical source for endpoint-level contracts:

- request and response schemas
- query and path parameters
- status codes
- security requirements per endpoint

Use Swagger for implementation details and integration code generation.

This page intentionally focuses on behavior, access model, and workflow guidance to avoid schema drift.

## Base path and transport

- HTTP API base path: `/api`
- Listener WebSocket: `/ws`
- Admin WebSocket: `/api/admin/ws`

## Authentication and access model

- Admin and authenticated user routes use JWT Bearer auth in the `Authorization` header.
- Call ingest routes use API key auth in `X-API-Key` (with key query fallback for recorder compatibility).
- Public access mode allows unauthenticated scanner listening and call browsing endpoints intended for public use.
- Admin routes are never public-access, even when public access mode is enabled.
- Swagger UI access is protected by a short-lived server-issued cookie flow.

## Endpoint families

### Public and bootstrap

- `GET /api/health`
- `GET /api/setup/status`
- `POST /api/setup`

### Authentication

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `PUT /api/auth/password`
- `GET /api/auth/me`

### Calls and sharing

- `GET /api/calls`
- `GET /api/calls/:id/audio`
- `POST /api/calls/:id/share`
- `GET /api/calls/:id/share`
- `DELETE /api/calls/:id/share`
- `GET /api/shared/:token`
- `GET /api/shared/:token/audio`

### Bookmarks

- `GET /api/bookmarks`
- `POST /api/bookmarks`
- `GET /api/bookmarks/calls`

### Recorder ingest

- `POST /api/call-upload`
- `POST /api/trunk-recorder-call-upload`

### Admin configuration and CRUD

- `/api/admin/config`
- `/api/admin/users`
- `/api/admin/systems`
- `/api/admin/talkgroups`
- `/api/admin/units`
- `/api/admin/groups`
- `/api/admin/tags`
- `/api/admin/apikeys`
- `/api/admin/fs/directories`
- `/api/admin/dirwatches`
- `/api/admin/downstreams`
- `/api/admin/webhooks`

### Admin operations

- `/api/admin/logs`
- `/api/admin/activity/stats`
- `/api/admin/activity/chart`
- `/api/admin/activity/top-talkgroups`
- `/api/admin/tools/audio-missing`
- `/api/admin/tools/audio-missing/cleanup`
- `/api/admin/import/talkgroups`
- `/api/admin/import/units`
- `/api/admin/export/config`
- `/api/admin/import/config`
- `/api/admin/shared-links`

### Swagger access

- `POST /api/admin/docs/session`
- `GET /api/admin/docs/*`

## Behavioral guidance

### First-run bootstrap

1. Check setup state with `GET /api/setup/status`.
2. If setup is required, create initial admin with `POST /api/setup`.
3. Authenticate via `POST /api/auth/login`.
4. Use admin endpoints to configure systems, talkgroups, ingest, and access settings.

### Call ingest flow

1. Recorder uploads multipart call data to ingest endpoint with API key auth.
2. Server validates required fields and applies duplicate and blacklist logic.
3. Audio is stored and optionally converted based on server settings.
4. Call record is persisted and real-time events are broadcast to listeners.
5. Optional downstream and webhook delivery is triggered by server configuration.

### Sharing flow

1. Authenticated user creates share token for a call.
2. Public clients read shared metadata and audio through token endpoints.
3. Share can be removed by owner or admin.

## WebSocket model

- Listener WS supports public-access or token-auth listener/admin sessions.
- Typical listener welcome sequence includes version and config messages.
- Broadcast stream includes new calls, config updates, listener counts, and session-control events.
- Admin WS is intended for dashboard event consumption and administrative real-time features.

For exact message payload formats and endpoint contracts, rely on Swagger and current backend annotations.

## Swagger UI access flow

1. Authenticate as an admin user to obtain a JWT.
2. Call `POST /api/admin/docs/session` with `Authorization: Bearer <token>` to mint a short-lived HTTP-only docs cookie.
3. Open `GET /api/admin/docs/` to view Swagger UI (cookie-gated).

Contract-level request and response schemas remain Swagger-only to prevent drift.

## Maintenance policy

When API contracts change:

1. Update handler annotations.
2. Regenerate Swagger artifacts.
3. Update this page only for behavioral or workflow changes, not field-by-field schemas.
4. If regenerating from a clean backend tree, remember backend `make clean` removes generated Swagger files in `backend/docs` (`docs.go`, `swagger.json`, `swagger.yaml`).
