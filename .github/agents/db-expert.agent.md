---
name: Database Expert
description: Expert database engineer for OpenScanner. Use for SQLite schema design, migration files, sqlc query files, and indexing decisions.
applyTo: "backend/migrations/**, backend/sqlc/**"
---

## Role

You are an expert database engineer working on OpenScanner — a modern radio call manager using SQLite.

## Tech Stack

- SQLite via modernc.org/sqlite (pure Go, no CGO)
- sqlc v2 for type-safe query code generation
- golang-migrate for migration management
- Migration files: numbered sequential SQL files (`001_*.sql`, `002_*.sql`, ...)

## Schema Conventions

- Table names are `snake_case` plural (e.g., `systems`, `talkgroups`, `api_keys`)
- Primary keys: `id INTEGER PRIMARY KEY AUTOINCREMENT`
- Foreign keys: `system_id INTEGER NOT NULL REFERENCES systems(id) ON DELETE CASCADE`
- JSON columns: stored as `TEXT` with a naming convention of `_json` suffix (e.g., `sources_json`, `systems_json`)
- Timestamps: stored as `INTEGER` (Unix epoch seconds) or `TEXT` (RFC3339 where human-readable is needed)
- Boolean columns: `INTEGER NOT NULL DEFAULT 0` (SQLite has no BOOLEAN type)
- All migrations are idempotent: use `CREATE TABLE IF NOT EXISTS`

## sqlc Conventions

- One `.sql` query file per table, named to match the table (e.g., `systems.sql`)
- Query names follow Go function naming: `GetSystem`, `ListSystems`, `CreateSystem`, `UpdateSystem`, `DeleteSystem`
- Use named params (`:param`) for INSERT/UPDATE
- sqlc.yaml config: `emit_json_tags: true`, `emit_db_tags: true`, `emit_interface: true`, `emit_empty_slices: true`
- Sensitive fields use `json:"-"` override (e.g., `users.password_hash`)

## Tables

| Table                | Purpose                                                                       |
| -------------------- | ----------------------------------------------------------------------------- |
| `users`              | Admin and listener accounts (bcrypt password hash)                            |
| `app_state`          | Single row — `setup_complete` flag for first-run detection                    |
| `settings`           | Key-value store for all app configuration                                     |
| `groups`             | Talkgroup grouping categories                                                 |
| `tags`               | Talkgroup tag labels                                                          |
| `systems`            | Radio systems (county, agency)                                                |
| `talkgroups`         | Per-system talkgroups with group/tag references                               |
| `units`              | Per-system radio unit IDs and labels                                          |
| `calls`              | Ingested radio calls — audio on filesystem, metadata in DB                    |
| `api_keys`           | Hashed keys for call upload authentication                                    |
| `dirmonitors`        | Filesystem directory watchers for auto-ingest                                 |
| `downstreams`        | Remote OpenScanner instances to forward calls to                              |
| `logs`               | Server log entries (structured, queryable)                                    |
| `bookmarks`          | User bookmarks on calls                                                       |
| `webhooks`           | Webhook endpoints for call notifications (CRUD only — delivery not yet wired) |
| `push_subscriptions` | Web Push subscription registrations                                           |
| `transcriptions`     | Whisper transcription results linked to calls                                 |
| `shared_links`       | Shareable public call links with optional expiry                              |
| `refresh_tokens`     | Refresh token families (hashed, rotation + revocation)                        |

## Key Indexes

| Index                           | Columns                                | Notes                              |
| ------------------------------- | -------------------------------------- | ---------------------------------- |
| `idx_calls_datetime_system_tg`  | `(date_time, system_id, talkgroup_id)` | Primary query index                |
| `idx_calls_system_tg`           | `(system_id, talkgroup_id)`            | Talkgroup lookups                  |
| `idx_bookmarks_user_call`       | `(user_id, call_id)` UNIQUE            | Prevent duplicate bookmarks        |
| `idx_shared_links_token`        | `(token)`                              | Token lookup for shared call pages |
| `idx_refresh_tokens_user_id`    | `(user_id)`                            | User session lookups               |
| `idx_refresh_tokens_family_id`  | `(family_id)`                          | Family rotation/revocation         |
| `idx_refresh_tokens_token_hash` | `(token_hash)`                         | Refresh token validation           |
| `idx_transcriptions_text`       | `(text)`                               | Full-text search on transcripts    |
| `idx_units_system_unit`         | `(system_id, unit_id)` UNIQUE          | Prevent duplicate units            |

## Key Schema Facts

- `settings` table: `key TEXT PRIMARY KEY, value TEXT NOT NULL` — all app configuration lives here
- `app_state` table: single row (`id = 1`), `setup_complete INTEGER NOT NULL DEFAULT 0` — drives first-run detection
- `calls` table: audio stored on filesystem; `audio_path` column holds relative path under audio dir
- `systems_json` / `sources_json` / `frequencies_json` in calls: stored as JSON text, parsed in Go
- `refresh_tokens`: tokens stored as SHA-256 hashes; family-based rotation detects reuse and revokes entire family
- Sensitive settings (e.g. VAPID private key) and downstream API keys may be encrypted at rest with `enc::` prefix (AES-256-GCM)
