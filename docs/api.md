# OpenScanner — API Reference

> **Implementation status:** All endpoints listed below are implemented and available.

## Implemented Endpoints

### Health

#### `GET /api/health`

Unauthenticated readiness check.

**Response `200`:**

```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

### Setup

#### `GET /api/setup/status`

Returns whether initial setup is needed and whether public access is enabled. Unauthenticated.

**Response `200`:**

```json
{
  "needsSetup": true,
  "publicAccess": false
}
```

#### `POST /api/setup`

Creates the initial admin user and marks setup as complete. Mutex-protected to prevent TOCTOU race conditions. Unauthenticated (only works when `needsSetup` is `true`).

**Request:**

```json
{
  "username": "admin",
  "password": "securepass"
}
```

**Validation:** username must not be empty; password must be ≥ 8 characters.

**Response `200`:**

```json
{
  "ok": true
}
```

**Error responses:** `400` (invalid input), `409` (setup already complete), `500` (server error).

### Auth

#### `POST /api/auth/login`

Authenticates a user with username/password. Rate-limited via middleware (3 failures → 10-minute IP lockout). Uses timing-safe comparison with dummy bcrypt hash to prevent username enumeration.

**Request:**

```json
{
  "username": "admin",
  "password": "securepass"
}
```

**Response `200`:**

```json
{
  "token": "<jwt>",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin"
  },
  "passwordNeedChange": false
}
```

**Error responses:** `400` (missing fields), `401` (invalid credentials), `429` (rate limited).

#### `POST /api/auth/logout`

Revokes the current JWT. Requires `Authorization: Bearer <token>`.

**Response `200`:**

```json
{
  "ok": true
}
```

#### `PUT /api/auth/password`

Changes the authenticated user's password. Revokes all tokens for the user (credential compromise mitigation). Requires `Authorization: Bearer <token>`.

**Request:**

```json
{
  "currentPassword": "oldpass",
  "newPassword": "newpass123"
}
```

**Response `200`:**

```json
{
  "ok": true
}
```

**Error responses:** `400` (invalid input / password < 8 chars), `401` (wrong current password).

#### `GET /api/auth/me`

Returns the authenticated user's info from the JWT claims. Requires `Authorization: Bearer <token>`.

**Response `200`:**

```json
{
  "id": 1,
  "username": "admin",
  "role": "admin"
}
```

---

### Call Search

#### `GET /api/calls`

Paginated call archive search with filtering. Uses `OptionalJWTAuth` middleware — works for unauthenticated users when `publicAccess=true`; when a valid JWT is present, per-user bookmark status is included in results.

**Query Parameters:**

| Param          | Type   | Default | Description                             |
| -------------- | ------ | ------- | --------------------------------------- |
| `system_id`    | int    | _(all)_ | Filter by system ID                     |
| `talkgroup_id` | int    | _(all)_ | Filter by talkgroup ID                  |
| `date_from`    | int64  | _(any)_ | Start of date range (Unix timestamp)    |
| `date_to`      | int64  | _(any)_ | End of date range (Unix timestamp)      |
| `page`         | int    | `1`     | Page number (1-indexed)                 |
| `limit`        | int    | `25`    | Results per page (max 100)              |
| `sort`         | string | `desc`  | Sort direction by date: `asc` or `desc` |

**Response `200`:**

```json
{
  "calls": [
    {
      "id": 12345,
      "dateTime": 1712345678,
      "frequency": 851012500,
      "duration": 8,
      "source": 1234,
      "systemId": 1,
      "talkgroupId": 101,
      "systemLabel": "County",
      "talkgroupLabel": "Fire Dispatch",
      "talkgroupName": "Fire Dispatch",
      "talkgroupTag": "Dispatch",
      "talkgroupGroup": "Fire",
      "talkgroupLed": "#ff0000",
      "transcript": "Engine 5 respond to...",
      "bookmarked": true
    }
  ],
  "total": 1423
}
```

**Notes:**

- `bookmarked` is only present when a valid JWT is provided; omitted for unauthenticated requests
- `limit` is clamped to a maximum of 100; values above 100 are silently reduced
- Results are paginated — use `total` with `page` and `limit` to calculate page count

#### `GET /api/calls/:id/audio`

Streams the audio file for a specific call. Uses `OptionalJWTAuth` middleware. Path traversal protection ensures audio paths cannot escape the recordings directory.

**Response `200`:** Audio file with appropriate `Content-Type` header and `Content-Disposition: inline`.

**Error responses:** `400` (invalid call ID), `404` (call not found or audio file missing), `500` (server error).

---

### Bookmarks

#### `GET /api/bookmarks`

Returns call IDs bookmarked by the authenticated user. Requires `Authorization: Bearer <token>`.

**Response `200`:**

```json
{
  "callIds": [12345, 12346, 12400]
}
```

Returns `{"callIds": []}` if the user has no bookmarks.

#### `POST /api/bookmarks`

Toggles a bookmark on a call for the authenticated user. If the bookmark exists, it is deleted; if it doesn't, it is created. Requires `Authorization: Bearer <token>`.

**Request:**

```json
{
  "callId": 12345
}
```

**Response `200` — bookmarked:**

```json
{
  "bookmarked": true,
  "id": 42
}
```

**Response `200` — unbookmarked:**

```json
{
  "bookmarked": false
}
```

**Error responses:** `400` (invalid body), `401` (no JWT), `500` (server error).

---

### Call Upload

Both endpoints are handled by the same `PostCallUpload` handler. Requires `X-API-Key` header authentication.

#### `POST /api/call-upload`

Rdio-scanner-style multipart call upload.

#### `POST /api/trunk-recorder-call-upload`

Alias for the above — accepts identical fields for Trunk Recorder compatibility.

**Auth:** `X-API-Key: <key>` header (or `?key=` query parameter as fallback).

**Rate limit:** 60 requests per minute per API key (configurable via `apiKeyCallRate` setting). Exceeding the limit returns `429`.

**Request: `multipart/form-data`**

| Field            | Type         | Required | Description                                     |
| ---------------- | ------------ | -------- | ----------------------------------------------- |
| `audio`          | file         | yes      | Audio file (WAV, MP3, AAC, M4A, OGG, Opus, …)   |
| `systemId`       | int string   | yes      | System decimal (radio system ID)                |
| `talkgroupId`    | int string   | yes      | Talkgroup decimal                               |
| `dateTime`       | int64 string | yes      | Unix timestamp (seconds)                        |
| `systemLabel`    | string       | no       | System name — used when `autoPopulate=true`     |
| `talkgroupGroup` | string       | no       | Talkgroup group — used when `autoPopulate=true` |
| `talkgroupLabel` | string       | no       | Talkgroup label — used when `autoPopulate=true` |
| `talkgroupTag`   | string       | no       | Talkgroup tag                                   |
| `frequency`      | int string   | no       | Primary frequency in Hz                         |
| `duration`       | int string   | no       | Duration in seconds                             |
| `source`         | int string   | no       | Primary source unit ID                          |
| `frequencies`    | string       | no       | JSON array of frequency objects                 |
| `sources`        | string       | no       | JSON array of source unit objects               |
| `patches`        | string       | no       | JSON array of patched talkgroup IDs             |

**Response `200` — call accepted:**

```json
{ "id": 12345 }
```

**Response `200` — duplicate detected:**

```json
{ "message": "duplicate" }
```

**Error responses:**

| Code  | Reason                                                                                |
| ----- | ------------------------------------------------------------------------------------- |
| `400` | Missing `audio`, `systemId`, `talkgroupId`, or `dateTime`; unparseable integer fields |
| `401` | Missing or invalid `X-API-Key`                                                        |
| `429` | Rate limit exceeded (60 req/min per API key by default)                               |
| `500` | Storage or DB error                                                                   |

**Auto-populate behaviour:** When `autoPopulate=true` (default), an unknown `systemId` or `talkgroupId` is automatically created in the database using the supplied label/group/tag fields. When `autoPopulate=false`, unknown identifiers return `400`.

**Audio storage:** Files are written to `{audioDir}/{YYYY}/{MM}/{DD}/{filename}`. The conversion mode is controlled by the `audioConversion` setting:

| Value | Behaviour                                     |
| ----- | --------------------------------------------- |
| `0`   | Disabled — keep original file                 |
| `1`   | Convert to AAC 32 kbps (default)              |
| `2`   | Convert to AAC 32 kbps + `acompressor` filter |
| `3`   | Convert to AAC 32 kbps + `loudnorm` filter    |

Conversion is performed asynchronously via a bounded FFmpeg worker pool (`runtime.NumCPU()` workers). `Store` blocks until conversion completes before inserting the DB record.

**Duplicate detection:** Controlled by the `disableDuplicateDetection` and `duplicateDetectionTimeFrame` settings. If the last call for the same system+talkgroup falls within the time window (default 500 ms), the upload is rejected with `{"message": "duplicate"}` (HTTP 200).

---

### WebSocket

Both WebSocket endpoints use the `coder/websocket` library with **permessage-deflate** compression (`CompressionContextTakeover`). All messages are JSON arrays: `[command, payload?, flags?]`. Audio data is sent as a separate binary frame immediately after the `CAL` text frame.

#### `GET /ws` — Listener WebSocket

Upgrades to a WebSocket connection for real-time call streaming.

**Auth (one of):**

1. **Public access** — When `publicAccess=true`, no authentication required. Client connects and immediately receives `VER` + `CFG` welcome messages.
2. **Listener JWT** — Client sends the JWT token string as the first message. Server validates token, checks revocation, verifies `listener` role, and loads user grants. Connection limit enforced per user.

**Welcome sequence (on successful auth):**

1. `["VER", {"version": "...", "branding": "...", "email": "..."}]`
2. `["CFG", <configPayload>]`

**Rejection messages:**

- `["XPR"]` — Invalid credentials or revoked token. Connection closed. Frontend clears credentials and redirects to login.
- `["MAX"]` — Server has reached `maxClients` limit, or per-user connection limit exceeded. Connection closed.

**Grant filtering:** Clients only receive `CAL` events for systems/talkgroups they are authorized to see. Public-access clients receive all events.

**Connection limits:**

- Global `maxClients` setting (checked before accepting connection)
- Per-user `limit` column on `users` table

#### `GET /api/admin/ws` — Admin WebSocket

Upgrades to a WebSocket connection for admin dashboard events.

**Auth:** JWT with `admin` role via `?token=<jwt>` query parameter (WebSocket cannot send custom headers). Returns `401` if missing/invalid, `403` if non-admin role.

**Behaviour:**

- No welcome messages (`VER`/`CFG` are not sent)
- Receives all broadcast events with no grant filtering
- Broadcast-only — no client-to-server commands processed

#### WebSocket Commands

| Command | Direction       | Payload                                             | Description                                                                                                                                                                                                      |
| ------- | --------------- | --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CAL`   | Server → Client | `{systemId, talkgroupId, ...}` + binary audio frame | New call data. Text frame with call metadata is followed immediately by a binary frame containing the audio file bytes. The two frames are sent atomically per client (mutex-protected to prevent interleaving). |
| `CFG`   | Server → Client | `{systems, talkgroups, groups, tags, ...}`          | Full config payload. Sent on connect (listener only) and when admin updates config.                                                                                                                              |
| `VER`   | Server → Client | `{"version", "branding", "email"}`                  | Server version and branding. Sent on connect (listener only).                                                                                                                                                    |
| `LSC`   | Server → Client | `<count>` (integer)                                 | Active listener count. Broadcast on connect/disconnect, debounced to max 1 per 3 seconds.                                                                                                                        |
| `XPR`   | Server → Client | _(none)_                                            | Session expired or auth failure. Connection closed after sending.                                                                                                                                                |
| `MAX`   | Server → Client | _(none)_                                            | Max clients reached or per-user limit exceeded. Connection closed after sending.                                                                                                                                 |
| `LFM`   | Bidirectional   | `{...}` (map)                                       | Live feed map update. Client sends to update; server echoes back.                                                                                                                                                |
| `LCL`   | Server → Client | `{"calls": [...], "total": <n>}`                    | Paginated call list results.                                                                                                                                                                                     |
| `TRN`   | Server → Client | `{"callId": <id>, "text": "..."}`                   | Transcript ready for a call.                                                                                                                                                                                     |

