# OpenScanner — API Reference

> **Implementation status:** Only Phase 3 endpoints are implemented. All other endpoints listed below are planned for future phases and are marked accordingly.

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

## Planned Endpoints (not yet implemented)

### Call Ingest

- `POST /api/call-upload` — Generic call upload (API key auth)
- `POST /api/trunk-recorder-call-upload` — Trunk Recorder format (API key auth)

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
