# OpenScanner API Guide

OpenScanner exposes an HTTP API plus WebSocket feeds for listener and admin clients.

## Canonical Contract Source

Swagger is the canonical source for endpoint contracts:

- request and response schemas
- query and path parameters
- status codes
- security requirements

This page focuses on behavior and workflows to reduce schema drift.

## Base Paths and Transport

- HTTP API base path: /api
- Listener WebSocket: /ws
- Admin WebSocket: /api/admin/ws

## Authentication Model

- JWT Bearer auth: Authorization header
- API key auth for upload endpoints: X-API-Key header (or key form field compatibility)
- Admin endpoints always require admin JWT
- Some listener routes use optional JWT and allow anonymous access when publicAccess is enabled
- Swagger UI is cookie-gated via POST /api/admin/docs/session

## Endpoint Families

### Public/bootstrap

- GET /api/health
- GET /api/setup/status
- POST /api/setup

### Auth (JWT)

- POST /api/auth/login
- POST /api/auth/logout
- PUT /api/auth/password
- GET /api/auth/me
- GET /api/auth/tg-selection
- PUT /api/auth/tg-selection

### Calls, audio, bookmarks, sharing

- GET /api/calls
- GET /api/calls/{id}/audio
- GET /api/bookmarks
- POST /api/bookmarks
- GET /api/bookmarks/calls
- POST /api/calls/{id}/share
- GET /api/calls/{id}/share
- DELETE /api/calls/{id}/share
- GET /api/shared/{token}
- GET /api/shared/{token}/audio

### Upload

- POST /api/call-upload
- POST /api/trunk-recorder-call-upload

### Admin CRUD

- /api/admin/users
- /api/admin/systems
- /api/admin/talkgroups
- /api/admin/units
- /api/admin/groups
- /api/admin/tags
- /api/admin/apikeys
- /api/admin/dirmonitors
- /api/admin/downstreams
- /api/admin/webhooks

### Admin operations

- GET/PUT /api/admin/config
- GET /api/admin/fs/directories
- GET /api/admin/logs
- GET /api/admin/activity/stats
- GET /api/admin/activity/chart
- GET /api/admin/activity/top-talkgroups
- GET /api/admin/shared-links
- DELETE /api/admin/shared-links/{id}
- GET /api/admin/tools/audio-missing
- POST /api/admin/tools/audio-missing/cleanup
- POST /api/admin/import/talkgroups
- POST /api/admin/import/units
- GET /api/admin/export/talkgroups
- GET /api/admin/export/units
- GET /api/admin/export/config
- POST /api/admin/import/config
- POST /api/admin/radioreference/preview/csv
- POST /api/admin/radioreference/apply

### Swagger session

- POST /api/admin/docs/session
- GET /api/admin/docs/*

## Key Behavioral Notes

### Call search filters

GET /api/calls supports CSV multi-select filters:

- system_ids
- talkgroup_ids
- groups
- tags

Legacy single-value params are also accepted:

- system_id
- talkgroup_id
- group
- tag

Additional filters include date_from, date_to, sort, page, limit, transcript, bookmarked_only.

### Optional auth routes

- GET /api/calls and GET /api/calls/{id}/audio run behind optional JWT middleware
- Anonymous audio access is allowed only when publicAccess=true

### Ingest flow

1. Recorder submits multipart form data to an upload endpoint.
2. API key and rate limits are validated.
3. Auto-populate and duplicate checks run.
4. Audio is stored/converted.
5. Call is persisted and broadcast over WebSocket.
6. Downstream and webhook processing runs when configured.

### Share flow

1. Authenticated user creates a share token.
2. Public clients fetch metadata/audio by token.
3. Owner or admin can unshare.
4. Admin can list/delete shared links globally.

## Swagger UI Access Flow

1. Authenticate as admin and obtain JWT.
2. Call POST /api/admin/docs/session with Authorization header.
3. Open /api/admin/docs/ in browser.