**Reserved commands** (not yet implemented): `IOS`, `PID`, `SRV`.

#### Architecture Notes

- The Hub runs in a single goroutine processing `register`, `unregister`, and `broadcast` channels.
- All sends are non-blocking — slow clients are dropped (message skipped) rather than blocking the hub.
- Each client has a `readPump` (reads client commands) and `writePump` (sends queued messages + periodic pings) goroutine.
- Ping/pong keepalive: pings sent every 30 seconds; pong wait timeout is 60 seconds.
- Graceful shutdown: `context.WithCancel` propagates through the hub; `closeAll()` closes all client send channels.

---

## Admin CRUD Endpoints

All admin endpoints require `Authorization: Bearer <jwt>` with `admin` role. Returns `401` if JWT is missing/invalid, `403` if role is not `admin`.

### Common Error Responses

| Code  | Meaning                                                    |
| ----- | ---------------------------------------------------------- |
| `400` | Invalid request body or path parameter                     |
| `401` | Missing or invalid JWT                                     |
| `403` | Non-admin role                                             |
| `404` | Resource not found                                         |
| `409` | Unique constraint violation (duplicate)                    |
| `422` | Validation failure (missing required field, invalid value) |
| `500` | Internal server error                                      |

### Common Patterns

All CRUD resources follow the same pattern:

