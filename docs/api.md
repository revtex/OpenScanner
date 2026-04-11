# OpenScanner — API Reference

> Placeholder — OpenAPI 3.1 spec to be completed during Phase 14.
> The server will serve Swagger UI at `/api/docs`.

## Endpoints

### Setup
- `GET /api/setup/status`
- `POST /api/setup`

### Admin Auth
- `POST /api/admin/login`
- `POST /api/admin/logout`
- `PUT /api/admin/password`

### Call Ingest
- `POST /api/call-upload`
- `POST /api/trunk-recorder-call-upload`

### Config
- `GET /api/admin/config`
- `PUT /api/admin/config`

### Admin CRUD
- `/api/admin/systems`
- `/api/admin/talkgroups`
- `/api/admin/units`
- `/api/admin/groups`
- `/api/admin/tags`
- `/api/admin/api-keys`
- `/api/admin/accesses`
- `/api/admin/dirwatches`
- `/api/admin/downstreams`
- `GET /api/admin/logs`

### WebSocket
- `GET /ws` — listener socket
- `GET /api/admin/ws` — admin dashboard socket
