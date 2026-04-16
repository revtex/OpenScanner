# Backend Logging Improvement Plan

## Overview

The audit found ~40+ unlogged error paths, 25+ CRUD operations with no audit trail, and many logs missing critical context attributes (call ID, user ID, request IP). This plan adds Info/Warn/Error/Debug logs across the backend so that a submitted log file alone can diagnose any issue.

---

## Phase 1 — Admin CRUD Audit Trail

> **Status: ✅ Complete** — committed `9e1ff66`

These are the highest-priority gaps. Every admin action that mutates data should emit an `Info` log with who did what and which resource was affected. Currently zero CRUD success paths are logged.

### `backend/internal/api/crud.go`

Add `slog.Info` at the success path of every Create/Update/Delete handler. Extract the acting admin's user ID from `c.Get("userID")` and include it as `"by"`. Include the resource ID as `"id"` and any meaningful label.

| Handler            | Message                     | Attributes                                        |
| ------------------ | --------------------------- | ------------------------------------------------- |
| `CreateUser`       | `admin: user created`       | `id`, `username`, `role`, `by`                    |
| `UpdateUser`       | `admin: user updated`       | `id`, `username`, `role`, `disabled`, `by`        |
| `DeleteUser`       | `admin: user deleted`       | `id`, `by`                                        |
| `CreateSystem`     | `admin: system created`     | `id`, `system_id`, `label`, `by`                  |
| `UpdateSystem`     | `admin: system updated`     | `id`, `system_id`, `by`                           |
| `DeleteSystem`     | `admin: system deleted`     | `id`, `by`                                        |
| `CreateTalkgroup`  | `admin: talkgroup created`  | `id`, `talkgroup_id`, `by`                        |
| `UpdateTalkgroup`  | `admin: talkgroup updated`  | `id`, `talkgroup_id`, `by`                        |
| `DeleteTalkgroup`  | `admin: talkgroup deleted`  | `id`, `by`                                        |
| `CreateAPIKey`     | `admin: api key created`    | `id`, `ident`, `by` — **never log the key value** |
| `UpdateAPIKey`     | `admin: api key updated`    | `id`, `ident`, `by`                               |
| `DeleteAPIKey`     | `admin: api key deleted`    | `id`, `by`                                        |
| `CreateDirMonitor` | `admin: dirmonitor created` | `id`, `dir`, `by`                                 |
| `UpdateDirMonitor` | `admin: dirmonitor updated` | `id`, `dir`, `by`                                 |
| `DeleteDirMonitor` | `admin: dirmonitor deleted` | `id`, `by`                                        |
| `CreateDownstream` | `admin: downstream created` | `id`, `url`, `by`                                 |
| `UpdateDownstream` | `admin: downstream updated` | `id`, `url`, `by`                                 |
| `DeleteDownstream` | `admin: downstream deleted` | `id`, `by`                                        |
| `CreateWebhook`    | `admin: webhook created`    | `id`, `url`, `by`                                 |
| `UpdateWebhook`    | `admin: webhook updated`    | `id`, `url`, `by`                                 |
| `DeleteWebhook`    | `admin: webhook deleted`    | `id`, `by`                                        |
| `CreateGroup`      | `admin: group created`      | `id`, `label`, `by`                               |
| `UpdateGroup`      | `admin: group updated`      | `id`, `label`, `by`                               |
| `DeleteGroup`      | `admin: group deleted`      | `id`, `by`                                        |
| `CreateTag`        | `admin: tag created`        | `id`, `label`, `by`                               |
| `UpdateTag`        | `admin: tag updated`        | `id`, `label`, `by`                               |
| `DeleteTag`        | `admin: tag deleted`        | `id`, `by`                                        |
| `CreateUnit`       | `admin: unit created`       | `id`, `by`                                        |
| `UpdateUnit`       | `admin: unit updated`       | `id`, `by`                                        |
| `DeleteUnit`       | `admin: unit deleted`       | `id`, `by`                                        |
| `ReorderSystems`   | `admin: systems reordered`  | `count`, `by` — one log per batch                 |
| `ReorderAPIKeys`   | `admin: api keys reordered` | `count`, `by` — one log per batch                 |

### `backend/internal/api/config.go` — `PutConfig`

- Add `slog.Info("admin: config saved", "keys", []string{...}, "by", actorID)` after successful commit
- Add `slog.Info("admin: log level changed", "level", value, "by", actorID)` when `logLevel` key is present
- Do **not** log raw values for sensitive keys (`vapidPrivateKey`, `vapidPublicKey`) — log key names only

### `backend/internal/api/admin.go` — `PutPassword`

- Add `slog.Info("auth: password changed", "user_id", userID, "ip", c.ClientIP())` on success
- Add `slog.Warn("auth: password change rejected - wrong current password", "user_id", userID, "ip", c.ClientIP())` on 401 branch
- Add `slog.Error(...)` on the DB failure branches that currently return 500 silently

### `backend/internal/api/setup.go` — `PostSetup` and `GetSetupStatus`

- Import `"log/slog"` (currently missing)
- Add `slog.Error(...)` on all error branches that return 500 (currently completely silent)
- Add `slog.Info("setup: initial admin account created", "username", req.Username)` on success

---

## Phase 2 — Error Paths Missing Logs

> **Status: ✅ Complete**

### `backend/internal/api/calls.go` — `PostCallUpload`