- **GET** `/api/admin/{resource}` — List all. Returns `200` with JSON array.
- **POST** `/api/admin/{resource}` — Create. Returns `201` with created object.
- **PUT** `/api/admin/{resource}/:id` — Update by ID. Returns `200` with updated object.
- **DELETE** `/api/admin/{resource}/:id` — Delete by ID. Returns `200` with `{"ok": true}`.

---

### Config

#### `GET /api/admin/config`

Returns all settings as a JSON array of key/value objects.

**Response `200`:**

```json
[
  {"key": "audioConversion", "value": "1"},
  {"key": "publicAccess", "value": "false"},
  {"key": "maxClients", "value": "200"},
  ...
]
```

#### `PUT /api/admin/config`

Updates one or more settings. Only known setting keys are accepted (allowlist-validated). Broadcasts a `CFG` WebSocket message to all connected clients after update.

**Request:**

```json
{
  "publicAccess": "true",
  "maxClients": "500"
}
```

**Allowed setting keys:** `activityDashboard`, `afsSystems`, `apiKeyCallRate`, `audioConversion`, `autoPopulate`, `branding`, `darkMode`, `dimmerDelay`, `disableDuplicateDetection`, `duplicateDetectionTimeFrame`, `email`, `keyboardShortcuts`, `keypadBeeps`, `maxClients`, `playbackGoesLive`, `pruneDays`, `publicAccess`, `pushNotifications`, `searchPatchedTalkgroups`, `shareableLinks`, `showListenersCount`, `sortTalkgroups`, `tagsToggle`, `time12hFormat`, `transcriptionBinary`, `transcriptionEnabled`, `transcriptionLanguage`, `transcriptionModel`, `vapidPrivateKey`, `vapidPublicKey`.

