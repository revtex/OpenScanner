# OpenScanner — Admin Guide

> The admin dashboard UI is planned for Phase 11. This guide documents the backend admin API that is already functional. All endpoints require JWT authentication with the `admin` role.

## Authentication

All admin endpoints require `Authorization: Bearer <jwt>` with `admin` role. Obtain a token via `POST /api/auth/login`.

## Admin API Endpoints

All admin endpoints are under `/api/admin/`. Full request/response details are in [api.md](api.md#admin-crud-endpoints).

### Resource Management (CRUD)

Each resource supports **GET** (list), **POST** (create), **PUT /:id** (update), **DELETE /:id** (delete):

| Resource | Base Path | Notes |
|----------|-----------|-------|
| Users | `/api/admin/users` | Cannot delete own account; password hashed on create; role: `admin` or `listener` |
| Systems | `/api/admin/systems` | `system_id` must be unique |
| Talkgroups | `/api/admin/talkgroups` | Unique per system (`system_id` + `talkgroup_id`) |
| Units | `/api/admin/units` | Unique per system |
| Groups | `/api/admin/groups` | Label must be unique |
| Tags | `/api/admin/tags` | Label must be unique |
| API Keys | `/api/admin/apikeys` | Auto-generates UUID v4 key if not provided |
| Accesses | `/api/admin/accesses` | Access code required; for anonymous listener access |
| Dirwatches | `/api/admin/dirwatches` | Directory path required |
| Downstreams | `/api/admin/downstreams` | URL required |
| Webhooks | `/api/admin/webhooks` | URL required |

### Configuration

- **GET /api/admin/config** — Returns all settings as a `{key: value}` JSON object
- **PUT /api/admin/config** — Updates settings; unknown keys are rejected; broadcasts `CFG` to all WebSocket clients

### Logs

- **GET /api/admin/logs** — Query params: `from` (unix), `to` (unix), `level` (`info`/`warn`/`error`). Max 10,000 rows returned.

### Import / Export

- **POST /api/admin/import/talkgroups** — Multipart CSV upload with `system_id` form field
- **POST /api/admin/import/units** — Multipart CSV upload with `system_id` form field
- **GET /api/admin/export/config** — Downloads full config as JSON
- **POST /api/admin/import/config** — Imports full config JSON (transactional; duplicates skipped)

## Planned Coverage (Phase 11 — Admin Dashboard UI)

- First-run setup wizard
- Dashboard panels
- Systems / talkgroups / units management UI
- Access codes management UI
- API keys management UI
- DirWatch configuration UI
- Downstream instances UI
- Options / settings UI
- Logs viewer UI
- CSV import UI
- JSON config export/import UI