- Add `slog.Warn("call-upload: incomplete data", "reason", incompleteReason, "api_key_id", apiKeyID)` when returning 417 — currently silent, but important for diagnosing recorder connection tests
- Add `slog.Info("call-upload: complete", "call_id", callID, "system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw, "duration_ms", duration, "audio_path", relPath, "api_key_id", apiKeyID)` at the end of the success path — **the single most important missing log in the entire codebase**
- Add `"system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw` attrs to the existing `slog.Error("failed to store audio file", ...)` call

### `backend/internal/audio/processor.go` — `Store` and `StoreFile`

- Add `slog.Debug("audio: file written", "path", relPath, "size_bytes", fh.Size)` after `dst.Close()`
- Add `slog.Warn("audio: failed to remove original after conversion", "path", destPath, "error", err)` on the silent `_ = err` branch
- Add `slog.Debug("audio: conversion complete", "input", safeName, "output", relOut)` at success return

### `backend/internal/api/admin.go` — `PostLogin`

- Add `slog.Error("auth: failed to generate token", "user_id", user.ID, "error", err)` on the token generation failure branch

---

## Phase 3 — Improve Existing Logs (Missing Context)

> **Status: ✅ Complete**

### `backend/internal/middleware/middleware.go` — `APIKeyAuth`

- Add `slog.Debug("middleware: api key auth success", "api_key_id", key.ID, "ident", key.Ident.String, "path", c.Request.URL.Path)` — currently only failure paths are logged

### `backend/internal/ws/client.go` — `readPump`

- Add `slog.Warn("ws: received unknown command", "cmd", msg.Cmd)` for unhandled command types — currently unmatched commands are silently ignored

### `backend/internal/ws/hub.go` — `BroadcastCFG`

- Add `slog.Debug("ws: cfg broadcast complete", "clients", count)` after the rebuild and broadcast — currently no confirmation that it succeeded

### `backend/internal/api/calls.go` — Existing error log

- Improve `slog.Error("failed to store audio file", "error", err)` to include `"system_id", systemIDRaw, "talkgroup_id", talkgroupIDRaw` so the caller context is preserved

### `backend/internal/downstream/pusher.go` — `pushCall`

- Add HTTP response status code to the success path at Debug level — helps distinguish 200 vs 201 vs unexpected accepted codes

---

## Phase 4 — Startup & Lifecycle Logging

### `backend/cmd/server/main.go`

- Add `slog.Info("server: startup complete", "addr", addr, "recordings_dir", ..., "db", ...)` after all services have started and the HTTP listener is ready
- Add `slog.Info("server: shutdown complete")` at graceful shutdown completion
- Add `slog.Debug("server: loaded settings from db", "logLevel", ..., "publicAccess", ..., "autoPopulate", ...)` during startup settings read

### `backend/internal/db/open.go`

- Check whether migration logging exists; if not, add `slog.Info("db: migration applied", "version", n, "name", name)` per migration step applied

---

## Key Rules

| Rule                 | Detail                                                                                              |
| -------------------- | --------------------------------------------------------------------------------------------------- |
| Actor ID             | Extract via `c.Get("userID")` — available on all JWT-protected routes, no middleware changes needed |
| CRUD success level   | `slog.Info` — these are audit-trail events, not verbose diagnostics                                 |
| Sensitive values     | Never log raw values for `vapidPrivateKey`, `vapidPublicKey` — log key name only                    |
| Token values         | Never log JWT token strings or API key values at any level                                          |
| Error log attributes | Always include `"error", err` plus the resource/context IDs, never just the error alone             |

---

## Verification Steps

1. `make build` in `backend/` — must compile with exit 0
2. Start server, set log level to **DEBUG**, create a system via admin API, upload a call
3. Confirm log output shows: actor user ID, resource ID, call pipeline stages end-to-end (request → system resolved → tg resolved → audio stored → db inserted → ws broadcast → complete)
4. Set log level to **INFO** — confirm no debug flood, only meaningful audit events (logins, CRUD mutations, call ingested)
5. Simulate a failure (wrong API key, bad audio) — confirm the log alone is sufficient to diagnose the problem without needing to reproduce it

---

## Files Affected

| File                                        | Change type                                                    |
| ------------------------------------------- | -------------------------------------------------------------- |
| `backend/internal/api/crud.go`              | 30+ new `slog.Info` success logs                               |
| `backend/internal/api/admin.go`             | `PutPassword` logs, `PostLogin` error log                      |
| `backend/internal/api/config.go`            | `PutConfig` completion log with key names                      |
| `backend/internal/api/setup.go`             | Add `slog` import, error + success logs                        |
| `backend/internal/api/calls.go`             | Completion Info log, incomplete Warn, improved error attrs     |
| `backend/internal/audio/processor.go`       | Post-write Debug, silent-error Warn, conversion-complete Debug |
| `backend/internal/middleware/middleware.go` | APIKeyAuth success Debug                                       |
| `backend/internal/ws/client.go`             | Unknown command Warn                                           |
| `backend/internal/ws/hub.go`                | CFG broadcast complete Debug                                   |
| `backend/internal/downstream/pusher.go`     | HTTP status on success Debug                                   |
| `backend/cmd/server/main.go`                | Startup complete, shutdown complete, settings loaded           |
| `backend/internal/db/open.go`               | Per-migration Info log (if not already present)                |