**Response `200`:**

```json
{ "ok": true }
```

**Error `400`:** Unknown setting key included in request body.

---

### Users

#### `GET /api/admin/users`

Returns all users.

#### `POST /api/admin/users`

Creates a new user.

**Request:**

```json
{
  "username": "newuser",
  "password": "securepass123",
  "role": "listener",
  "disabled": 0,
  "systems_json": null,
  "expiration": null,
  "limit": null
}
```

**Validation:**

- `username` is required (422 if empty)
- `password` must be ≥ 8 characters (422 if too short)
- `role` must be `"admin"` or `"listener"` (defaults to `"listener"` if omitted; 422 if invalid)
- `username` must be unique (409 if duplicate)

**Response `201`:** Created user object (password_hash excluded from serialisation).

#### `PUT /api/admin/users/:id`

Updates a user (does not change password — use `PUT /api/auth/password` for that).

**Request:**

```json
{
  "username": "updatedname",
  "role": "admin",
  "disabled": 0,
  "systems_json": null,
  "expiration": null,
  "limit": null
}
```

**Validation:** Same as create (minus password).

#### `DELETE /api/admin/users/:id`

Deletes a user. **Constraint:** An admin cannot delete their own account (returns `400`).

---

### Systems

#### `GET /api/admin/systems`

