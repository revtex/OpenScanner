# OpenScanner — API Reference

> **Implementation status:** Phases 1–4 endpoints are implemented. All other endpoints listed below are planned for future phases and are marked accordingly.

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
  "token": "<jwt>",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "admin"
  }
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
  "message": "logged out"
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
  "message": "password updated"
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

### Call Upload

Both endpoints are handled by the same `PostCallUpload` handler. Requires `X-API-Key` header authentication.

#### `POST /api/call-upload`

Rdio-scanner-style multipart call upload.

#### `POST /api/trunk-recorder-call-upload`

Alias for the above — accepts identical fields for Trunk Recorder compatibility.

**Auth:** `X-API-Key: <key>` header (or `key` form field as fallback).

**Rate limit:** 60 requests per minute per API key (configurable via `apiKeyCallRate` setting). Exceeding the limit returns `429`.

**Request: `multipart/form-data`**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `audio` | file | yes | Audio file (WAV, MP3, AAC, M4A, OGG, Opus, …) |
| `systemId` | int string | yes | System decimal (radio system ID) |
| `talkgroupId` | int string | yes | Talkgroup decimal |
| `dateTime` | int64 string | yes | Unix timestamp (seconds) |
| `systemLabel` | string | no | System name — used when `autoPopulate=true` |
| `talkgroupGroup` | string | no | Talkgroup group — used when `autoPopulate=true` |
| `talkgroupLabel` | string | no | Talkgroup label — used when `autoPopulate=true` |
| `talkgroupTag` | string | no | Talkgroup tag |
| `frequency` | int string | no | Primary frequency in Hz |
| `duration` | int string | no | Duration in seconds |
| `source` | int string | no | Primary source unit ID |
| `frequencies` | string | no | JSON array of frequency objects |
| `sources` | string | no | JSON array of source unit objects |
| `patches` | string | no | JSON array of patched talkgroup IDs |

**Response `200` — call accepted:**

```json
{"id": 12345}
```

**Response `200` — duplicate detected:**

```json
{"message": "duplicate"}
```

**Error responses:**

| Code | Reason |
|------|--------|
| `400` | Missing `audio`, `systemId`, `talkgroupId`, or `dateTime`; unparseable integer fields |
| `401` | Missing or invalid `X-API-Key` |
| `429` | Rate limit exceeded (60 req/min per API key by default) |
| `500` | Storage or DB error |

**Auto-populate behaviour:** When `autoPopulate=true` (default), an unknown `systemId` or `talkgroupId` is automatically created in the database using the supplied label/group/tag fields. When `autoPopulate=false`, unknown identifiers return `400`.

**Audio storage:** Files are written to `{audioDir}/{YYYY}/{MM}/{DD}/{filename}`. The conversion mode is controlled by the `audioConversion` setting:

| Value | Behaviour |
|-------|-----------|
| `0` | Disabled — keep original file |
| `1` | Convert to AAC 32 kbps (default) |
| `2` | Convert to AAC 32 kbps + `acompressor` filter |
| `3` | Convert to AAC 32 kbps + `loudnorm` filter |

Conversion is performed asynchronously via a bounded FFmpeg worker pool (`runtime.NumCPU()` workers). `Store` blocks until conversion completes before inserting the DB record.

**Duplicate detection:** Controlled by the `disableDuplicateDetection` and `duplicateDetectionTimeFrame` settings. If the last call for the same system+talkgroup falls within the time window (default 500 ms), the upload is rejected with `{"message": "duplicate"}` (HTTP 200).

---

## Planned Endpoints (not yet implemented)

### Admin Config

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

- `GET /ws` — Listener socket
- `GET /api/admin/ws` — Admin dashboard socket