Returns all systems.

#### `POST /api/admin/systems`

**Request:** `CreateSystemParams` fields: `system_id` (int, unique), `label`, `auto_populate`, `blacklists_json`, `led`, `order`.

**Error `409`:** `system_id` already exists.

#### `PUT /api/admin/systems/:id`

#### `DELETE /api/admin/systems/:id`

#### `PUT /api/admin/systems/reorder`

Reorders systems by updating the `order` column. Runs in a single transaction.

**Request:**

```json
{
  "systems": [
    {"id": 1, "order": 0},
    {"id": 2, "order": 1}
  ]
}
```

**Response `200`:**

```json
{ "ok": true }
```

**Error responses:** `400` (empty array or invalid body), `404` (system ID not found).

---

### Talkgroups

#### `GET /api/admin/talkgroups`

Returns all talkgroups across all systems.

#### `POST /api/admin/talkgroups`

**Request:** `CreateTalkgroupParams` fields: `system_id`, `talkgroup_id`, `label`, `name`, `frequency`, `led`, `group_id`, `tag_id`, `order`.

**Error `409`:** Talkgroup already exists (unique constraint on `system_id` + `talkgroup_id`).

#### `PUT /api/admin/talkgroups/:id`

#### `DELETE /api/admin/talkgroups/:id`

---

### Units

#### `GET /api/admin/units`

Returns all units across all systems.

#### `POST /api/admin/units`

**Request:** `CreateUnitParams` fields: `system_id`, `unit_id`, `label`, `order`.

**Error `409`:** Unit already exists.

#### `PUT /api/admin/units/:id`

#### `DELETE /api/admin/units/:id`

---

### Groups

#### `GET /api/admin/groups`

#### `POST /api/admin/groups`

**Request:**

```json
{ "label": "Fire" }
```

**Validation:** `label` is required (422 if empty). Must be unique (409 if duplicate).

#### `PUT /api/admin/groups/:id`

#### `DELETE /api/admin/groups/:id`

---

### Tags

#### `GET /api/admin/tags`

#### `POST /api/admin/tags`

**Request:**

```json
{ "label": "Dispatch" }
```

**Validation:** `label` is required (422 if empty). Must be unique (409 if duplicate).

#### `PUT /api/admin/tags/:id`

#### `DELETE /api/admin/tags/:id`

---

### API Keys

#### `GET /api/admin/apikeys`

#### `POST /api/admin/apikeys`

**Request:** `CreateAPIKeyParams` fields: `key` (auto-generated UUID v4 if empty), `ident`, `disabled`, `systems_json`, `order`.

**Error `409`:** Key already exists.

#### `PUT /api/admin/apikeys/:id`

#### `DELETE /api/admin/apikeys/:id`

#### `PUT /api/admin/apikeys/reorder`

Reorders API keys by updating the `order` column. Runs in a single transaction.

**Request:**

```json
{
  "apiKeys": [
    {"id": 1, "order": 0},
    {"id": 2, "order": 1}
  ]
}
```

**Response `200`:**

```json
{ "ok": true }
```

**Error responses:** `400` (empty array or invalid body), `404` (API key not found).

#### `POST /api/admin/apikeys/migrate-hash`

Migrates legacy plaintext API keys to SHA-256 hashes. Keys already in hex-encoded SHA-256 format are skipped.

**Response `200`:**

```json
{ "migrated": 3 }
```

---

---

### Server Directory Browser

#### `GET /api/admin/fs/directories`

Lists directories on the server filesystem. Used by the Dirwatches panel to browse for recording directories.

**Query Parameters:**

| Param  | Type   | Default | Description                     |
| ------ | ------ | ------- | ------------------------------- |
| `path` | string | `/`     | Absolute directory path to list |

**Response `200`:**

```json
{
  "path": "/mnt/recordings",
  "parent": "/mnt",
  "directories": [
    {"name": "system1", "path": "/mnt/recordings/system1"},
    {"name": "system2", "path": "/mnt/recordings/system2"}
  ]
}
```

**Notes:**

- `parent` is `null` when listing the root directory `/`
- Hidden directories (dotfiles) are excluded
- At root level, system virtual directories (`/proc`, `/sys`, `/dev`, etc.) are excluded

**Error `422`:** Path is not absolute, does not exist, or is not a directory.

---

### Dirwatches

> **Service reload:** Creating, updating, or deleting a dirwatch entry triggers an immediate `Service.Reload` — all watcher goroutines are stopped and restarted from the DB. No server restart is required for config changes to take effect.

#### `GET /api/admin/dirwatches`

#### `POST /api/admin/dirwatches`

**Request:** `CreateDirwatchParams` fields: `directory` (required, 422 if empty), `type`, `mask`, `extension`, `frequency`, `delay`, `delete_after`, `use_polling`, `disabled`, `system_id`, `talkgroup_id`, `order`.

#### `PUT /api/admin/dirwatches/:id`

#### `DELETE /api/admin/dirwatches/:id`

---

### Downstreams

> **Service reload:** Creating, updating, or deleting a downstream entry triggers an immediate `Service.Reload` — all pusher goroutines are stopped and restarted from the DB. No separate restart endpoint is needed.

#### `GET /api/admin/downstreams`

#### `POST /api/admin/downstreams`

**Request:** `CreateDownstreamParams` fields: `url` (required, 422 if empty), `api_key`, `systems_json`, `disabled`, `order`.

#### `PUT /api/admin/downstreams/:id`

#### `DELETE /api/admin/downstreams/:id`

---

### Webhooks

#### `GET /api/admin/webhooks`

#### `POST /api/admin/webhooks`

**Request:** `CreateWebhookParams` fields: `url` (required, 422 if empty), `type`, `secret`, `systems_json`, `disabled`, `order`.

#### `PUT /api/admin/webhooks/:id`

#### `DELETE /api/admin/webhooks/:id`

---

### Logs

#### `GET /api/admin/logs`

Returns server log entries, optionally filtered by time range and level.

**Query Parameters:**

| Param   | Type   | Default | Description                                 |
| ------- | ------ | ------- | ------------------------------------------- |
| `from`  | int64  | `0`     | Start of date range (Unix timestamp)        |
| `to`    | int64  | _now_   | End of date range (Unix timestamp)          |
| `level` | string | _(all)_ | Filter by level: `info`, `warn`, or `error` |

**Response `200`:**

```json
[
  {"id": 1, "date_time": 1712345678, "level": "info", "message": "server started"},
  ...
]
```

**Truncation:** Results are capped at 10,000 rows. When truncated, an `X-Truncated: true` response header is set.

---

### Missing Audio Tools

#### `GET /api/admin/tools/audio-missing`

Scans a page of archived calls and returns entries whose audio file does not exist on disk.

**Query Parameters:**

| Param    | Type  | Default | Description                    |
| -------- | ----- | ------- | ------------------------------ |
| `limit`  | int   | `200`   | Calls to check per page (max 1000) |
| `offset` | int   | `0`     | Offset into calls list         |

**Response `200`:**

```json
{
  "recordingsDir": "/data/recordings",
  "limit": 200,
  "offset": 0,
  "totalCalls": 5000,
  "checked": 200,
  "missing": [
    {
      "id": 123,
      "dateTime": 1712345678,
      "audioPath": "2025/01/01/call.m4a",
      "audioName": "call.m4a",
      "reason": "file not found"
    }
  ]
}
```

#### `POST /api/admin/tools/audio-missing/cleanup`

Deletes call database rows for calls whose audio is confirmed missing. Re-checks each call at delete time — if the file has reappeared, the call is skipped.

**Request:**

```json
{
  "confirm": true,
  "callIds": [123, 456, 789]
}
```

**Validation:** `confirm` must be `true`, `callIds` is required and limited to 1000 entries.

**Response `200`:**

```json
{
  "requested": 3,
  "deleted": 2,
  "skipped": [
    {
      "id": 789,
      "dateTime": 1712345999,
      "audioPath": "2025/01/01/other.m4a",
      "audioName": "other.m4a",
      "reason": "file now exists"
    }
  ]
}
```

**Error `400`:** Missing `confirm`, empty `callIds`, or too many IDs (>1000).

---

### Import / Export

#### `POST /api/admin/import/talkgroups`

Imports talkgroups from a CSV file for a specific system.

**Request: `multipart/form-data`**

| Field       | Type       | Required | Description                   |
| ----------- | ---------- | -------- | ----------------------------- |
| `file`      | file       | yes      | CSV file                      |
| `system_id` | int string | yes      | Target system ID (must exist) |

**CSV format:** Columns: `talkgroup_id`, `label`, `name`, `tag_id`, `group_id`, `frequency`, `led`, `order`. Header row is auto-detected and skipped. Rows with non-numeric first column are skipped.

**Response `200`:**

```json
{ "imported": 42 }
```

**Safety limit:** Max 100,000 rows per import.

**Error responses:** `400` (missing system_id/file, invalid CSV, system not found), `500` (database error).

#### `POST /api/admin/import/units`

Imports units from a CSV file for a specific system.

**Request: `multipart/form-data`**

| Field       | Type       | Required | Description                   |
| ----------- | ---------- | -------- | ----------------------------- |
| `file`      | file       | yes      | CSV file                      |
| `system_id` | int string | yes      | Target system ID (must exist) |

**CSV format:** Columns: `unit_id`, `label`, `order`. Header row is auto-detected and skipped.

**Response `200`:**

```json
{ "imported": 15 }
```

#### `GET /api/admin/export/config`

Exports the full server configuration as a JSON download.

**Response `200`:** `Content-Disposition: attachment; filename="openscanner-config.json"`

```json
{
  "settings": [...],
  "users": [...],
  "systems": [...],
  "talkgroups": [...],
  "units": [...],
  "groups": [...],
  "tags": [...],
  "apiKeys": [...],
  "dirwatches": [...],
  "downstreams": [...],
  "webhooks": [...]
}
```

#### `POST /api/admin/import/config`

Imports a full configuration JSON blob. Runs in a single database transaction — all-or-nothing. Duplicates are skipped (upserts where applicable).

**Request:** JSON body matching the export format (all top-level arrays are optional).

**Setting keys are validated** against the allowlist — unknown keys are skipped with a log warning.

**Response `200`:**

```json
{ "ok": true }
```

**Error `400`:** Invalid JSON body.
