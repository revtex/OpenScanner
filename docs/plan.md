# OpenScanner — Implementation Plan

## Overview

OpenScanner is a modern reimplementation of [rdio-scanner](https://github.com/chuot/rdio-scanner): a web-based software-defined radio call manager with real-time audio streaming.

### Stack Decisions

| Layer              | Technology                                                       |
| ------------------ | ---------------------------------------------------------------- |
| Backend language   | Go 1.25                                                          |
| HTTP framework     | Gin                                                              |
| WebSocket          | coder/websocket (github.com/coder/websocket)                     |
| Database           | SQLite via modernc.org/sqlite (pure Go, no CGO)                  |
| DB query layer     | sqlc (type-safe code generation)                                 |
| DB migrations      | golang-migrate                                                   |
| Auth               | golang-jwt/jwt v5 + golang.org/x/crypto/bcrypt                   |
| TLS                | Built-in TLS + golang.org/x/crypto/acme/autocert (Let's Encrypt) |
| Service daemon     | kardianos/service (systemd / Windows service / launchd)          |
| Audio storage      | Filesystem (paths stored in DB)                                  |
| Audio conversion   | FFmpeg (external subprocess) with bounded worker pool            |
| Transcription      | Local Whisper binary (CPU or GPU via NVIDIA CUDA)                |
| Push notifications | webpush-go (Web Push / VAPID)                                    |
| Logging            | log/slog (structured, levelled)                                  |
| Deployment         | go:embed frontend into single Go binary                          |
| Frontend framework | React 18 + TypeScript (strict) + Vite                            |
| UI components      | DaisyUI + Tailwind CSS (dark/light themes)                       |
| State management   | Redux Toolkit + RTK Query                                        |
| Virtual scrolling  | @tanstack/react-virtual (long admin lists)                       |
| PWA                | Service Worker (app-shell cache + push notifications) + manifest |
| Frontend tests     | Vitest + React Testing Library                                   |
| E2E tests          | Playwright                                                       |
| Dev tooling        | air (Go hot-reload) + Vite proxy (single `make dev`)             |

---

## Repository Layout

```
openscanner/                     ← monorepo root
├── backend/
│   ├── cmd/server/main.go       ← wire Gin, DB, WS hub, services; CLI subcommands; graceful shutdown
│   ├── cmd/migrate/main.go      ← standalone migration runner
│   ├── internal/
│   │   ├── api/                 ← Gin route handlers
│   │   │   ├── routes.go        ← all route registrations
│   │   │   ├── admin.go         ← admin auth + CRUD handlers
│   │   │   ├── calls.go         ← call upload handlers
│   │   │   ├── health.go        ← GET /api/health readiness probe
│   │   │   ├── setup.go         ← first-run setup endpoints
│   │   │   ├── share.go         ← public call share endpoint (/call/:id)
│   │   │   └── webhooks.go      ← webhook delivery + CRUD handlers
│   │   ├── ws/                  ← WebSocket hub
│   │   │   ├── hub.go           ← hub: register/unregister/broadcast
│   │   │   ├── client.go        ← listener + admin WS client
│   │   │   └── messages.go      ← WS command type definitions
│   │   ├── db/                  ← sqlc-generated (do not edit manually)
│   │   ├── audio/               ← FFmpeg pipeline
│   │   │   ├── processor.go     ← save file + FFmpeg conversion
│   │   │   ├── duplicate.go     ← duplicate call detection
│   │   │   ├── worker.go        ← bounded FFmpeg worker pool (channel queue)
│   │   │   └── transcriber.go   ← Whisper transcription worker pool
│   │   ├── dirwatch/            ← fsnotify-based directory watcher
│   │   │   ├── watcher.go       ← fsnotify watcher + polling fallback
│   │   │   ├── parsers.go       ← per-recorder-type file parsers
│   │   │   └── mask.go          ← meta-mask token expansion
│   │   ├── downstream/          ← call pusher to remote instances
│   │   │   └── pusher.go        ← one goroutine per downstream config
│   │   ├── auth/                ← JWT + bcrypt + RBAC
│   │   │   ├── auth.go          ← JWT sign/verify (with role claims), bcrypt hash/verify
│   │   │   └── ratelimit.go     ← login rate limiter (3 fails → 10-min lockout)
│   │   ├── seed/                ← first-run DB seeder
│   │   │   └── seed.go          ← inserts default settings, groups, tags, app_state
│   │   ├── config/              ← server startup configuration
│   │   │   └── config.go        ← CLI flags, env vars, INI file parsing
│   │   ├── notify/              ← push notification sender
│   │   │   └── push.go          ← Web Push delivery via webpush-go
│   │   └── middleware/          ← Gin middleware
│   │       └── middleware.go    ← JWTAuth, APIKeyAuth, RateLimit, RequestID, logging
│   ├── migrations/              ← numbered .sql migration files
│   │   ├── 001_create_users.sql
│   │   ├── 002_create_app_state.sql
│   │   ├── 003_create_settings.sql
│   │   ├── 004_create_groups.sql
│   │   ├── 005_create_tags.sql
│   │   ├── 006_create_systems.sql
│   │   ├── 007_create_talkgroups.sql
│   │   ├── 008_create_units.sql
│   │   ├── 009_create_calls.sql
│   │   ├── 010_create_api_keys.sql
│   │   ├── 011_create_accesses.sql
│   │   ├── 012_create_dirwatches.sql
│   │   ├── 013_create_downstreams.sql
│   │   ├── 014_create_logs.sql
│   │   ├── 015_create_bookmarks.sql
│   │   ├── 016_create_webhooks.sql
│   │   ├── 017_create_push_subscriptions.sql
│   │   ├── 018_create_transcriptions.sql
│   │   └── 019_drop_accesses.sql
│   └── sqlc/
│       ├── sqlc.yaml
│       └── queries/             ← one .sql file per table
│           ├── users.sql
│           ├── app_state.sql
│           ├── settings.sql
│           ├── calls.sql
│           ├── systems.sql
│           ├── talkgroups.sql
│           ├── units.sql
│           ├── groups.sql
│           ├── tags.sql
│           ├── api_keys.sql
│           ├── dirwatches.sql
│           ├── downstreams.sql
│           ├── logs.sql
│           ├── bookmarks.sql
│           ├── webhooks.sql
│           ├── push_subscriptions.sql
│           └── transcriptions.sql
├── frontend/
│   ├── src/
│   │   ├── main.tsx             ← React entry point
│   │   ├── app/
│   │   │   ├── store.ts         ← Redux store configuration
│   │   │   ├── api.ts           ← RTK Query base API
│   │   │   └── slices/
│   │   │       ├── scannerSlice.ts  ← live feed, hold, avoid, queue, TG selection
│   │   │       ├── authSlice.ts     ← JWT token, user profile (id, username, role), setup state
│   │   │       ├── adminSlice.ts    ← admin CRUD data + config
│   │   │       └── callsSlice.ts    ← archived calls search state
│   │   ├── pages/
│   │   │   ├── Scanner.tsx      ← main scanner UI page
│   │   │   ├── Admin.tsx        ← admin dashboard page
│   │   │   ├── Login.tsx        ← login page (username + password)
│   │   │   ├── Setup.tsx        ← first-run wizard page
│   │   │   └── SharedCall.tsx   ← public shareable call player page (/call/:id)
│   │   ├── components/
│   │   │   ├── ui/              ← shared UI components
│   │   │   ├── scanner/
│   │   │   │   ├── LEDPanel.tsx         ← green/orange/blink LED states
│   │   │   │   ├── DisplayPanel.tsx     ← 6-line info display
│   │   │   │   ├── ControlToolbar.tsx   ← Two-row icon toolbar (playback + mode toggles)
│   │   │   │   ├── HistoryPanel.tsx     ← last 5 calls, double-click full-screen
│   │   │   │   ├── SelectTGPanel.tsx    ← TG selection slide-out panel
│   │   │   │   ├── SearchPanel.tsx      ← archive search slide-out panel
│   │   │   │   ├── BookmarkButton.tsx   ← star/flag toggle on current call
│   │   │   │   ├── BookmarksPanel.tsx   ← slide-out saved calls list
│   │   │   │   ├── WaveformVisualizer.tsx ← audio waveform (Web Audio AnalyserNode)
│   │   │   │   ├── TranscriptPanel.tsx  ← call transcript display (below display)
│   │   │   │   └── KeyboardShortcuts.tsx ← shortcut handler + help modal
│   │   │   └── admin/
│   │   │       ├── AdminLayout.tsx      ← sidebar navigation
│   │   │       ├── UsersPanel.tsx       ← user account management (admin/listener)
│   │   │       ├── SystemsPanel.tsx     ← systems + talkgroups + units CRUD
│   │   │       ├── ApiKeysPanel.tsx     ← API key management
│   │   │       ├── DirWatchPanel.tsx    ← directory watch configuration
│   │   │       ├── DownstreamsPanel.tsx ← downstream instance configuration
│   │   │       ├── GroupsTagsPanel.tsx  ← groups and tags CRUD
│   │   │       ├── OptionsPanel.tsx     ← all key/value settings
│   │   │       ├── LogsPanel.tsx        ← server log viewer
│   │   │       ├── ToolsPanel.tsx       ← CSV import, JSON export/import
│   │   │       ├── WebhooksPanel.tsx    ← webhook configuration CRUD
│   │   │       └── ActivityPanel.tsx    ← live activity stats dashboard
│   │   ├── services/
│   │   │   ├── wsClient.ts      ← WebSocket client: auto-reconnect, Redux dispatch
│   │   │   └── audioPlayer.ts   ← playback queue: HTMLAudioElement, Web Audio, beeps, preloading
│   │   ├── hooks/
│   │   │   ├── useScanner.ts    ← composite hook for scanner state + dispatch
│   │   │   ├── useAudioPlayer.ts
│   │   │   ├── useWebSocket.ts  ← initialises wsClient, exposes connection status
│   │   │   ├── useKeyboardShortcuts.ts ← keyboard event handler + shortcut map
│   │   │   └── useTheme.ts      ← dark/light theme toggle + localStorage persist
│   │   └── types/
│   │       └── index.ts         ← Call, System, Talkgroup, Group, Tag, ApiKey,
│   │                               DirWatch, Downstream, Settings, WsMessage, Bookmark,
│   │                               Webhook, PushSubscription, Transcription types
│   ├── index.html
│   ├── sw.ts                    ← Service Worker (app-shell caching for PWA)
│   ├── public/
│   │   ├── manifest.json        ← PWA manifest (app name, icons, display: standalone)
│   │   └── audio/               ← Keypad beep WAV assets (Uniden/Motorola bundled sounds)
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── tsconfig.json
│   └── package.json
├── docs/
│   ├── plan.md                  ← this file
│   ├── architecture.md          ← Mermaid system diagram + component descriptions
│   ├── api.md                   ← OpenAPI 3.1 endpoint reference
│   ├── admin-guide.md           ← UI walkthrough
│   ├── deployment.md            ← bare metal, Docker, reverse proxy
│   └── recorder-integration.md ← per-recorder setup instructions
├── e2e/
│   ├── playwright.config.ts
│   ├── package.json
│   └── specs/
│       ├── setup-wizard.spec.ts
│       ├── admin-login.spec.ts
│       ├── scanner.spec.ts
│       └── call-upload.spec.ts
├── .github/
│   ├── copilot-instructions.md
│   ├── agents/
│   │   ├── go-expert.agent.md
│   │   ├── react-expert.agent.md
│   │   ├── db-expert.agent.md
│   │   ├── docs-expert.agent.md
│   │   ├── reviewer.agent.md
│   │   └── testing-expert.agent.md
│   └── workflows/ci.yml
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── .gitignore
└── README.md
```

---

## Expert Agents

Six agent definition files live in `.github/agents/`. Each scopes itself to a domain:

| Agent                     | Domain                                                                | Scope                                              |
| ------------------------- | --------------------------------------------------------------------- | -------------------------------------------------- |
| `go-expert.agent.md`      | Go, Gin, sqlc, coder/websocket, kardianos/service, FFmpeg, Go testing | `backend/**`                                       |
| `react-expert.agent.md`   | React 18, TypeScript, DaisyUI, Tailwind, Redux Toolkit, RTK Query     | `frontend/**`                                      |
| `db-expert.agent.md`      | SQLite schema, sqlc queries, migrations, indexing                     | `backend/migrations/**`, `backend/sqlc/**`         |
| `docs-expert.agent.md`    | OpenAPI, Markdown, Mermaid architecture diagrams                      | `docs/**`                                          |
| `reviewer.agent.md`       | Security (OWASP Top 10), code quality, race conditions, performance   | `**`                                               |
| `testing-expert.agent.md` | Go `httptest`, Vitest, React Testing Library, Playwright              | `**/*_test.go`, `frontend/**/*.test.tsx`, `e2e/**` |

---

## Database Schema

All application configuration is stored in the database. Server startup configuration (listen address, DB path, TLS) is handled via CLI flags, environment variables, or an optional INI config file — see **Server Configuration** section below.

**Pragmas applied on connection open:**

```sql
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
```

### `users`

All admin and listener accounts. First-run wizard creates the initial admin user.

| Column          | Type                     | Notes                                                  |
| --------------- | ------------------------ | ------------------------------------------------------ |
| `id`            | INTEGER PK AUTOINCREMENT |                                                        |
| `username`      | TEXT UNIQUE              | login identifier                                       |
| `password_hash` | TEXT                     | bcrypt, cost ≥ 12                                      |
| `role`          | TEXT                     | `admin` or `listener`                                  |
| `disabled`      | INTEGER                  | 0/1                                                    |
| `systems_json`  | TEXT                     | JSON grant rules (listeners only; NULL = all systems)  |
| `expiration`    | INTEGER                  | Unix epoch, nullable (listeners only)                  |
| `limit`         | INTEGER                  | concurrent connection limit, nullable (listeners only) |
| `created_at`    | INTEGER                  | Unix epoch seconds                                     |
| `updated_at`    | INTEGER                  | Unix epoch seconds                                     |

### `app_state`

Single-row table for application-level flags.

| Column           | Type       | Notes                           |
| ---------------- | ---------- | ------------------------------- |
| `id`             | INTEGER PK | Always 1                        |
| `setup_complete` | INTEGER    | 0 = needs setup, 1 = configured |

### `settings`

Every app option is a key/value pair here.

| Column  | Type    |
| ------- | ------- |
| `key`   | TEXT PK |
| `value` | TEXT    |

**Seeded defaults:**

| Key                           | Default   | Notes                                                                                                 |
| ----------------------------- | --------- | ----------------------------------------------------------------------------------------------------- |
| `autoPopulate`                | `true`    |                                                                                                       |
| `pruneDays`                   | `7`       |                                                                                                       |
| `maxClients`                  | `200`     |                                                                                                       |
| `time12hFormat`               | `false`   | When `true`, display and history show 12-hour time (AM/PM); when `false`, 24-hour format              |
| `dimmerDelay`                 | `5000`    | ms of inactivity before display panel dims (reduces brightness); 0 = never dim                        |
| `keypadBeeps`                 | `uniden`  | `uniden`, `motorola`, or empty                                                                        |
| `duplicateDetectionTimeFrame` | `500`     | ms                                                                                                    |
| `disableDuplicateDetection`   | `false`   |                                                                                                       |
| `sortTalkgroups`              | `false`   |                                                                                                       |
| `audioConversion`             | `1`       | 0=disabled, 1=enabled, 2=enabled+norm, 3=enabled+loudnorm                                             |
| `showListenersCount`          | `false`   | When `true`, show active listener count in the status bar (`LEDPanel`) via `LSC` WS event             |
| `tagsToggle`                  | `false`   | When `true`, show tag-based toggles in the Select TG panel alongside group/system toggles             |
| `playbackGoesLive`            | `false`   | When `true`, finishing archive playback automatically switches back to LIVE mode                      |
| `searchPatchedTalkgroups`     | `false`   |                                                                                                       |
| `publicAccess`                | `false`   | When `true`, scanner is open to everyone — no login required                                          |
| `shareableLinks`              | `false`   | When `true`, public call share URLs are enabled (`/call/:id`)                                         |
| `keyboardShortcuts`           | `true`    | When `true`, keyboard shortcuts are enabled on scanner page                                           |
| `darkMode`                    | `true`    | `true` = dark theme, `false` = light theme; persisted per-instance in localStorage too                |
| `pushNotifications`           | `false`   | When `true`, browser push notification subscriptions are accepted                                     |
| `webhooksEnabled`             | `false`   | When `true`, webhook delivery is active                                                               |
| `transcriptionEnabled`        | `false`   | When `true`, calls are queued for local Whisper transcription                                         |
| `transcriptionModel`          | `base`    | Whisper model size: `tiny`, `base`, `small`, `medium`, `large`                                        |
| `transcriptionBinary`         | `whisper` | Path to local Whisper binary (e.g. `whisper`, `whisper.cpp`, or absolute path)                        |
| `transcriptionLanguage`       | `en`      | Language code for Whisper (ISO 639-1)                                                                 |
| `activityDashboard`           | `false`   | When `true`, activity stats are collected and dashboard is available                                  |
| `afsSystems`                  | ``        | AFS/P25 system IDs (comma-separated); used by `#TGAFS` mask token and AFS-aware display formatting    |
| `branding`                    | ``        | Custom text displayed in the status bar (`LEDPanel`); sent to clients via `VER` WS command            |
| `email`                       | ``        | Contact email displayed in the status bar (`LEDPanel`); sent to clients via `VER` WS command          |
| `vapidPublicKey`              | ``        | Auto-generated VAPID public key (created on first enable of `pushNotifications`); never manually set  |
| `vapidPrivateKey`             | ``        | Auto-generated VAPID private key (created on first enable of `pushNotifications`); never manually set |

### `systems`

| Column            | Type                     | Notes                            |
| ----------------- | ------------------------ | -------------------------------- |
| `id`              | INTEGER PK AUTOINCREMENT |                                  |
| `system_id`       | INTEGER UNIQUE           | user-facing radio system ID      |
| `label`           | TEXT                     | display name                     |
| `auto_populate`   | INTEGER                  | 0/1                              |
| `blacklists_json` | TEXT                     | JSON array of blacklisted TG IDs |
| `led`             | TEXT                     | CSS color string                 |
| `order`           | INTEGER                  | display order                    |

### `talkgroups`

| Column         | Type                     | Notes                           |
| -------------- | ------------------------ | ------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                 |
| `system_id`    | INTEGER FK → systems     | CASCADE DELETE                  |
| `talkgroup_id` | INTEGER                  | radio TG ID (unique per system) |
| `label`        | TEXT                     | short label                     |
| `name`         | TEXT                     | full name                       |
| `frequency`    | INTEGER                  | Hz                              |
| `led`          | TEXT                     | CSS color                       |
| `group_id`     | INTEGER FK → groups      | nullable                        |
| `tag_id`       | INTEGER FK → tags        | nullable                        |
| `order`        | INTEGER                  |                                 |

**Constraint:** `UNIQUE(system_id, talkgroup_id)`

### `units`

| Column      | Type                     | Notes          |
| ----------- | ------------------------ | -------------- |
| `id`        | INTEGER PK AUTOINCREMENT |                |
| `system_id` | INTEGER FK → systems     | CASCADE DELETE |
| `unit_id`   | INTEGER                  | radio unit ID  |
| `label`     | TEXT                     | friendly name  |
| `order`     | INTEGER                  |                |

### `groups`

| Column  | Type                     |
| ------- | ------------------------ |
| `id`    | INTEGER PK AUTOINCREMENT |
| `label` | TEXT UNIQUE              |

### `tags`

| Column  | Type                     |
| ------- | ------------------------ |
| `id`    | INTEGER PK AUTOINCREMENT |
| `label` | TEXT UNIQUE              |

### `calls`

Audio file is stored on the filesystem. Only the relative path is in the DB.

| Column             | Type                     | Notes                                               |
| ------------------ | ------------------------ | --------------------------------------------------- |
| `id`               | INTEGER PK AUTOINCREMENT |                                                     |
| `audio_path`       | TEXT                     | relative path under audio dir                       |
| `audio_name`       | TEXT                     | original filename                                   |
| `audio_type`       | TEXT                     | MIME type                                           |
| `date_time`        | INTEGER                  | Unix epoch seconds                                  |
| `frequency`        | INTEGER                  | Hz                                                  |
| `duration`         | INTEGER                  | call duration in ms                                 |
| `source`           | INTEGER                  | unit ID                                             |
| `sources_json`     | TEXT                     | JSON array `[{pos,src,tag}]`                        |
| `frequencies_json` | TEXT                     | JSON array `[{pos,freq,len,errorCount,spikeCount}]` |
| `patches_json`     | TEXT                     | JSON array of patched TG IDs                        |
| `system_id`        | INTEGER FK → systems     | CASCADE DELETE                                      |
| `talkgroup_id`     | INTEGER FK → talkgroups  | SET NULL                                            |

**Index:** `CREATE INDEX idx_calls_datetime_system_tg ON calls(date_time, system_id, talkgroup_id)`

**Index:** `CREATE INDEX idx_calls_system_tg ON calls(system_id, talkgroup_id)`

### `api_keys`

| Column         | Type                     | Notes                                 |
| -------------- | ------------------------ | ------------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                       |
| `key`          | TEXT UNIQUE              | UUID v4                               |
| `ident`        | TEXT                     | human label (e.g. "Trunk Recorder 1") |
| `disabled`     | INTEGER                  | 0/1                                   |
| `systems_json` | TEXT                     | JSON grant rules                      |
| `order`        | INTEGER                  |                                       |

### `accesses` (removed)

> **Dropped in migration 019.** The legacy access codes / PIN-based listener authentication feature has been removed. Authentication is now purely user-based (JWT) or public access mode.

### `dirwatches`

| Column         | Type                     | Notes                                                                           |
| -------------- | ------------------------ | ------------------------------------------------------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                                                                 |
| `directory`    | TEXT                     | absolute path to watch                                                          |
| `type`         | TEXT                     | `trunk-recorder`, `sdrtrunk`, `rtlsdr-airband`, `dsdplus`, `proscan`, `voxcall` |
| `mask`         | TEXT                     | meta-mask pattern                                                               |
| `extension`    | TEXT                     | file extension filter                                                           |
| `frequency`    | INTEGER                  | fallback frequency Hz                                                           |
| `delay`        | INTEGER                  | ms polling delay                                                                |
| `delete_after` | INTEGER                  | 0/1 delete file after import                                                    |
| `use_polling`  | INTEGER                  | 0/1 use polling instead of fsnotify (for CIFS/NFS)                              |
| `disabled`     | INTEGER                  | 0/1                                                                             |
| `system_id`    | INTEGER FK → systems     | nullable                                                                        |
| `talkgroup_id` | INTEGER FK → talkgroups  | nullable                                                                        |
| `order`        | INTEGER                  |                                                                                 |

### `downstreams`

| Column         | Type                     | Notes                           |
| -------------- | ------------------------ | ------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                 |
| `url`          | TEXT                     | target instance base URL        |
| `api_key`      | TEXT                     | API key for the target instance |
| `systems_json` | TEXT                     | JSON grant/filter rules         |
| `disabled`     | INTEGER                  | 0/1                             |
| `order`        | INTEGER                  |                                 |

### `logs`

| Column      | Type                     | Notes                   |
| ----------- | ------------------------ | ----------------------- |
| `id`        | INTEGER PK AUTOINCREMENT |                         |
| `date_time` | INTEGER                  | Unix epoch seconds      |
| `level`     | TEXT                     | `info`, `warn`, `error` |
| `message`   | TEXT                     |                         |

### `bookmarks`

User-saved calls for later review.

| Column       | Type                     | Notes                                            |
| ------------ | ------------------------ | ------------------------------------------------ |
| `id`         | INTEGER PK AUTOINCREMENT |                                                  |
| `call_id`    | INTEGER FK → calls       | CASCADE DELETE                                   |
| `user_id`    | INTEGER FK → users       | CASCADE DELETE; nullable for public listeners    |
| `session_id` | TEXT                     | localStorage key for public (non-auth) bookmarks |
| `created_at` | INTEGER                  | Unix epoch seconds                               |

**Index:** `CREATE UNIQUE INDEX idx_bookmarks_user_call ON bookmarks(user_id, call_id)`

### `webhooks`

Outbound webhook configurations for call event delivery.

| Column         | Type                     | Notes                                              |
| -------------- | ------------------------ | -------------------------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                                    |
| `url`          | TEXT                     | Delivery endpoint URL                              |
| `type`         | TEXT                     | `generic`, `discord`                               |
| `secret`       | TEXT                     | HMAC signing secret for generic webhooks; nullable |
| `systems_json` | TEXT                     | JSON grant/filter rules (null = all systems)       |
| `disabled`     | INTEGER                  | 0/1                                                |
| `order`        | INTEGER                  |                                                    |

### `push_subscriptions`

Browser push notification subscriptions.

| Column         | Type                     | Notes                                                 |
| -------------- | ------------------------ | ----------------------------------------------------- |
| `id`           | INTEGER PK AUTOINCREMENT |                                                       |
| `user_id`      | INTEGER FK → users       | CASCADE DELETE; nullable for public listeners         |
| `session_id`   | TEXT                     | localStorage key for public (non-auth) subscriptions  |
| `endpoint`     | TEXT                     | Push service URL                                      |
| `keys_json`    | TEXT                     | JSON `{p256dh, auth}`                                 |
| `systems_json` | TEXT                     | JSON filter — which systems/TGs trigger notifications |
| `created_at`   | INTEGER                  | Unix epoch seconds                                    |

### `transcriptions`

Speech-to-text results for calls.

| Column        | Type                     | Notes                  |
| ------------- | ------------------------ | ---------------------- |
| `id`          | INTEGER PK AUTOINCREMENT |                        |
| `call_id`     | INTEGER FK → calls       | CASCADE DELETE; UNIQUE |
| `text`        | TEXT                     | Full transcript text   |
| `language`    | TEXT                     | Detected language code |
| `model`       | TEXT                     | Whisper model used     |
| `duration_ms` | INTEGER                  | Processing time in ms  |
| `created_at`  | INTEGER                  | Unix epoch seconds     |

**Index:** `CREATE INDEX idx_transcriptions_text ON transcriptions(text)` (for full-text search)

---

## API Surface

### Setup (unauthenticated — disabled after first-run)

| Method | Path                | Description                                                               |
| ------ | ------------------- | ------------------------------------------------------------------------- |
| GET    | `/api/health`       | Health/readiness probe — returns `{status: "ok", version: "..."}`         |
| GET    | `/api/setup/status` | Returns `{needsSetup: bool, publicAccess: bool}`                          |
| POST   | `/api/setup`        | `{username, password}` → creates initial admin user, marks setup complete |

### Auth

| Method | Path                 | Auth | Description                                                         |
| ------ | -------------------- | ---- | ------------------------------------------------------------------- |
| POST   | `/api/auth/login`    | —    | `{username, password}` → `{token, user, passwordNeedChange}`        |
| POST   | `/api/auth/logout`   | JWT  | Invalidates token                                                   |
| PUT    | `/api/auth/password` | JWT  | `{currentPassword, newPassword}` — any user can change own password |
| GET    | `/api/auth/me`       | JWT  | Returns current user profile (id, username, role)                   |

### Call Ingest

| Method | Path                              | Auth                  | Description                                                        |
| ------ | --------------------------------- | --------------------- | ------------------------------------------------------------------ |
| POST   | `/api/call-upload`                | API key (`X-API-Key`) | Multipart — all recorder types (Trunk Recorder, SDRTrunk, voxcall) |
| POST   | `/api/trunk-recorder-call-upload` | API key               | Trunk Recorder–specific wrapper                                    |

**Call Ingest Behavior:**

- **Duplicate detection:** Rejects calls matching an existing call on the same talkgroup within `duplicateDetectionTimeFrame` ms (200 OK with `{"message": "duplicate"}`)
- **Auto-populate:** When `autoPopulate=true`, upserts system and talkgroup from incoming call metadata
- **Per-API-key rate limit:** Configurable, default 60 requests/min per API key
- **Call pruning:** Background goroutine (1-hour ticker) deletes calls + audio files older than `pruneDays`; uses batch deletion (500 rows per batch with `runtime.Gosched()` yields) to avoid long DB locks; bookmarked calls are exempt from pruning
- **Audio conversion:** FFmpeg worker pool converts uploaded audio to m4a; 4 modes controlled by `audioConversion` setting (0=disabled, 1=enabled, 2=+normalization, 3=+loud normalization); invoked via arg slice (never shell string)

### Config

| Method | Path                | Auth | Description                           |
| ------ | ------------------- | ---- | ------------------------------------- |
| GET    | `/api/admin/config` | JWT  | Full config JSON                      |
| PUT    | `/api/admin/config` | JWT  | Update config; broadcasts `CFG` on WS |

### Admin CRUD (all JWT-protected, admin role required)

All resources support `GET` (list), `POST` (create), `PUT /:id` (update), `DELETE /:id`.

**User management constraints:** Admin cannot delete own account; password is accepted on create (hashed server-side); role change requires admin; disabled users cannot authenticate.

| Resource    | Path                     |
| ----------- | ------------------------ |
| Users       | `/api/admin/users`       |
| Systems     | `/api/admin/systems`     |
| Talkgroups  | `/api/admin/talkgroups`  |
| Units       | `/api/admin/units`       |
| Groups      | `/api/admin/groups`      |
| Tags        | `/api/admin/tags`        |
| API Keys    | `/api/admin/apikeys`     |
| DirWatches  | `/api/admin/dirwatches`  |
| Downstreams | `/api/admin/downstreams` |

**Read-only endpoints** (no POST/PUT/DELETE):

| Method | Path                               | Description                                                              |
| ------ | ---------------------------------- | ------------------------------------------------------------------------ |
| GET    | `/api/admin/logs?from=&to=&level=` | Filterable log viewer (logs are server-generated, never created via API) |

### Import / Export

| Method | Path                           | Auth | Description             |
| ------ | ------------------------------ | ---- | ----------------------- |
| POST   | `/api/admin/import/talkgroups` | JWT  | CSV upload              |
| POST   | `/api/admin/import/units`      | JWT  | CSV upload              |
| GET    | `/api/admin/export/config`     | JWT  | Full JSON config export |
| POST   | `/api/admin/import/config`     | JWT  | Full JSON config import |

### Shareable Call Links (when `shareableLinks` enabled)

| Method | Path                   | Auth | Description                                              |
| ------ | ---------------------- | ---- | -------------------------------------------------------- |
| GET    | `/api/calls/:id/share` | —    | Returns call metadata + audio stream for public playback |
| GET    | `/call/:id`            | —    | Serves `SharedCall.tsx` page (frontend route, not API)   |

### Bookmarks

| Method | Path                     | Auth           | Description                  |
| ------ | ------------------------ | -------------- | ---------------------------- |
| GET    | `/api/bookmarks`         | JWT or session | List user’s bookmarked calls |
| POST   | `/api/bookmarks`         | JWT or session | `{callId}` → bookmark a call |
| DELETE | `/api/bookmarks/:callId` | JWT or session | Remove bookmark              |

### Webhooks (admin only)

| Method | Path                           | Auth        | Description       |
| ------ | ------------------------------ | ----------- | ----------------- |
| GET    | `/api/admin/webhooks`          | JWT (admin) | List all webhooks |
| POST   | `/api/admin/webhooks`          | JWT (admin) | Create webhook    |
| PUT    | `/api/admin/webhooks/:id`      | JWT (admin) | Update webhook    |
| DELETE | `/api/admin/webhooks/:id`      | JWT (admin) | Delete webhook    |
| POST   | `/api/admin/webhooks/:id/test` | JWT (admin) | Send test payload |

### Push Notifications (when `pushNotifications` enabled)

| Method | Path                  | Auth           | Description                                |
| ------ | --------------------- | -------------- | ------------------------------------------ |
| GET    | `/api/push/vapid-key` | —              | Returns server’s VAPID public key          |
| POST   | `/api/push/subscribe` | JWT or session | Register push subscription with TG filters |
| DELETE | `/api/push/subscribe` | JWT or session | Unsubscribe                                |
| PUT    | `/api/push/subscribe` | JWT or session | Update TG filter on existing subscription  |

### Transcriptions (when `transcriptionEnabled` enabled)

| Method | Path                                  | Auth          | Description                                                |
| ------ | ------------------------------------- | ------------- | ---------------------------------------------------------- |
| GET    | `/api/calls/:id/transcript`           | JWT or public | Returns transcript for a call (404 if not yet transcribed) |
| GET    | `/api/admin/transcriptions/status`    | JWT (admin)   | Queue depth, processing stats, model info                  |
| POST   | `/api/admin/transcriptions/retry/:id` | JWT (admin)   | Re-queue a failed transcription                            |

### Activity Dashboard (when `activityDashboard` enabled)

| Method | Path                        | Auth        | Description                                            |
| ------ | --------------------------- | ----------- | ------------------------------------------------------ |
| GET    | `/api/admin/activity/stats` | JWT (admin) | Current stats: calls/hour, busiest TGs, uptime, totals |
| GET    | `/api/admin/activity/chart` | JWT (admin) | Hourly call counts for last 24h (sparkline data)       |

### OpenAPI Documentation

| Method | Path                     | Auth | Description                        |
| ------ | ------------------------ | ---- | ---------------------------------- |
| GET    | `/api/docs`              | —    | Swagger UI (embedded static files) |
| GET    | `/api/docs/openapi.yaml` | —    | Raw OpenAPI 3.1 spec               |

### CLI Management (via `cmd/server` subcommands)

These operations call the HTTP API from the command line using a stored JWT token:

| Subcommand        | Description                                                                              |
| ----------------- | ---------------------------------------------------------------------------------------- |
| `login`           | Authenticate and store JWT token locally                                                 |
| `logout`          | Clear stored JWT token                                                                   |
| `change-password` | Change own password non-interactively                                                    |
| `config-get`      | Export full config JSON to file                                                          |
| `config-set`      | Import config JSON from file                                                             |
| `user-add`        | Create a user account (admin or listener role, with optional expiration, limit, systems) |
| `user-remove`     | Remove a user account by username                                                        |

Flags: `+url` (server address, default `http://localhost:3000`), `+username`, `+password`, `+role` (`admin` or `listener`), `+token` (keystore path), `+expiration` (RFC3339), `+limit`, `+systems` (comma-separated system IDs).

Environment: `OPENSCANNER_ADMIN_PASSWORD` can provide the password instead of `+password`.

### WebSocket

| Path                | Auth                                | Description                  |
| ------------------- | ----------------------------------- | ---------------------------- |
| `GET /ws`           | Public (if enabled) or listener JWT | Listener connection          |
| `GET /api/admin/ws` | JWT (admin role)                    | Admin dashboard live updates |

### WebSocket Commands

All messages are JSON arrays: `[command, payload?, flags?]`

| Command | Direction       | Description                                                    |
| ------- | --------------- | -------------------------------------------------------------- |
| `CAL`   | Server → client | New call data                                                  |
| `CFG`   | Server → client | Full config broadcast                                          |
| `XPR`   | Server → client | Session expired                                                |
| `LCL`   | Server → client | Paginated call list (search results)                           |
| `LSC`   | Server → client | Active listeners count                                         |
| `LFM`   | Bidirectional   | Live feed map update (client sends selection, server confirms) |
| `MAX`   | Server → client | Max clients reached                                            |
| `VER`   | Server → client | Server version + branding + email                              |
| `TRN`   | Server → client | Transcript ready for a call (callId + text)                    |

**WebSocket Implementation Details:**

- **Binary audio frames:** After sending a `CAL` JSON text frame, the server immediately sends the audio file bytes as a binary WebSocket frame (avoids a separate HTTP fetch for audio data)
- **permessage-deflate compression:** Enabled via `websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover}` for reduced bandwidth
- **Non-blocking sends:** Hub uses `select` with default drop — slow clients are skipped rather than blocking the broadcast loop
- **`LSC` debouncing:** Listener-count broadcasts are debounced via `time.AfterFunc` reset (max once per 3 seconds) to avoid broadcast storms during reconnect waves
- **Per-user grant filtering:** Hub only sends `CAL` events the client is authorized to receive based on their system/TG grants; public-access clients receive all

**Reserved for future use (mobile/push notification support):**

| Command | Direction       | Description                        |
| ------- | --------------- | ---------------------------------- |
| `IOS`   | Client → server | iOS-specific client identification |
| `PID`   | Client → server | Push notification ID registration  |
| `SRV`   | Server → client | Server info                        |

---

## Server Configuration

Server startup settings cannot be stored in the database (they are needed to open the DB). These are configured via CLI flags, environment variables, or an optional INI config file (`openscanner.ini`).

### Configuration Precedence

CLI flags > environment variables > INI file > built-in defaults

### Settings

| Flag               | Env Var                      | INI Key         | Default              | Description                                                         |
| ------------------ | ---------------------------- | --------------- | -------------------- | ------------------------------------------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`         | `listen`        | `:3000`              | HTTP listen address                                                 |
| `--db-file`        | `OPENSCANNER_DB_FILE`        | `db_file`       | `openscanner.db`     | SQLite database file path                                           |
| `--base-dir`       | `OPENSCANNER_BASE_DIR`       | `base_dir`      | executable directory | Base directory for all data files                                   |
| `--ssl-listen`     | `OPENSCANNER_SSL_LISTEN`     | `ssl_listen`    | —                    | HTTPS listen address                                                |
| `--ssl-cert`       | `OPENSCANNER_SSL_CERT`       | `ssl_cert_file` | —                    | TLS certificate file (PEM)                                          |
| `--ssl-key`        | `OPENSCANNER_SSL_KEY`        | `ssl_key_file`  | —                    | TLS private key file (PEM)                                          |
| `--ssl-auto-cert`  | `OPENSCANNER_SSL_AUTO_CERT`  | `ssl_auto_cert` | —                    | Domain for Let's Encrypt auto-cert                                  |
| `--admin-password` | `OPENSCANNER_ADMIN_PASSWORD` | —               | —                    | Reset first admin user's password on startup                        |
| `--config`         | —                            | —               | `openscanner.ini`    | Path to INI config file                                             |
| `--config-save`    | —                            | —               | —                    | Write current flags to INI file and exit                            |
| `--version`        | —                            | —               | —                    | Print version and exit                                              |
| `--service`        | —                            | —               | —                    | Service command: `install`, `uninstall`, `start`, `stop`, `restart` |

### System Service (Daemon)

OpenScanner can install itself as a system service via `kardianos/service`:

- **Linux:** systemd unit
- **macOS:** launchd plist
- **Windows:** Windows Service

Usage: `openscanner --service install`, then `openscanner --service start`.

---

## First-Run Flow

```
1. Server boots, reads config from flags/env/INI
2. golang-migrate runs all pending migrations
3. seed.go runs → inserts default settings rows, default groups and tags, creates app_state row (setup_complete=0)
4. Frontend loads → GET /api/setup/status → {needsSetup: true}
5. Frontend redirects to /setup
6. User enters desired admin username + password → POST /api/setup
7. Server creates admin user (bcrypt-hashed password), sets setup_complete=1
8. Frontend redirects to /login
9. All future boots: setup_complete=1 → /api/setup/status → {needsSetup: false} → wizard disabled
```

---

## Authentication & RBAC

### Roles

| Role       | Permissions                                                                        |
| ---------- | ---------------------------------------------------------------------------------- |
| `admin`    | Full access: dashboard, config, user management, all CRUD, admin WS                |
| `listener` | Scanner UI only: WS listen (filtered by per-user system/TG grants), archive search |

### User Accounts

- Stored in `users` table with bcrypt-hashed passwords (cost ≥ 12)
- Login via `POST /api/auth/login` with `{username, password}` → JWT token
- JWT tokens: max 5 concurrent per user; oldest invalidated on 6th login
- JWT payload includes `userId`, `username`, `role`; middleware checks role on protected routes
- Rate limit: 3 failed login attempts → 10-minute lockout per IP (in-memory)
- `passwordNeedChange: true` returned if password has never been changed after account creation

### Admin

- First-run setup wizard creates the initial admin user (`POST /api/setup` with `{username, password}`)
- Admins can create/edit/disable/delete other users (admin or listener) via `/api/admin/users`
- Admins cannot delete their own account
- `--admin-password` flag or `OPENSCANNER_ADMIN_PASSWORD` env var resets the first admin user's password on startup

### Listener Users (WebSocket)

- Listener users authenticate via WS by sending JWT token (obtained from `POST /api/auth/login`)
- Per-user grants: expiration date, concurrent connection limit, system/TG access rules (stored on user row)
- Disabled users are rejected at login

### Public Access (Open Listening)

- Controlled by the `publicAccess` setting (default: `false`), toggled in admin Options panel
- When enabled, **any visitor can open the scanner and listen without logging in**
- Public listeners have access to all systems/talkgroups (no server-side filtering)
- Public listeners can use the Select TG panel to filter their own feed — selection is stored client-side in `localStorage` and sent to the server via `LFM` command (same as authenticated listeners); this is a per-session preference, not persisted on the server
- Public listeners are still subject to `maxClients` connection limit
- Admins can enable this for community/hobbyist deployments where open access is desired
- When disabled, listeners must authenticate via JWT (listener user account)
- The admin dashboard always requires admin JWT regardless of this setting

### API Keys (Call Ingest)

- UUID v4 keys
- Sent via `X-API-Key` header or `?key=` query param
- Per-key system/TG access rules; can be enabled/disabled

---

## Web UI Design

OpenScanner's UI is a **purpose-built radio monitoring interface** — data-dense, responsive, and keyboard-friendly. The display panel uses a dark monospace readout for at-a-glance call data, while controls use a modern **icon toolbar** with contextual tooltips. The overall aesthetic is closer to a professional monitoring dashboard than a physical scanner replica.

### Design Principles

1. **Dark-first, light-capable** — ships with dark and light DaisyUI themes; user toggles via sun/moon icon; preference persisted in `localStorage`
2. **Monitoring dashboard** — dense data readout on top; clean icon toolbar below; no skeuomorphic hardware imitation
3. **Mobile-friendly** — scanner is fully usable on a phone in portrait mode
4. **Minimal chrome** — no top nav bar on scanner; admin uses a sidebar
5. **Density** — display shows maximum information at a glance without scrolling
6. **Keyboard-driven** — every scanner action has a keyboard shortcut; help modal via `?`
7. **Accessible offline** — PWA with Service Worker caching + push notification support

### DaisyUI Theme Configuration

```js
// tailwind.config.ts — daisyUI themes (dark + light)
daisyui: {
  themes: [
    {
      "openscanner-dark": {
        "primary":          "#00e676",   // green — live LED, active states
        "primary-content":  "#000000",
        "secondary":        "#ff9100",   // orange — paused/archive states
        "accent":           "#29b6f6",   // blue — info, links
        "neutral":          "#1e1e1e",   // main background
        "neutral-content":  "#e0e0e0",   // text on neutral
        "base-100":         "#121212",   // deepest background
        "base-200":         "#1e1e1e",   // card/panel background
        "base-300":         "#2d2d2d",   // elevated surfaces (buttons)
        "info":             "#29b6f6",
        "success":          "#00e676",
        "warning":          "#ffea00",
        "error":            "#ff1744",
      },
    },
    {
      "openscanner-light": {
        "primary":          "#2e7d32",   // dark green — legible on light bg
        "primary-content":  "#ffffff",
        "secondary":        "#e65100",   // dark orange
        "accent":           "#0277bd",   // dark blue
        "neutral":          "#f5f5f5",   // light background
        "neutral-content":  "#1e1e1e",   // dark text
        "base-100":         "#ffffff",   // white background
        "base-200":         "#f5f5f5",   // light grey panels
        "base-300":         "#e0e0e0",   // elevated surfaces
        "info":             "#0277bd",
        "success":          "#2e7d32",
        "warning":          "#f9a825",
        "error":            "#c62828",
      },
    },
  ],
  darkTheme: "openscanner-dark",
}
```

Theme is toggled by setting `data-theme="openscanner-dark"` or `data-theme="openscanner-light"` on the `<html>` element. The `useTheme` hook reads the server `darkMode` setting as default and stores the user's override in `localStorage`.

### LED Color Map

LED dot (12×24px, top-right of status bar) indicates scanner state:

| State              | Color            | CSS                       | Animation                    |
| ------------------ | ---------------- | ------------------------- | ---------------------------- |
| Live — receiving   | Green `#00e676`  | `box-shadow: 0 0 6px 3px` | Solid                        |
| Live — idle        | Green `#00e676`  | Same                      | Solid (dimmer)               |
| Paused             | Last color       | Same                      | `blink 2s step-end infinite` |
| Playback (archive) | Orange `#ff9100` | Same                      | Solid                        |
| No link            | Off `#505050`    | No shadow                 | None                         |

Per-talkgroup LED colors (configurable in TG settings): green, blue, cyan, magenta, orange, red, white, yellow.

### Responsive Breakpoints

| Breakpoint | Width      | Layout                                                            |
| ---------- | ---------- | ----------------------------------------------------------------- |
| `sm`       | < 640px    | Single column; history below controls; panels full-screen overlay |
| `md`       | 640–1023px | Single column; max-width 640px centered                           |
| `lg`       | ≥ 1024px   | Scanner centered at 640px; admin sidebar visible                  |

### Scanner Page Layout

The scanner page is a single vertically-stacked column, centered, max-width 640px, with 24px padding.

**Multi-instance support:** The `?id=` URL parameter creates isolated localStorage keys for TG selection, allowing multiple scanner instances with different TG configurations in separate browser tabs (e.g., `/scanner?id=police`, `/scanner?id=fire`).

```
┌─────────────────────────────────────────────┐
│ OPENSCANNER                    [☼/☾] [LED]│  ← Status bar + theme toggle
├─────────────────────────────────────────────┤
│ 12:34:56              L: 3           Q: 2   │  ← Row 1: clock, listeners, queue
│                                             │  ← Row 2: (spacer, small text)
│ System Name                    Tag Name     │  ← Row 3: system + tag
│ TG Label                 04/10  12:34:56    │  ← Row 4: TG label + date/time
│                                             │
│           ████ Talkgroup Name ████          │  ← Row 5: TG name (large, bold)
│                                             │
│ F: 851.025                   TGID: 12345    │  ← Row 6: frequency + TGID
│ E: 0  S: 0                    UID: 54321    │  ← Row 7: errors, spikes, unit ID
│                [☆] [↗]  ⏲ 30M  AVOID  PATCH  │  ← Row 8: bookmark, share, flags
│─────────────────────────────────────────────│
│ “Police Dispatch: requesting backup...”      │  ← Transcript (if available)
│─────────────────────────────────────────────│
│ Time     │ System   │ Talkgroup │ Name      │  ← History header
│ 12:34:50 │ Police   │ Dispatch  │ Main Disp │  ← History row (bold = playing)
│ 12:34:32 │ Fire     │ Tac 1     │ Fire Tac  │
│ 12:34:11 │ Police   │ Patrol    │ North Pct │
│ 12:33:58 │ EMS      │ Dispatch  │ EMS Disp  │
│ 12:33:40 │ Police   │ Dispatch  │ Main Disp │
├─────────────────────────────────────────────┤
│                                             │
│  ⏵  ⏸  ⏭  ⟲  │  🔇━━━━●━━━━🔊  │  ⬇  ☆  │  ← Toolbar row 1
│                                             │
│  LIVE   HOLD▾  AVOID▾  SELECT▾  SEARCH  ⋯  │  ← Toolbar row 2
│                                             │
└─────────────────────────────────────────────┘
```

#### Status Bar (`LEDPanel.tsx`)

- Flex row: branding text (left, uppercase, letter-spacing 2px) + theme toggle (sun/moon icon button) + LED dot (right)
- Branding text from `branding` setting (default: "OPENSCANNER")
- Theme toggle: `☼` (sun) in dark mode, `☾` (moon) in light mode; 20px icon; calls `useTheme().toggle()`; hidden when `darkMode` setting is `false`
- LED is a 12×24px rectangle with colored `box-shadow` glow
- Height: 1.5rem; margin-bottom: 24px

#### Display Panel (`DisplayPanel.tsx`)

- **Dark surface** (`base-200` background) with subtle inner shadow
- 8 rows of monospace-style data (font-size 14px, line-height 20px)
- Row 5 (TG name) is large: font-size 24px, line-height 32px, font-weight bold
- Row 8 (flags) right-aligned; AVOID/PATCH badges shown as small pills with `base-300` bg
- **Bookmark star** (`BookmarkButton.tsx`): `☆` (outline) / `★` (filled) icon button on row 8; toggles bookmark for current call; only shown when a call is loaded
- **Share icon**: `↗` icon button on row 8; copies shareable call link to clipboard; only shown when `shareableLinks` setting is enabled
- Double-click anywhere → fullscreen modal (same display, scaled up)
- When idle (no call playing): slightly dimmed background
- When auth required: centered unlock code input overlaid on display

#### Transcript Panel (`TranscriptPanel.tsx`)

- Embedded between the 8-row display and history table, inside the same dark surface
- Only rendered when `transcriptionEnabled` setting is `true` **and** the current call has a transcript
- Single-line or multi-line text, font-size 13px, italic, `neutral-content` at 80% opacity
- Wrapped in a collapsible `<details>` element (open by default); clicking the summary row collapses/expands
- Receives live updates via WS `TRN` event — text appears shortly after call finishes playing
- If no transcript available: element is hidden (no empty placeholder)

#### History Table (inside `DisplayPanel`)

- Embedded below the 8-row display, inside the same dark surface (not a separate panel)
- Table with 4 columns: Time (10%), System (25%), Talkgroup (25%), Name (40%)
- Font-size 11px; rows 21px tall; header text 40% opacity
- Currently-playing row has `font-weight: 700`
- Shows last 5 calls; rows separated by 1px border at 20% opacity
- **Bookmark indicator**: small `★` star icon (8px) appended to the Name column for bookmarked calls
- **Share button**: small `↗` icon on hover/tap on each row (only when `shareableLinks` enabled); copies link to clipboard

#### Control Toolbar (`ControlToolbar.tsx`)

Controls are arranged as a **two-row icon toolbar** — compact, modern, and touch-friendly. No skeuomorphic hardware buttons.

**Row 1 — Playback + Quick Actions** (horizontal flex, centered, `gap-2`):

- **Play/Pause** (`⏵` / `⏸`): `btn btn-circle btn-primary` (44px); toggles live feed or pauses playback; `primary` fill when live, `secondary` fill when paused
- **Skip** (`⏭`): `btn btn-circle btn-ghost` (36px); skips current call
- **Replay** (`⟲`): `btn btn-circle btn-ghost` (36px); replays the previous call
- **Divider**: 1px vertical rule (`divider divider-horizontal`)
- **Volume slider**: inline `<input type="range">` (120px wide, 36px tall); DaisyUI `range range-xs range-primary`; hidden on mobile — tap volume icon to toggle popover
- **Divider**: 1px vertical rule
- **Download** (`⬇`): `btn btn-circle btn-ghost` (36px); downloads current call audio file
- **Bookmark** (`☆` / `★`): `btn btn-circle btn-ghost` (36px); `text-warning` when active

**Row 2 — Mode Toggles** (horizontal flex, centered, `gap-2`):

- **LIVE**: `btn btn-sm` — toggles live feed on/off; `btn-primary` when active, `btn-ghost` when off; pulsing green dot indicator when receiving
- **HOLD▾**: `btn btn-sm` dropdown — on click shows dropdown menu with "Hold System" and "Hold Talkgroup" options; `btn-secondary` when either hold is active
- **AVOID▾**: `btn btn-sm` dropdown — on click shows dropdown menu with duration options (30m, 60m, 120m, Permanent) for the current TG; `btn-warning` when avoids are active; badge count of avoided TGs
- **SELECT▾**: `btn btn-sm` — opens the Select TG slide-out panel
- **SEARCH**: `btn btn-sm` — opens the Search panel
- **⋯ (overflow)**: `btn btn-sm btn-ghost` — dropdown with: Saved Calls, Fullscreen, Keyboard Shortcuts (`?`)

**Active state indicators:**

- Active toggles use filled button variants (`btn-primary`, `btn-secondary`, `btn-warning`)
- Inactive toggles use `btn-ghost` (transparent background, visible on hover)
- Hold/Avoid dropdowns show a colored badge count when items are held/avoided
- LIVE button has a 6px pulsing green dot (`animate-pulse`) when actively receiving calls

**Touch & responsive:**

- Row 1 icons: 44px touch target on `sm`, 36px on `md`+
- Row 2 buttons: min-height 36px, `gap-1` on `sm` to fit all buttons
- Volume slider collapses to icon-only on `sm`; tapping opens a vertical popover slider
- Overflow menu (`⋯`) absorbs Saved/Fullscreen/Shortcuts on `sm` to save space; on `lg` all items are inline

**Tooltips:**

- Every toolbar button has a DaisyUI `tooltip` (bottom) showing the action name + keyboard shortcut: e.g. "Pause (Space)", "Skip (S)", "Replay (R)"
- Tooltips hidden on touch devices (shown only on hover)

### Select TG Panel (`SelectTGPanel.tsx`)

Slides in from the **right** edge (full-width on mobile, 400px on desktop). Uses a **collapsible accordion** layout with chip-style toggles.

```
┌──────────────────────────────────┐
│ Select Talkgroups          [← X] │  ← Title + close
├──────────────────────────────────┤
│  Groups:                         │
│  [Law ✔] [Fire ✔] [EMS] [All ✔]  │  ← Group chip toggles
│  [All Off]  [All On]             │  ← Global actions
├──────────────────────────────────┤
│  ▼ Police                  [2/5] │  ← System accordion (expanded)
│    [● Dispatch ✔]  [● Tac 1]     │  ← TG chips (● = LED color)
│    [● Tac 2 ✔]    [● Patrol]    │
│    [● Records]                   │
│    [Off]  [On]                   │  ← Per-system quick actions
├──────────────────────────────────┤
│  ▶ Fire                    [0/3] │  ← System accordion (collapsed)
├──────────────────────────────────┤
│  ▶ EMS                    [1/2] │
└──────────────────────────────────┘
```

- **Group chips** at top: DaisyUI `btn btn-xs` toggles; `btn-primary` when active, `btn-ghost` when off; partial state shown with `btn-outline btn-primary`
- **System accordions**: DaisyUI `collapse collapse-arrow bg-base-200`; header shows system name + active/total count badge (`badge badge-sm`); click to expand/collapse
- **TG chips**: DaisyUI `btn btn-xs` with a 6px colored left border matching the TG’s LED color; `btn-primary` when enabled, `btn-ghost` when disabled; `animate-pulse` border for temporarily avoided TGs
- **Per-system Off/On**: `btn btn-xs btn-ghost` quick-action links below each system’s chips
- **Global All Off / All On**: `btn btn-sm btn-outline` at the top
- Virtual scrolling for systems with many TGs (via `@tanstack/react-virtual`)
- State persisted to `localStorage` keyed by `?id=` URL param

### Search Panel (`SearchPanel.tsx`)

Slides in from the **left** edge (full-width on mobile, 500px on desktop). Uses a **split layout**: results list on top, collapsible filter form below.

```
┌───────────────────────────────────────┐
│ Search Calls               [X →] │  ← Title + close
├───────────────────────────────────────┤
│  ⊳ 12:34 │ Police │ Dispatch │ ★   │  ← Result row + bookmark
│  ⊳ 12:33 │ Fire   │ Tac 1    │     │
│  ■ 12:32 │ EMS    │ Dispatch │     │  ← Playing (stop icon)
│  ⊳ 12:31 │ Police │ Patrol   │     │
│  ⊳ 12:30 │ Police │ Dispatch │ ★   │
│  ... (virtualized)                │
│───────────────────────────────────────│
│  [◀ Prev]  Page 1 of 10  [Next ▶] │  ← Paginator
│  [💾 Download mode]                │  ← Toggle download vs play
├───────────────────────────────────────┤
│  ▼ Filters                  [3]   │  ← Collapsible (shows active count)
│  Transcript [________________]   │
│  System     [All Systems   ▾]    │
│  Talkgroup  [All Talkgroups▾]    │
│  Group      [All Groups    ▾]    │
│  Tag        [All Tags      ▾]    │
│  Date from  [__________]         │
│  Date to    [__________]         │
│  Sort       [Newest first  ▾]    │
│  [☆ Bookmarked only]              │
│  [Reset filters]                  │
└───────────────────────────────────────┘
```

- **Results list**: virtualized via `@tanstack/react-virtual` (not paginated-only); each row is a flex row with play/stop icon, time, system, TG name, bookmark indicator
- **Paginator**: `btn btn-sm` prev/next + page count; sits between results and filters
- **Download mode**: `toggle toggle-primary` to switch play buttons to download buttons
- **Filters section**: DaisyUI `collapse collapse-arrow`; header shows count of active filters as a `badge`; expands on click
- **Transcript search**: `input input-bordered input-sm`; queries `GET /api/calls?transcript=<text>` with `LIKE` search on `transcriptions.text`; only shown when `transcriptionEnabled` setting is `true`
- **Bookmarked only**: `toggle toggle-sm` to filter to bookmarked calls only
- **Bookmark indicator**: `★` star icon (text-warning) after the TG name for bookmarked calls
- **Reset filters**: `btn btn-ghost btn-sm` clears all active filters
- All filter inputs use DaisyUI `select select-bordered select-sm` and `input input-bordered input-sm`
- Loading state: `loading loading-spinner` overlaid on results area

### Bookmarks Panel (`BookmarksPanel.tsx`)

Slides in from the **right** edge (full-width on mobile, 400px on desktop). Triggered by the overflow menu (⋯) → "Saved Calls" or the `B` keyboard shortcut.

```
┌──────────────────────────────────┐
│ Saved Calls (12)         [← X] │  ← Title + count + close
├──────────────────────────────────┤
│  ⊳ 12:34 │ Police │ Dispatch  [✕]│  ← Row + remove button
│  ⊳ 08:21 │ Fire   │ Tac 1    [✕]│
│  ⊳ 15:02 │ EMS    │ Dispatch [✕]│
│  ⊳ 22:45 │ Police │ Patrol   [✕]│
│  ... (scrollable)               │
├──────────────────────────────────┤
│  [⬇ Download all]  [🗑 Clear]   │  ← Bulk actions
└──────────────────────────────────┘
```

- Lists all bookmarked calls for the current user (authenticated) or from `localStorage` (public)
- Each row: play icon, time, system, TG name, `✕` remove button (always visible)
- Swipe-left on mobile reveals remove action as alternative gesture
- **Download all**: downloads a ZIP of all bookmarked call audio files
- **Clear all**: removes all bookmarks (with confirmation dialog)
- Sorted by bookmark date (newest first)
- Empty state: centered text "No saved calls yet" with a hint about the bookmark star icon

### Keyboard Shortcuts Help (`KeyboardShortcuts.tsx`)

DaisyUI `modal` overlay triggered by pressing `?` on the scanner page. Only shown when `keyboardShortcuts` setting is `true`.

```
┌──────────────────────────────────┐
│ Keyboard Shortcuts        [× X] │
├──────────────────────────────────┤
│  Space ········· Pause / Resume  │
│  S ············· Skip Next       │
│  R ············· Replay Last     │
│  H ············· Hold System     │
│  J ············· Hold Talkgroup  │
│  A ············· Avoid (cycle)   │
│  B ············· Toggle Bookmark │
│  F ············· Fullscreen      │
│  ← → ··········· Volume ±5%     │
│  Escape ········ Close Panel     │
│  ? ············· This Help       │
└──────────────────────────────────┘
```

- Two-column layout: key (left, `kbd` DaisyUI class) + action description (right)
- Closes on `Escape` or clicking outside
- Subtle `base-200` background with `base-300` key badges

### Shared Call Page (`SharedCall.tsx`)

Public page at `/call/:id` — no authentication required. Only accessible when `shareableLinks` setting is enabled; returns 404 otherwise.

```
┌──────────────────────────────────────────┐
│                                          │
│         🔗 OPENSCANNER                  │
│                                          │
│  System: Police      Tag: Law Dispatch   │
│  Talkgroup: Main Dispatch (TGID: 12345)  │
│  Date: 04/10/2026    Time: 12:34:56      │
│  Duration: 8.2s      Frequency: 851.025  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  ▶  ━━━━━━━━━━━●━━━━  3:21 / 8.2s│  │  ← Audio player
│  └────────────────────────────────────┘  │
│                                          │
│  "Police Dispatch: requesting backup     │  ← Transcript (if available)
│   at 5th and Main..."                    │
│                                          │
│  [ 💾 Download ]                         │
│                                          │
│  Shared from OpenScanner                 │  ← Footer with branding
└──────────────────────────────────────────┘
```

- Standalone page — no scanner chrome, no controls
- DaisyUI `card` centered on `base-100` background, max-width 500px
- Native `<audio>` element with controls (play/pause, seek, volume)
- OpenGraph `<meta>` tags in `<head>` for link previews: `og:title` (system + TG name), `og:description` (date + duration), `og:audio` (direct audio URL)
- Transcript shown below audio player if `transcriptionEnabled` and transcript exists
- Download button fetches the audio file directly
- If call doesn't exist or `shareableLinks` is disabled: shows a simple "Call not found" message

### Login Page (`Login.tsx`)

Centered card on `base-100` background (adapts to theme):

```
┌──────────────────────────────┐
│                              │
│        🔒 OPENSCANNER       │
│                              │
│   Username: [____________]   │
│   Password: [____________]   │
│                              │
│        [ Sign In ]           │
│                              │
│   Incorrect credentials.     │  ← error toast (conditional)
│                              │
└──────────────────────────────┘
```

- DaisyUI `card` component, max-width 400px, centered vertically and horizontally
- `base-200` card on `base-100` background
- Primary-colored "Sign In" button
- On `passwordNeedChange=true` response: show change-password form inline
- Non-admin users see an error message (cannot access admin dashboard)

### Setup Page (`Setup.tsx`)

Centered card, similar to login but with step indicator:

```
┌──────────────────────────────┐
│                              │
│      ⚙ Initial Setup        │
│      Step 1 of 1             │
│                              │
│   Username: [____________]   │
│   Password: [____________]   │
│   Confirm:  [____________]   │
│                              │
│       [ Create Admin ]       │
│                              │
└──────────────────────────────┘
```

- Only shown when `GET /api/setup/status` returns `needsSetup=true`
- After submission, redirects to `/login`
- Password validation: minimum 8 characters

### Admin Dashboard (`AdminLayout.tsx`)

```
┌──────┬───────────────────────────────────────────┐
│      │  Users                                    │  ← Page title
│ USR  ├───────────────────────────────────────────┤
│ SYS  │                                           │
│ GRP  │  ┌─────────────────────────────────────┐  │
│ API  │  │ Username │ Role  │ Disabled │ Expire │  │  ← Data table
│ ACC  │  │──────────│───────│──────────│────────│  │
│ DIR  │  │ admin    │ Admin │    ✗     │   —    │  │
│ DWN  │  │ user1    │ Listn │    ✗     │ 12/31  │  │
│ OPT  │  │ user2    │ Listn │    ✓     │   —    │  │
│ LOG  │  └─────────────────────────────────────┘  │
│ TLS  │                                           │
│      │  [+ Add User]                             │
│ OUT  │                                           │
└──────┴───────────────────────────────────────────┘
  sidebar                    content area
```

- **Sidebar** (left): icon-only on mobile (`sm`), icon + label on `lg`
- Width: 64px collapsed, 200px expanded
- Sidebar items (abbreviated): USR=Users, SYS=Systems, GRP=Groups & Tags, API=API Keys, DIR=Dir Watches, DWN=Downstreams, OPT=Options, LOG=Logs, TLS=Tools, Scanner (Home icon), OUT=Sign Out
- Active item highlighted with `primary` color left border
- **Content area**: max-width 1200px, padding 24px
- Each panel is a DaisyUI `card` with a data table inside
- Tables use DaisyUI `table table-zebra` styling
- Action buttons (edit, delete, toggle) in last column
- Add/create forms: DaisyUI `modal` triggered by button at bottom
- `SystemsPanel`: expandable rows with nested talkgroup/unit lists; drag-to-reorder via `@dnd-kit`; virtual scrolling for large systems
- `LogsPanel`: virtualized table with live WS updates; date range + level filter at top
- `ToolsPanel`: file upload areas (CSV/JSON), password change form
- **`WebhooksPanel`**: CRUD table for webhook configs (URL, type `generic`/`discord`, system/TG filter, enabled toggle); "Test" button sends a sample payload and shows success/failure; only visible when `webhooksEnabled` setting is `true`
- **`ActivityPanel`**: stat cards (calls today / this week / all time, server uptime, active listeners, transcription queue depth) + sparkline chart (calls per hour, last 24h) + busiest TGs list; refreshes on panel focus; only visible when `activityDashboard` setting is `true`
- **`OptionsPanel`**: includes toggle switches for all extended features (`shareableLinks`, `keyboardShortcuts`, `darkMode`, `pushNotifications`, `webhooksEnabled`, `transcriptionEnabled`, `activityDashboard`) with descriptions; transcription sub-settings (binary path, model size dropdown, language input) shown conditionally when transcription is enabled
- Error feedback: DaisyUI `toast` (bottom-right, auto-dismiss 5s)
- Optimistic updates: immediate UI change, revert on API error

#### Admin Responsive Behavior

| Breakpoint        | Sidebar                                | Content                     |
| ----------------- | -------------------------------------- | --------------------------- |
| `sm` (< 640px)    | Hidden; hamburger menu opens as drawer | Full width                  |
| `md` (640–1023px) | Icons only (64px)                      | Remaining width             |
| `lg` (≥ 1024px)   | Icons + labels (200px)                 | Remaining width, max 1200px |

### Typography

| Element              | Font                                 | Size | Weight                  |
| -------------------- | ------------------------------------ | ---- | ----------------------- |
| Body                 | System sans-serif (Tailwind default) | 14px | 400                     |
| Display rows         | `font-mono` (monospace)              | 14px | 400                     |
| Display TG name      | `font-mono`                          | 24px | 700                     |
| History table        | System sans-serif                    | 11px | 400                     |
| Toolbar icons        | System sans-serif                    | 20px | 400                     |
| Toolbar mode toggles | System sans-serif                    | 12px | 500                     |
| Admin headings       | System sans-serif                    | 20px | 600                     |
| Branding text        | System sans-serif                    | 16px | 400, letter-spacing 2px |

### Spacing System

- All spacing uses Tailwind's 4px grid: `p-1` (4px), `p-2` (8px), `p-4` (16px), `p-6` (24px)
- Scanner outer padding: `p-6` (24px)
- Button gaps: `gap-2` (8px) between toolbar icons; `gap-2` between mode toggles
- Section gaps: `mb-4` (16px) between status bar, display, and toolbar
- Admin content padding: `p-6` (24px)

### Animations & Transitions

| Animation            | Duration | Easing                 | Used On                                       |
| -------------------- | -------- | ---------------------- | --------------------------------------------- |
| LED blink (paused)   | 2s       | `step-end` infinite    | LED dot when paused                           |
| TG avoid pulse       | 1.5s     | `ease-in-out` infinite | TG chip border when avoided (`animate-pulse`) |
| LIVE receiving pulse | 1.5s     | `ease-in-out` infinite | Green dot on LIVE button (`animate-pulse`)    |
| Panel slide-in       | 300ms    | `ease-in-out`          | SelectTG, Search, and Bookmarks panels        |
| Dropdown open        | 150ms    | `ease-out`             | HOLD▾ and AVOID▾ dropdown menus               |
| Toast appear         | 200ms    | `ease-out`             | Admin error/success toasts                    |
| Display dim          | 500ms    | `ease`                 | Display background when idle                  |
| Tooltip fade         | 100ms    | `ease-in`              | Toolbar button tooltips                       |

### DaisyUI Component Mapping

| UI Element                                | DaisyUI Class(es)                                                                       |
| ----------------------------------------- | --------------------------------------------------------------------------------------- |
| Playback buttons (play/pause/skip/replay) | `btn btn-circle btn-primary` / `btn-ghost`                                              |
| Mode toggles (LIVE/HOLD/AVOID)            | `btn btn-sm btn-primary` / `btn-secondary` / `btn-warning` / `btn-ghost`                |
| Dropdown menus (HOLD▾, AVOID▾)            | `dropdown` + `menu bg-base-200 shadow-lg`                                               |
| Volume slider                             | `range range-xs range-primary`                                                          |
| Toolbar dividers                          | `divider divider-horizontal`                                                            |
| Toolbar tooltips                          | `tooltip tooltip-bottom`                                                                |
| Overflow menu (⋯)                         | `dropdown dropdown-end` + `menu`                                                        |
| Theme toggle                              | `btn btn-ghost btn-circle` (sun/moon icon)                                              |
| Bookmark star                             | `btn btn-ghost btn-xs` (☆/★ icon, `text-warning` when filled)                           |
| Share button                              | `btn btn-ghost btn-xs` (↗ icon)                                                         |
| Transcript panel                          | `collapse collapse-open bg-base-300`                                                    |
| SelectTG panel                            | `drawer` slide-out + `collapse collapse-arrow` accordions + `btn btn-xs` chip toggles   |
| Search panel                              | `drawer` slide-out + `collapse collapse-arrow` filter section + virtualized result list |
| Bookmarks panel                           | `drawer` slide-out (same layout as Search/SelectTG)                                     |
| Keyboard shortcuts modal                  | `modal` + `modal-box` + `kbd` for key badges                                            |
| Shared call page                          | `card bg-base-200 shadow-xl` centered                                                   |
| Admin sidebar                             | `menu bg-base-200`                                                                      |
| Admin tables                              | `table table-zebra`                                                                     |
| Form inputs                               | `input input-bordered`                                                                  |
| Dropdowns (filters)                       | `select select-bordered`                                                                |
| Modals (add/edit)                         | `modal` + `modal-box`                                                                   |
| Toasts                                    | `toast` + `alert`                                                                       |
| Cards (admin panels)                      | `card bg-base-200`                                                                      |
| Toggles                                   | `toggle toggle-primary`                                                                 |
| Badges (role, status)                     | `badge badge-primary` / `badge-secondary`                                               |
| Stat cards (activity)                     | `stat` inside `stats shadow`                                                            |
| Sparkline chart                           | Custom SVG or `<canvas>` — no chart library                                             |
| Loading states                            | `loading loading-spinner`                                                               |
| Login/Setup card                          | `card bg-base-200 shadow-xl`                                                            |
| Buttons (admin)                           | `btn btn-primary` / `btn-ghost` / `btn-error`                                           |
| Date pickers                              | Native `input[type=datetime-local]` styled with `input input-bordered`                  |
| Pagination                                | Custom (prev/next buttons with `btn btn-sm`)                                            |

### OpenScanner Design Identity

OpenScanner has its own visual language — it is **not** a clone or reskin of any existing scanner application.

| Design Aspect    | OpenScanner Approach                                                                        |
| ---------------- | ------------------------------------------------------------------------------------------- |
| Controls         | Modern icon toolbar (DaisyUI `btn` + `dropdown`); no skeuomorphic hardware buttons          |
| Layout           | Two-row toolbar: playback icons on top, mode toggles below                                  |
| State indication | Filled button variants for active states; badge counts; pulsing dots — no LED grids         |
| Theme            | Dual dark/light DaisyUI themes with one-click toggle                                        |
| Mobile           | Responsive breakpoints; overflow menu; touch-sized targets                                  |
| Display          | Monospace data readout with inner shadow on dark surface                                    |
| Interactions     | Tooltips with keyboard hints; dropdown menus; inline volume slider                          |
| Features         | Bookmarks, shareable links, transcription, push notifications, webhooks, activity dashboard |
| Admin            | Same-SPA sidebar layout; feature toggles in Options panel                                   |
| API              | OpenAPI/Swagger docs at `/api/docs`                                                         |
| PWA              | Service Worker + manifest + push notification support                                       |

---

## Extended Features

All extended features are **configurable** — disabled by default (except keyboard shortcuts). Each can be enabled/disabled via the admin Options panel or the `settings` table.

### Shareable Call Links

- **Setting:** `shareableLinks` (default: `false`)
- When enabled, each call gets a public URL: `/call/<id>`
- `SharedCall.tsx` page renders a minimal embedded player: call metadata (system, TG, date, time, duration) + audio `<audio>` element + DOWNLOAD button
- Backend endpoint `GET /api/calls/:id/share` returns call metadata and streams audio; returns 404 if `shareableLinks` is disabled or call doesn't exist
- No authentication required — the URL is the share token
- Share button appears on call history rows and search results when enabled
- Metadata includes OpenGraph tags for link previews (Discord, Slack, Twitter embeds)

### Keyboard Shortcuts

- **Setting:** `keyboardShortcuts` (default: `true`)
- `useKeyboardShortcuts.ts` hook registers key handlers on the scanner page
- Shortcuts:

| Key       | Action                         |
| --------- | ------------------------------ |
| `Space`   | Pause / Resume                 |
| `S`       | Skip next                      |
| `R`       | Replay last                    |
| `H`       | Hold current TG                |
| `J`       | Hold current system            |
| `A`       | Avoid (cycle 30/60/120 min)    |
| `F`       | Toggle fullscreen              |
| `←` / `→` | Volume down / up (5% steps)    |
| `?`       | Show/hide shortcuts help modal |
| `B`       | Bookmark current call          |
| `Escape`  | Close any open panel           |

- `KeyboardShortcuts.tsx` — help overlay modal listing all shortcuts (triggered by `?`)
- Shortcuts are disabled when any input/textarea is focused
- Disabled entirely when `keyboardShortcuts` setting is `false`

### Dark / Light Theme Toggle

- **Setting:** `darkMode` (default: `true`)
- DaisyUI supports multiple themes — define both `openscanner-dark` and `openscanner-light` themes in `tailwind.config.ts`
- `useTheme.ts` hook: reads server `darkMode` setting as default; user can override locally (stored in `localStorage`)
- Toggle button in scanner status bar (sun/moon icon) and admin sidebar
- Theme is applied by setting `data-theme` attribute on `<html>` element

### OpenAPI / Swagger Docs

- OpenAPI 3.1 YAML spec file describing all endpoints, request/response schemas, auth schemes
- Embedded Swagger UI served at `/api/docs` via `go:embed` (swagger-ui-dist static files)
- Raw spec available at `/api/docs/openapi.yaml`
- Always available (not toggleable) — useful for third-party integrations

### Activity Dashboard / Stats

- **Setting:** `activityDashboard` (default: `false`)
- When enabled, the backend collects activity metrics on each call ingest:
  - Calls per hour (rolling 24h window)
  - Busiest talkgroups (top 10 by call count in last 24h)
  - Total calls today / this week / all time
  - Server uptime
  - Active listeners count
  - Transcription queue depth (if transcription enabled)
- `ActivityPanel.tsx` in admin dashboard: sparkline chart (calls/hour for last 24h), stat cards, busiest TG list
- Data served via `GET /api/admin/activity/stats` and `GET /api/admin/activity/chart`
- Stats are computed on-demand from the `calls` table (no separate stats table needed for SQLite)

### Call Bookmarking

- **Setting:** always available (no toggle — lightweight feature)
- Authenticated users: bookmarks stored in `bookmarks` table (foreign key to `users` + `calls`)
- Public listeners: bookmarks stored in `localStorage` (keyed by session ID); a `session_id` is stored in the DB row for cross-tab consistency
- `BookmarkButton.tsx` — star icon on the current call display + on history/search rows; toggles on click
- `BookmarksPanel.tsx` — slide-out panel (accessible via a SAVED button or keyboard shortcut `B`) listing all bookmarked calls with play/download/unbookmark actions
- Bookmarks persist across sessions for authenticated users; cleared on browser data clear for public listeners
- Bookmarked calls are excluded from auto-pruning (they are only deleted when the user un-bookmarks them or the call is manually deleted by admin)

### Push Notifications

- **Setting:** `pushNotifications` (default: `false`)
- Uses the Web Push protocol (RFC 8030) via `github.com/SherClockHolmes/webpush-go`
- VAPID key pair auto-generated on first enable and stored in `settings` table (`vapidPublicKey`, `vapidPrivateKey`)
- Frontend: notification permission prompt on first enable; user selects which systems/TGs to notify on
- `push_subscriptions` table stores endpoint + keys per user (or per session for public listeners)
- Backend: after call ingest, if call matches any subscription's TG filter, send push notification with call summary (system, TG name, time, duration)
- Notification click opens the scanner page and plays the call
- Service Worker handles push events (`self.addEventListener('push', ...)` in `sw.ts`)

### Webhook / Discord Integration

- **Setting:** `webhooksEnabled` (default: `false`)
- `webhooks` table stores delivery targets with type (`generic` or `discord`), URL, signing secret, and system/TG filter
- **Generic webhooks:** POST JSON payload to URL; `X-OpenScanner-Signature` header with HMAC-SHA256 of body using `secret` field
- **Discord webhooks:** POST Discord-formatted embed (title, description, color, fields for system/TG/time/duration, audio URL if `shareableLinks` enabled)
- Delivery runs in a background goroutine pool (separate from FFmpeg workers); retries 3× with exponential backoff (1s, 4s, 16s)
- Failed deliveries logged to `logs` table
- `WebhooksPanel.tsx` in admin: CRUD for webhook configs + test button
- Admin can filter per webhook which systems/TGs trigger delivery

### Call Transcription (Local Whisper Binary)

- **Setting:** `transcriptionEnabled` (default: `false`)
- Uses a local Whisper binary (e.g. `whisper.cpp`, OpenAI `whisper` CLI, or `faster-whisper`) — **not** an API service
- **Settings:**
  - `transcriptionBinary` — path to Whisper executable (default: `whisper`)
  - `transcriptionModel` — model size: `tiny`, `base`, `small`, `medium`, `large` (default: `base`)
  - `transcriptionLanguage` — ISO 639-1 language code (default: `en`)
- **Processing pipeline:**
  1. After call ingest + audio conversion, call is queued for transcription
  2. Transcription worker pool (bounded, default 1 worker for GPU exclusivity) invokes: `<binary> --model <model> --language <lang> --output-format txt <audio_file>`
  3. Output text stored in `transcriptions` table (one row per call)
  4. WS broadcast `TRN` event to connected clients with `{callId, text}`
- **Frontend:**
  - `TranscriptPanel.tsx` — expandable panel below the display showing transcript of current call
  - Search panel gains a "Search transcripts" text input — queries `GET /api/calls?transcript=<text>` which performs a `LIKE` search on `transcriptions.text`
  - Transcript text displayed in call share page when available
- **Docker GPU passthrough:**
  - `Dockerfile` has a separate build target: `FROM nvidia/cuda:12.6.0-runtime-ubuntu24.04 AS runtime-gpu`
  - `docker-compose.yml` includes a `gpu` service profile with `deploy.resources.reservations.devices` for NVIDIA GPU
  - Non-GPU image uses CPU-only Whisper (slower but functional)
  - Startup check: if `transcriptionEnabled` is true and binary is not found, log warning and disable transcription (non-fatal)

---

## Implementation Phases

### Phase 1 — Foundation & Scaffolding ✅ COMPLETE

**Goal:** All tooling set up; `make build` and `make test` run without errors.

1. Initialise Go module (`go.mod`) and install all dependencies
2. Initialise frontend with `pnpm create vite`, add DaisyUI, Tailwind, Redux Toolkit, RTK Query, React Router
3. Root `Makefile` with `build`, `dev`, `test`, `lint`, `migrate`, `generate` targets
4. Backend `Makefile` delegating to Go toolchain
5. `docker-compose.yml` for local dev (mounts `./data` and `./audio`, exposes `:3000`)
6. Add `internal/config/config.go` — CLI flag parsing, env var binding, INI file loading
7. Dev tooling: `air` config (`.air.toml`) for Go hot-reload + Vite `proxy` config pointing at Go backend; `make dev` runs both via `Make` (or `concurrently`)
8. `.vscode/agents/` — all 6 agent definition files _(already scaffolded)_
9. `.github/copilot-instructions.md` _(already scaffolded)_
10. `.github/workflows/ci.yml` _(already scaffolded)_

**Deliverables:** Compiling Go binary, running Vite dev server, passing empty test suites. ✅ All verified.

**Agents:** Go Expert (Go module, Makefile, config), React Expert (Vite init, Tailwind/DaisyUI setup), Docs Expert (CI workflow, copilot-instructions).

**References:** [Repository Layout](#repository-layout), [Server Configuration](#server-configuration), [DaisyUI Theme Configuration](#daisyui-theme-configuration).

---

### Phase 2 — Database Schema & Seeding ✅ COMPLETE

**Goal:** All tables created, sqlc generates typed Go code, first-run detection works.

1. Write all 14 numbered migration SQL files (full `CREATE TABLE IF NOT EXISTS` statements)
2. Write `sqlc.yaml` configuration
3. Write all 14 `sqlc/queries/*.sql` files (CRUD queries per table, including `users.sql` and `app_state.sql`)
4. Run `sqlc generate` → produces `internal/db/` typed Go files
5. Write `internal/seed/seed.go` — inserts all default `settings` rows, default groups (`Air`, `EMS`, `Fire`, `Interop`, `Law`, `Unknown`), default tags (`Air Traffic Control`, `Emergency`, `Fire Dispatch`, `Fire Tac`, `Fire Talk`, `Interop`, `Security`, `Service`, `Untagged`), creates `app_state` row (setup_complete=0)
6. Write `internal/db/db.go` — opens SQLite connection, applies WAL pragmas (`journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`), runs migrations on startup
7. Configure `log/slog` in `cmd/server/main.go` — JSON handler for production, Text handler for development; all packages use `slog.Info`/`slog.Warn`/`slog.Error` with structured key-value pairs (no `log.Println`)
8. Wire migration runner into `cmd/migrate/main.go`

**Deliverables:** `make migrate` creates a valid SQLite DB; `make generate` produces no errors.

**Agents:** Database Expert (migrations, sqlc config, queries), Go Expert (db.go, seed.go, slog config, migrate CLI).

**References:** [Database Schema](#database-schema) (all 18 tables), [Settings](#settings) (seeded defaults).

---

### Phase 3 — Backend Auth, RBAC & Setup ✅ COMPLETE

**Goal:** User accounts with roles work end-to-end; setup wizard creates initial admin; JWT auth enforces role-based access.

1. `internal/auth/auth.go` — bcrypt hash/verify, JWT sign/verify (HS256 with 32-byte secret), max-5-token enforcement per user; JWT payload: `{userId, username, role, exp}`; role-aware middleware helpers (`RequireAdmin`, `RequireAuth`)
2. `internal/auth/ratelimit.go` — in-memory rate limiter (3 fails → 10-min lockout per IP)
3. `internal/middleware/middleware.go` — `JWTAuth` (extracts user + role from token), `RequireAdmin` (rejects non-admin), `APIKeyAuth`, `RateLimit` Gin middleware; `RequestID` middleware (UUID v4 per request, injected into `slog` context and `X-Request-ID` response header); CSRF protection via `SameSite=Strict` cookie attribute on JWT (relies on JWT-in-header pattern for inherent CSRF safety)
4. `internal/api/setup.go` — `GET /api/setup/status`, `POST /api/setup` (accepts `{username, password}`, creates admin user, sets `app_state.setup_complete=1`); blocked once setup is complete
5. `internal/api/admin.go` — `POST /api/auth/login` (username + password → JWT with role), `POST /api/auth/logout`, `PUT /api/auth/password` (any authenticated user changes own password), `GET /api/auth/me` (returns current user profile)
6. `internal/api/health.go` — `GET /api/health` → `{status: "ok", version: "..."}`; Docker HEALTHCHECK target
7. `internal/api/routes.go` — register all Phase 3 routes; admin routes use `RequireAdmin` middleware
8. Unit tests: JWT round-trip with role claims, bcrypt verify, rate limiter lockout, setup endpoint disables itself, role-based route rejection (listener → admin route = 403), request ID propagation

**Deliverables:** `go test ./internal/auth/... ./internal/api/... -run TestSetup` passes.

**Agents:** Go Expert (auth, middleware, API handlers, routes), Testing Expert (unit + integration tests), Reviewer (security audit: bcrypt cost, JWT claims, rate limiter, CSRF).

**References:** [Authentication & RBAC](#authentication--rbac) (roles, JWT, rate limiting, CSRF), [Setup](#setup-unauthenticated--disabled-after-first-run) + [Auth](#auth) endpoints, [First-Run Flow](#first-run-flow).

---

### Phase 4 — Backend Call Ingest ✅ COMPLETE

**Goal:** Audio files accepted, stored, and converted; duplicates rejected.

1. `internal/audio/processor.go` — save multipart file to filesystem under audio dir, sanitise path (no `../`), submit conversion job to FFmpeg worker pool for m4a conversion with configurable mode:
   - `0` = disabled (keep original)
   - `1` = enabled (`-c:a aac -b:a 32k`)
   - `2` = enabled + normalization (`-c:a aac -b:a 32k -af acompressor`)
   - `3` = enabled + loud normalization (`-c:a aac -b:a 32k -af loudnorm`)
     Return stored path
2. `internal/audio/worker.go` — bounded FFmpeg worker pool: `runtime.NumCPU()` workers reading from a buffered channel; each job spawns FFmpeg via `exec.CommandContext` (arg slice, never shell string); context-aware graceful drain on shutdown
3. `internal/audio/duplicate.go` — query last call per talkgroup within `duplicateDetectionTimeFrame` ms from `settings`; return bool
4. `internal/api/calls.go` — `POST /api/call-upload` (parse all fields, validate API key, duplicate check, store, broadcast `CAL` on WS, queue downstream push); `POST /api/trunk-recorder-call-upload` wrapper; per-API-key rate limiter (configurable, default 60 requests/min)
5. Auto-populate: if `autoPopulate=true`, upsert system + talkgroup from incoming call metadata
6. Call pruning goroutine: started in `main.go`; 1-hour ticker deletes calls + audio files older than `pruneDays`; **batch deletion** — delete in batches of 500 rows with `runtime.Gosched()` yields to avoid long DB locks
7. Unit tests: path sanitiser, duplicate detection logic, multipart parsing, worker pool drain

**Deliverables:** `POST /api/call-upload` with a valid API key returns 200; call appears in DB.

**Agents:** Go Expert (audio processor, FFmpeg worker pool, call API, pruning goroutine), Testing Expert (unit tests), Reviewer (path traversal sanitisation, arg-slice FFmpeg invocation).

**References:** [Call Ingest](#call-ingest) endpoints + behavior, [Settings](#settings) (`audioConversion`, `duplicateDetectionTimeFrame`, `pruneDays`, `autoPopulate`).

---

### Phase 5 — WebSocket Hub ✅ COMPLETE

**Goal:** Real-time call broadcast to all connected listeners.

1. `internal/ws/hub.go` — hub with `register`, `unregister`, `broadcast` channels; runs in a single goroutine; all sends are non-blocking (`select` with default drop); **permessage-deflate** compression enabled via `websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover}`
2. `internal/ws/client.go` — per-connection struct; separate goroutines for read pump and write pump; listener client authenticates via JWT token (listener user) **or** connects freely when `publicAccess` setting is enabled; admin client validates JWT with admin role; **binary audio frames**: after sending `CAL` JSON text frame, immediately send the audio file bytes as a binary WebSocket frame (avoids a separate HTTP fetch)
3. `internal/ws/messages.go` — typed constants and builder helpers for `CAL`, `CFG`, `XPR`, `LCL`, `LSC`, `LFM`, `MAX`, `VER`; reserved stubs for `IOS`, `PID`, `SRV`
4. Wire hub into `cmd/server/main.go`; register `GET /ws` and `GET /api/admin/ws` upgrade handlers; **graceful shutdown** with `context.WithCancel` + `srv.Shutdown(ctx)` (drain WS connections before exit)
5. `LSC` broadcast on every connect/disconnect — **debounced** via `time.AfterFunc` reset (max once per 3 seconds to avoid broadcast storms during reconnect waves)
6. `CFG` broadcast triggered by `PUT /api/admin/config`
7. Per-user system/TG grant filtering — hub only sends `CAL` events the client is authorised to receive; public-access clients receive all systems/TGs (no filtering)
8. Unit tests: hub broadcast, client auth, grant filtering

**Deliverables:** After call upload, connected WS client receives `CAL` event within 500ms.

**Agents:** Go Expert (hub, client, message types, graceful shutdown), Testing Expert (unit tests), Reviewer (auth bypass review, grant filtering, binary frame handling).

**References:** [WebSocket](#websocket) paths, [WebSocket Commands](#websocket-commands) (message format + all commands), [Public Access](#public-access-open-listening).

---

### Phase 6 — Admin CRUD APIs ✅

**Status: COMPLETE**

**Goal:** All admin management endpoints work; config is fully DB-backed.

1. ✅ CRUD handlers for all resources: users, systems, talkgroups, units, groups, tags, apikeys, dirwatches, downstreams, webhooks
2. ✅ User management: admin can list/create/update/disable/delete users; password field accepted on create, hashed server-side; admin cannot delete own account; role validation enforced (`admin` or `listener`)
3. ✅ `GET/PUT /api/admin/config` — reads all `settings` rows as a config object; writes back individual keys (allowlist-validated); broadcasts `CFG` on `PUT`
4. ✅ `GET /api/admin/logs` — filterable by `from`, `to` (Unix timestamps), `level`; truncated to 10,000 rows with `X-Truncated` header
5. ✅ `POST /api/admin/import/talkgroups` + `POST /api/admin/import/units` — CSV parsing with header detection, safety limit (100,000 rows), system existence validation
6. ✅ `GET /api/admin/export/config` + `POST /api/admin/import/config` — full JSON config round-trip (settings, users, systems, talkgroups, units, groups, tags, apikeys, dirwatches, downstreams, webhooks) in a single transaction
7. ✅ Integration tests: every endpoint including 401 (missing JWT), 404 (not found), 422 (validation fail)

**Deliverables:** Full admin dashboard backend is functional; all 50+ endpoints return correct status codes.

**Agents:** Go Expert (CRUD handlers, config API, import/export), Database Expert (any new queries needed), Testing Expert (integration tests for all endpoints).

**References:** [Admin CRUD](#admin-crud-all-jwt-protected-admin-role-required) endpoints, [Config](#config) endpoints, [Import / Export](#import--export) endpoints, [User Accounts](#user-accounts) (constraints).

---

### Phase 7 — DirWatch Service ✅

**Status: COMPLETE**

**Goal:** Audio files dropped into watched directories are automatically ingested.

1. ✅ `internal/dirwatch/watcher.go` — `fsnotify` watcher per configured directory; polling fallback for CIFS/NFS mounts (controlled by `use_polling` column, configurable delay); restarts when dirwatch config changes via admin API
2. ✅ `internal/dirwatch/parsers.go` — one parser function per recorder type:
   - `trunk-recorder` — JSON sidecar file
   - `sdrtrunk` — filename pattern (`<sysID>_<tgID>_<ts>.<ext>`)
   - `rtlsdr-airband` — filename pattern
   - `dsdplus` — DSDPlus Fast Lane format
   - `proscan` — ProScan format
   - `voxcall` — voxcall format
3. ✅ `internal/dirwatch/mask.go` — expand all meta-mask tokens: `#DATE`, `#TIME`, `#ZTIME`, `#GROUP`, `#SYSLBL`, `#TAG`, `#TGAFS`, `#UNIT`, `#TGLBL`, `#TGHZ`, `#TGKHZ`, `#TGMHZ`, `#TGID`
4. ✅ Delete-after-import: remove source file on successful ingest if `delete_after=1`
5. ✅ Unit tests: mask expansion for all tokens, each parser with fixture files

**Deliverables:** Drop a Trunk Recorder audio file + JSON into watched dir → call appears in scanner within configured delay.

**Agents:** Go Expert (watcher, parsers, mask expansion), Testing Expert (unit tests with fixture files).

**References:** [`dirwatches` table](#dirwatches) (columns, 6 recorder types), [Settings](#settings) (`autoPopulate`).

---

### Phase 8 — Downstream Pusher ✅

**Status: COMPLETE**

**Goal:** Accepted calls are forwarded to configured remote OpenScanner instances.

1. ✅ `internal/downstream/pusher.go` — fan-out pattern: one goroutine per active downstream with a buffered channel (1000 events); exponential backoff retry (1s→2s→4s→8s→30s cap, max 5 retries, with jitter); shuts down cleanly on context cancellation
2. ✅ System/TG grant filter applied before each push — `systems_json` column controls which calls are forwarded per downstream
3. ✅ Audio file read from filesystem and re-posted as multipart to the downstream's `/api/call-upload` with `X-API-Key` header
4. ✅ Pusher reloaded when downstream config changes via admin CRUD operations (create/update/delete triggers `Reload`)
5. ✅ Log all push successes and failures to `logs` table
6. ✅ Security: HTTP client disables redirects (SSRF protection); audio path traversal protection
7. ✅ Graceful shutdown: `dsService.Stop()` called after HTTP server shutdown

**Deliverables:** Configure a downstream → upload a call → downstream instance receives it.

**Agents:** Go Expert (pusher goroutine, backoff retry, grant filter).

**References:** [`downstreams` table](#downstreams) (columns), [Call Ingest](#call-ingest) (`POST /api/call-upload` target).

---

### Phase 9 — Frontend Scanner UI ✅

**Goal:** Main scanner interface is fully functional with live audio playback.

1. `src/main.tsx` — React app entry; wrap with Redux `Provider` and `RouterProvider`
2. `src/app/store.ts` — Redux store with all slices + RTK Query middleware
3. `src/app/api.ts` — RTK Query base API with `baseUrl: '/api'`
4. `src/services/wsClient.ts` — WebSocket service: connects to `/ws`, sends JWT if authenticated, auto-reconnects with exponential backoff (1s → 2s → 4s → ... max 30s, with jitter), dispatches WS events to Redux; handles binary audio frames (reads `Blob` for playback after `CAL` JSON); when `publicAccess` is enabled, connects without sending any auth command; `XPR` handler clears credentials before disconnecting
5. `src/services/audioPlayer.ts` — call queue manager: `HTMLAudioElement` playback, Web Audio API for volume, bundled keypad beep sounds (Uniden/Motorola WAV assets in `public/audio/`); **audio preloading** — when a call is playing, preload the next queued call's audio into a second `HTMLAudioElement` for gapless transitions
6. `src/hooks/useWebSocket.ts` — initialises wsClient on mount, exposes connection status
7. `src/pages/Scanner.tsx` — main layout: LED panel (top), Display panel (centre), Controls (bottom), History panel (right/bottom)
8. `src/components/scanner/LEDPanel.tsx` — green (live), orange (paused archive), blink (paused); CSS custom property for per-TG color
9. `src/components/scanner/DisplayPanel.tsx` — 6-line display; double-click → full-screen modal
10. `src/components/scanner/ControlToolbar.tsx` — two-row icon toolbar: playback controls (play/pause, skip, replay, volume, download, bookmark) + mode toggles (LIVE, HOLD▾, AVOID▾, SELECT▾, SEARCH, overflow ⋯)
11. `src/components/scanner/HistoryPanel.tsx` — scrollable last-5-calls list; double-click row → full-screen
12. `src/app/slices/scannerSlice.ts` — state: `isLive`, `isPaused`, `heldSystem`, `heldTG`, `avoidList`, `callQueue`, `currentCall`, `history`
13. `?id=` URL param: each unique ID gets its own localStorage key for TG selection (multi-instance support)
14. `frontend/sw.ts` — Service Worker for PWA app-shell caching: cache HTML, JS, CSS, and font assets on install; network-first for API calls; enables instant load on repeat visits and mobile home screen install
15. `frontend/public/manifest.json` — PWA manifest with app name, icons, `display: standalone`, dark theme color
16. Startup: call `GET /api/setup/status`; redirect to `/setup` if `needsSetup=true`; if `publicAccess=true`, connect WS immediately with no auth; otherwise redirect to `/login` for JWT authentication
17. Unit tests: LEDPanel renders all state variants, ControlToolbar dispatch correct Redux actions

**Deliverables:** Scanner page loads; live WS events update the display; audio plays.

**Agents:** React Expert (all scanner components, Redux slices, WS client, audio player, PWA service worker), Testing Expert (unit tests).

**References:** [Scanner Page Layout](#scanner-page-layout) (wireframe), [Status Bar](#status-bar-ledpaneltsx), [Display Panel](#display-panel-displaypaneltsx), [Control Toolbar](#control-toolbar-controltoolbartsx), [History Table](#history-table-inside-displaypanel), [WebSocket Commands](#websocket-commands), [Settings](#settings) (`publicAccess`, `keypadBeeps`, `dimmerDelay`), [First-Run Flow](#first-run-flow).

---

### Phase 10 — Frontend TG Selection & Search Panels ✅

**Goal:** Talkgroup selection and archive search are fully operational.

1. `src/components/scanner/SelectTGPanel.tsx` — slide-out from right:
   - Groups section: ON/OFF/PARTIAL tri-state (group ON if all TGs on, PARTIAL if mixed, OFF if all off)
   - ALL ON / ALL OFF buttons per group
   - Systems section: per-system TG toggle list; LED flash = temporarily avoided TG
   - ALL ON / ALL OFF per system
   - State persisted to localStorage (keyed by `?id=` param)
2. Avoid talkgroup: AVOID button cycles 30/60/120 min; countdown tracked in `scannerSlice`; avoided TG LED flashes in SelectTG panel
3. `src/components/scanner/SearchPanel.tsx` — slide-out from right:
   - RTK Query paginated call list via `GET /api/calls` with query params
   - PLAY / DOWNLOAD toggle per result row
   - Filters: system, TG, group, tag, date-from, date-to, sort direction
   - Patched talkgroup search toggle (controlled by `searchPatchedTalkgroups` setting)
4. `src/app/slices/callsSlice.ts` — search filter state
5. Unit tests: SelectTGPanel tri-state logic, SearchPanel filter param construction

**Deliverables:** TG selection persists after reload; archive search returns filtered paginated results.

**Agents:** React Expert (SelectTG panel, Search panel, slices), Testing Expert (unit tests).

**References:** [Select TG Panel](#select-tg-panel-selecttgpaneltsx) (wireframe + tri-state logic), [Search Panel](#search-panel-searchpaneltsx) (wireframe + filters), [Settings](#settings) (`searchPatchedTalkgroups`, `tagsToggle`, `sortTalkgroups`).

---

### Phase 11 — Frontend Admin Dashboard ✅ COMPLETE

**Goal:** Admin dashboard is fully functional for all configuration tasks.

1. `src/pages/AdminLogin.tsx` — login form (username + password); on successful login store JWT in memory (not localStorage); if `passwordNeedChange=true` redirect to change-password modal; non-admin users are rejected with an error message
2. `src/pages/Setup.tsx` — first-run wizard: form for admin username + password; calls `POST /api/setup`; redirects to `/login` on success
3. `src/components/admin/AdminLayout.tsx` — sidebar nav with links to all panels; protected route (redirect to login if no token)
4. `src/pages/Admin.tsx` — renders `AdminLayout` + outlet for panel routes
5. All admin panel components (each connects to RTK Query mutations for their resource):
   - `UsersPanel` — user accounts table: username, role badge (admin/listener), disabled toggle, expiration, connection limit, system grant editor; create-user form with role selector and password field
   - `SystemsPanel` — systems table with expandable nested talkgroup and unit sub-lists; drag-to-reorder via `@dnd-kit`; **virtual scrolling** via `@tanstack/react-virtual` for systems with many talkgroups
   - `ApiKeysPanel` — generate UUID, copy-to-clipboard, enable/disable toggle, system grant editor, drag-to-reorder
   - `DirWatchPanel` — directory path, type dropdown, mask field, extension, delay, delete-after toggle
   - `DownstreamsPanel` — URL, API key, system grant editor, enable/disable
   - `GroupsTagsPanel` — two simple tables: groups and tags with add/rename/delete
   - `OptionsPanel` — all settings key/value pairs rendered as appropriate input types (toggle, number, text); `publicAccess` toggle shown prominently with a warning badge explaining it opens the scanner to unauthenticated listeners
   - `LogsPanel` — date range pickers, level filter dropdown, **virtualized** scrollable log table; live updates via admin WS
   - `ToolsPanel` — CSV import (talkgroups/units), JSON config export button, JSON config import upload, change own password form
6. `src/app/slices/adminSlice.ts` + `src/app/slices/authSlice.ts` — RTK Query endpoints for all admin resources
7. Error toasts on mutation failures; optimistic updates where appropriate
8. Unit tests: AdminLogin redirect on `passwordNeedChange`, Setup form submission

**Deliverables:** Full admin dashboard works end-to-end; all config changes survive server restart.

**Agents:** React Expert (all admin panel components, RTK Query mutations, auth flow), Testing Expert (unit tests).

**References:** [Admin Dashboard](#admin-dashboard-adminlayouttsx) (wireframe + all panel specs), [Admin Responsive Behavior](#admin-responsive-behavior), [Auth](#auth) endpoints, [Admin CRUD](#admin-crud-all-jwt-protected-admin-role-required) endpoints, [Login Page](#login-page-logintsx), [Setup Page](#setup-page-setuptsx).

---

### Phase 12 — CLI, Daemon, SSL, Docker & Deployment

**Goal:** Production-ready binary with CLI management, system service support, Docker image; HTTPS support; single-file deployment via `go:embed`.

1. CLI subcommands in `cmd/server/main.go`: `login`, `logout`, `change-password`, `config-get`, `config-set`, `user-add`, `user-remove` — all call the HTTP API using a locally stored JWT token
2. System service support via `kardianos/service`: `--service install|uninstall|start|stop|restart`
3. **`go:embed`** — embed the `frontend/dist/` directory into the Go binary (`//go:embed all:frontend/dist`); serve via `http.FileServer` with Gin middleware fallback; enables single-file deployment with no external static files
4. TLS in Gin: `router.RunTLS(addr, certFile, keyFile)` with command-line flags `--ssl-listen`, `--ssl-cert`, `--ssl-key`
5. Let's Encrypt auto-cert via `golang.org/x/crypto/acme/autocert` (flag: `--ssl-auto-cert <domain>`)
6. Multi-stage `Dockerfile`:
   - Stage 1 (Go): `FROM golang:1.24-alpine AS go-builder` → `go build -o /openscanner ./cmd/server`
   - Stage 2 (Node): `FROM node:22-alpine AS node-builder` → `pnpm install && pnpm build`
   - Stage 3 (Runtime — CPU): `FROM alpine:3.21` + FFmpeg + CA certs + Whisper.cpp CPU binary; copies Go binary (frontend already embedded)
   - Stage 4 (Runtime — GPU): `FROM nvidia/cuda:12.6.0-runtime-ubuntu24.04` + FFmpeg + Whisper.cpp with CUDA; same Go binary
7. `docker-compose.yml` — single-service compose with volume mounts for data and audio dirs; `HEALTHCHECK CMD curl -f http://localhost:3000/api/health || exit 1`; optional `gpu` profile with `deploy.resources.reservations.devices` for NVIDIA GPU passthrough
8. `docs/deployment.md` — nginx reverse proxy example, Caddy Caddyfile example, bare-metal systemd service file, `--service install` usage, GPU passthrough instructions (Docker `--gpus`, compose GPU profile)
9. Startup check: warn (not fatal) if FFmpeg is not found; log warning and disable audio conversion; same for Whisper binary when transcription enabled

**Deliverables:** `docker build -t openscanner .` succeeds; `docker run -p 3000:3000 openscanner` serves the app; CLI subcommands work; `--service install` installs system service; `docker build --target runtime-gpu -t openscanner:gpu .` builds GPU image.

**Agents:** Go Expert (CLI subcommands, kardianos/service, go:embed, TLS), Docs Expert (deployment.md, Dockerfile, docker-compose, nginx/Caddy configs), Reviewer (Dockerfile security: non-root user, minimal base image, no secrets in layers).

**References:** [CLI Management](#cli-management-via-cmdserver-subcommands) (subcommands + flags), [Server Configuration](#server-configuration) (SSL flags, INI file), [System Service](#system-service-daemon).

---

### Phase 13 — Testing

**Goal:** Comprehensive automated test coverage across all layers.

#### Go Unit Tests

| Test file                            | Covers                                                                                        |
| ------------------------------------ | --------------------------------------------------------------------------------------------- |
| `internal/auth/auth_test.go`         | JWT sign → verify with role claims, expired token rejection, max-5-per-user token enforcement |
| `internal/auth/ratelimit_test.go`    | 3 failures → lockout, lockout expiry after 10 minutes                                         |
| `internal/audio/duplicate_test.go`   | Duplicate detection within/outside timeframe                                                  |
| `internal/audio/processor_test.go`   | Path sanitiser blocks `../`, valid paths accepted                                             |
| `internal/dirwatch/mask_test.go`     | All meta-mask tokens expand correctly                                                         |
| `internal/dirwatch/parsers_test.go`  | Each parser with fixture input files                                                          |
| `internal/audio/transcriber_test.go` | Whisper invocation arg construction, binary-not-found graceful disable                        |
| `internal/api/webhooks_test.go`      | HMAC-SHA256 signature calculation, Discord embed format, delivery retry logic                 |
| `internal/notify/push_test.go`       | VAPID key generation, subscription TG filter matching                                         |

#### Go Integration Tests (httptest)

| Test file                        | Covers                                                                                                                                                                 |
| -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/api/setup_test.go`     | `GET /api/setup/status` before/after setup; `POST /api/setup` creates admin user; blocked after complete                                                               |
| `internal/api/admin_test.go`     | Login success/fail, rate limit (429), password change, role-based access (listener → admin route = 403), all CRUD endpoints (200/201/401/403/404/422), user management |
| `internal/api/calls_test.go`     | `POST /api/call-upload` valid key → 200 + WS CAL; invalid key → 401; duplicate → 409                                                                                   |
| `internal/api/share_test.go`     | Share endpoint returns 200 when enabled, 404 when disabled; streams correct audio                                                                                      |
| `internal/api/bookmarks_test.go` | Create/delete/list bookmarks; user isolation; session-based bookmarks                                                                                                  |

#### Frontend Unit Tests (Vitest + React Testing Library)

| Test file                    | Covers                                                            |
| ---------------------------- | ----------------------------------------------------------------- |
| `LEDPanel.test.tsx`          | Green/orange/blink state renders, custom color CSS var            |
| `ControlToolbar.test.tsx`    | Each toolbar action dispatches correct Redux action on click      |
| `SelectTGPanel.test.tsx`     | Tri-state group logic (ON/OFF/PARTIAL), localStorage persistence  |
| `SearchPanel.test.tsx`       | Filter form updates RTK Query params                              |
| `authSlice.test.ts`          | Token stored on login, cleared on logout, role extracted from JWT |
| `scannerSlice.test.ts`       | Avoid list countdown, hold system/TG filter logic                 |
| `KeyboardShortcuts.test.tsx` | Key events dispatch correct actions, disabled when input focused  |
| `BookmarkButton.test.tsx`    | Toggle dispatches bookmark API call, star icon reflects state     |
| `useTheme.test.ts`           | Theme toggle updates data-theme, persists to localStorage         |

#### E2E Tests (Playwright)

| Spec file                    | Covers                                                                                                                                    |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `setup-wizard.spec.ts`       | Fresh DB → `/setup` redirect; wizard creates admin user; setup disabled after                                                             |
| `admin-login.spec.ts`        | Correct credentials → dashboard; wrong password → error; 3× wrong → 429; `passwordNeedChange` redirect; listener user cannot access admin |
| `scanner.spec.ts`            | Page loads; LIVE FEED toggle; SELECT TG panel; selection persists after reload; SEARCH panel filters                                      |
| `call-upload.spec.ts`        | `POST /api/call-upload` → WS CAL received → scanner display updates; history panel shows call; invalid key → 401                          |
| `keyboard-shortcuts.spec.ts` | Space pauses, S skips, R replays, ? shows help modal                                                                                      |
| `share-call.spec.ts`         | Share URL renders player with audio and metadata; 404 when feature disabled                                                               |
| `theme-toggle.spec.ts`       | Toggle switches theme; persists after reload                                                                                              |

#### CI Pipeline (`.github/workflows/ci.yml`)

- `go test ./...` on every push
- `vitest run` on every push
- Playwright on every push (after backend + frontend pass)

**Agents:** Testing Expert (all Go unit/integration tests, Vitest tests, Playwright E2E specs, CI pipeline), Go Expert (Go test helpers, httptest setup), React Expert (React Testing Library component tests).

**References:** [Verification Checklist](#verification-checklist) (35 acceptance criteria), [API Surface](#api-surface) (all endpoints for integration tests).

---

### Phase 14 — Documentation

**Goal:** All docs are complete, accurate, and match the final implementation.

1. `docs/architecture.md` — Mermaid system overview diagram, call ingest data flow diagram, WS message flow diagram, first-run flow diagram
2. `docs/api.md` — OpenAPI 3.1 YAML spec for all 30+ endpoints; Swagger UI served at `/api/docs`
3. `docs/admin-guide.md` — step-by-step UI walkthrough; screenshots/GIFs (optional)
4. `docs/deployment.md` — bare metal (Linux/macOS/Windows), Docker, docker-compose, nginx reverse proxy config, Caddy Caddyfile, Let's Encrypt, environment variables reference
5. `docs/recorder-integration.md` — per-recorder quick-start (Trunk Recorder JSON plugin config, SDRTrunk export path, RTLSDR-Airband dirwatch setup, DSDPlus Fast Lane, ProScan, voxcall)

**Agents:** Docs Expert (all documentation files, OpenAPI spec, Swagger UI integration, admin guide).

**References:** [OpenAPI Documentation](#openapi-documentation) endpoints, [Repository Layout](#repository-layout) (`docs/` directory), [API Surface](#api-surface) (all 30+ endpoints for OpenAPI spec).

---

### Phase 15 — Keyboard Shortcuts & Theme Toggle

**Goal:** Keyboard-driven scanner operation and dark/light theme switching.

1. `src/hooks/useKeyboardShortcuts.ts` — registers `keydown` handler on scanner page; maps keys to Redux actions; disabled when focus is inside `<input>`/`<textarea>` or when `keyboardShortcuts` setting is `false`
2. `src/components/scanner/KeyboardShortcuts.tsx` — `?` key opens a DaisyUI modal listing all shortcuts in a two-column table
3. `tailwind.config.ts` — define two DaisyUI themes: `openscanner-dark` (existing palette) and `openscanner-light` (inverted: light base, dark text)
4. `src/hooks/useTheme.ts` — reads `darkMode` setting from server `CFG`; user can override locally (stored in `localStorage`); sets `data-theme` attribute on `<html>`
5. Theme toggle button: sun/moon icon in scanner status bar (right of LED) and admin sidebar footer
6. Unit tests: keyboard events dispatch correct actions; theme toggle updates `data-theme`

**Deliverables:** All keyboard shortcuts work on scanner page; theme toggle persists across sessions.

**Agents:** React Expert (keyboard hook, theme hook, DaisyUI theme config, toggle component), Testing Expert (unit tests).

**References:** [Keyboard Shortcuts](#keyboard-shortcuts) (Extended Features — full shortcut list), [Dark / Light Theme Toggle](#dark--light-theme-toggle) (Extended Features), [Keyboard Shortcuts Help](#keyboard-shortcuts-help-keyboardshortcutstsx) (wireframe), [DaisyUI Theme Configuration](#daisyui-theme-configuration), [Settings](#settings) (`keyboardShortcuts`, `darkMode`).

---

### Phase 16 — Shareable Links, Bookmarks & Activity Dashboard

**Goal:** Share calls publicly, bookmark for later, view activity stats.

1. `internal/api/share.go` — `GET /api/calls/:id/share` returns call metadata + streams audio file; returns 404 if `shareableLinks` disabled; includes OpenGraph `<meta>` tags in response headers for link previews
2. `src/pages/SharedCall.tsx` — minimal public page: call info card (system, TG, date, time, duration, transcript if available) + `<audio>` player + download button; no scanner chrome
3. Share button on history rows and search results — copies `/call/<id>` URL to clipboard; only visible when `shareableLinks` is enabled
4. `bookmarks` migration + sqlc queries: `CreateBookmark`, `DeleteBookmark`, `ListBookmarksByUser`, `ListBookmarksBySession`, `IsBookmarked`
5. `src/components/scanner/BookmarkButton.tsx` — star icon; toggles bookmark via `POST/DELETE /api/bookmarks`; public listeners use sessionId
6. `src/components/scanner/BookmarksPanel.tsx` — slide-out panel listing saved calls with play/download/unbookmark
7. Bookmarked calls excluded from auto-pruning in `call_pruner` goroutine (skip calls that have a `bookmarks` foreign key)
8. `ActivityPanel.tsx` — admin panel with: calls/hour sparkline (last 24h), stat cards (today/week/total), top 10 busiest TGs, active listeners, server uptime
9. `GET /api/admin/activity/stats` + `GET /api/admin/activity/chart` — computed from `calls` table with aggregate queries
10. Unit tests: share endpoint 404 when disabled, bookmark toggle, activity stats query

**Deliverables:** Shared call URL renders player; bookmarks persist; activity dashboard shows live stats.

**Agents:** Go Expert (share API, bookmark API, activity stats endpoints), React Expert (SharedCall page, BookmarkButton, BookmarksPanel, ActivityPanel), Database Expert (bookmarks queries, activity aggregation queries), Testing Expert (unit tests), Reviewer (public share endpoint security, bookmark user isolation).

**References:** [Shareable Call Links](#shareable-call-links) (Extended Features), [Call Bookmarking](#call-bookmarking) (Extended Features), [Activity Dashboard / Stats](#activity-dashboard--stats) (Extended Features), [Shared Call Page](#shared-call-page-sharedcalltsx) (wireframe), [Bookmarks Panel](#bookmarks-panel-bookmarkspaneltsx) (wireframe), [`bookmarks` table](#bookmarks), [Shareable Call Links](#shareable-call-links-when-shareablelinks-enabled) + [Bookmarks](#bookmarks) + [Activity Dashboard](#activity-dashboard-when-activitydashboard-enabled) endpoints.

---

### Phase 17 — Push Notifications & Webhook Integration

**Goal:** Browser push notifications and outbound webhook delivery for call events.

1. `github.com/SherClockHolmes/webpush-go` dependency added
2. VAPID key pair: auto-generated on first enable of `pushNotifications` setting; stored as `vapidPublicKey`/`vapidPrivateKey` in `settings` table
3. `internal/notify/push.go` — Web Push delivery: reads `push_subscriptions` table, filters by TG match, sends push via webpush-go; handles expired/invalid subscriptions (auto-delete)
4. `push_subscriptions` migration + sqlc queries
5. Frontend: notification permission prompt; TG subscription picker modal; Service Worker `push` event handler in `sw.ts` (shows notification, click opens scanner)
6. `GET /api/push/vapid-key` + `POST/PUT/DELETE /api/push/subscribe` endpoints
7. `webhooks` migration + sqlc queries; `WebhooksPanel.tsx` admin CRUD
8. `internal/api/webhooks.go` — webhook delivery goroutine pool; generic (JSON + HMAC-SHA256 signature) and Discord (embed format) types
9. Webhook delivery: after call ingest, match against webhook TG filters, enqueue delivery; retry 3× with backoff (1s, 4s, 16s); log failures
10. `POST /api/admin/webhooks/:id/test` — sends a test payload with sample call data
11. Unit tests: VAPID key generation, push filter matching, webhook HMAC signature, Discord embed format

**Deliverables:** Push notification received on phone; Discord channel receives call embed; webhook delivery retries on failure.

**Agents:** Go Expert (webpush-go integration, webhook delivery, HMAC signing, VAPID key management), React Expert (notification permission prompt, TG subscription picker, Service Worker push handler, WebhooksPanel), Database Expert (push_subscriptions + webhooks queries), Testing Expert (unit tests), Reviewer (HMAC-SHA256 validation, push subscription cleanup, webhook secret handling).

**References:** [Push Notifications](#push-notifications) (Extended Features), [Webhook / Discord Integration](#webhook--discord-integration) (Extended Features), [`push_subscriptions` table](#push_subscriptions), [`webhooks` table](#webhooks), [Push Notifications](#push-notifications-when-pushnotifications-enabled) + [Webhooks](#webhooks-admin-only) endpoints, [Settings](#settings) (`pushNotifications`, `webhooksEnabled`, `vapidPublicKey`, `vapidPrivateKey`).

---

### Phase 18 — Call Transcription

**Goal:** Local Whisper-based speech-to-text for all ingested calls.

1. `internal/audio/transcriber.go` — transcription worker pool (default 1 worker for GPU exclusivity):
   - Reads `transcriptionBinary`, `transcriptionModel`, `transcriptionLanguage` from settings
   - Invokes Whisper via `exec.CommandContext` with arg slice (never shell string): `<binary> --model <model> --language <lang> --output-format txt <audio_file>`
   - Captures stdout as transcript text; stores in `transcriptions` table
   - On failure: logs error, does not retry automatically (admin can retry via API)
2. `transcriptions` migration + sqlc queries: `CreateTranscription`, `GetTranscriptionByCallID`, `SearchTranscriptions`
3. After call ingest + audio conversion, if `transcriptionEnabled` is `true`, queue call for transcription
4. On transcription complete, broadcast `TRN` WS event to all connected clients: `["TRN", {callId, text}]`
5. `src/components/scanner/TranscriptPanel.tsx` — collapsible panel below display; shows transcript of current call; updates live when `TRN` event arrives
6. Search panel: add "Search transcripts" text input; `GET /api/calls?transcript=<text>` queries `transcriptions.text` via `LIKE %text%`
7. `SharedCall.tsx` — show transcript text below audio player (if available)
8. `GET /api/admin/transcriptions/status` — returns queue depth, completed count, average processing time, current model
9. `POST /api/admin/transcriptions/retry/:id` — re-queue a specific call for transcription
10. `Dockerfile` additions:
    - Default image: include `whisper.cpp` CPU build (or `faster-whisper` pip install)
    - GPU target: `FROM nvidia/cuda:12.6.0-runtime-ubuntu24.04` base with cuBLAS; install `whisper.cpp` with CUDA support
    - `docker-compose.yml`: add `gpu` profile with `deploy.resources.reservations.devices: [{driver: nvidia, count: 1, capabilities: [gpu]}]`
11. Startup: if `transcriptionEnabled` is `true`, check binary exists; if not found, log warning and set `transcriptionEnabled` to `false`
12. Unit tests: transcriber invocation args, transcript storage, search query, TRN event broadcast

**Deliverables:** Uploaded call gets transcribed within 30s (model-dependent); transcript visible on scanner; searchable by text; GPU container transcribes in real-time.

**Agents:** Go Expert (transcriber worker pool, Whisper exec, transcription API, TRN WS event), React Expert (TranscriptPanel, search transcript input, SharedCall transcript), Database Expert (transcriptions queries), Docs Expert (Dockerfile GPU target instructions), Testing Expert (unit tests), Reviewer (command injection review for Whisper invocation).

**References:** [Call Transcription](#call-transcription-local-whisper-binary) (Extended Features), [Transcript Panel](#transcript-panel-transcriptpaneltsx) (wireframe), [`transcriptions` table](#transcriptions), [Transcriptions](#transcriptions-when-transcriptionenabled-enabled) endpoints, [WebSocket Commands](#websocket-commands) (`TRN`), [Settings](#settings) (`transcriptionEnabled`, `transcriptionBinary`, `transcriptionModel`, `transcriptionLanguage`).

---

## Verification Checklist

Each phase is complete when all of the following pass:

| #   | Check                                                                                                                                                |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `go test ./...` from `backend/` — all tests green                                                                                                    |
| 2   | `pnpm test -- --run` from `frontend/` — all Vitest tests green                                                                                       |
| 3   | `pnpm playwright test` from `e2e/` — all E2E tests green                                                                                             |
| 4   | `POST /api/call-upload` with valid `X-API-Key` → 200; connected WS client receives `CAL` within 500ms                                                |
| 5   | Fresh SQLite DB → `GET /api/setup/status` → `{needsSetup: true}`                                                                                     |
| 6   | After `POST /api/setup` with `{username, password}` → admin user created; `GET /api/setup/status` → `{needsSetup: false}`; `/setup` page returns 403 |
| 7   | Admin wrong password 3× → next attempt returns 429 for 10 minutes                                                                                    |
| 8   | `PUT /api/admin/config` → all connected admin WS clients receive `CFG` event                                                                         |
| 9   | Drop Trunk Recorder file into DirWatch directory → call appears in scanner within polling delay                                                      |
| 10  | Configure a downstream → upload a call → downstream instance DB contains the call                                                                    |
| 11  | CLI `login` → `config-get` → exports valid JSON; `config-set` → imports it back                                                                      |
| 12  | `--service install` registers system service on Linux/macOS/Windows                                                                                  |
| 13  | `docker build -t openscanner .` succeeds; `docker run -p 3000:3000 openscanner` → app at `:3000`                                                     |
| 14  | After server restart, all settings from SQLite persist (spot-check `pruneDays`, custom TG labels)                                                    |
| 15  | Swagger UI reachable at `/api/docs` and renders all endpoints                                                                                        |
| 16  | `GET /api/health` returns `{status: "ok"}` — Docker HEALTHCHECK passes                                                                               |
| 17  | `PRAGMA journal_mode` returns `wal` after DB connection open                                                                                         |
| 18  | All log output is structured JSON (slog) with request IDs                                                                                            |
| 19  | Single Go binary serves frontend (no external `dist/` folder needed)                                                                                 |
| 20  | Service Worker caches app shell; repeat loads work offline (static assets only)                                                                      |
| 21  | Admin panels with 1000+ rows render smoothly (virtual scrolling)                                                                                     |
| 22  | `make dev` starts both Go (air) and Vite dev servers with hot reload                                                                                 |
| 23  | Listener user JWT cannot access `/api/admin/*` routes (returns 403)                                                                                  |
| 24  | Admin can create/disable/delete listener users; disabled user cannot log in                                                                          |
| 25  | Public access mode allows unauthenticated scanner listening; admin routes remain protected                                                           |
| 26  | Keyboard shortcuts: `Space` pauses, `S` skips, `?` opens help modal; disabled in input fields                                                        |
| 27  | Theme toggle switches between dark and light; persists in localStorage after reload                                                                  |
| 28  | Share URL (`/call/:id`) renders public player with audio + metadata when `shareableLinks` enabled; returns 404 when disabled                         |
| 29  | Bookmark a call → call appears in Bookmarks panel; bookmarked calls survive auto-pruning                                                             |
| 30  | Webhook delivery: upload call → Discord channel receives embed within 5s (when `webhooksEnabled`)                                                    |
| 31  | Push notification: subscribe to TG → upload matching call → browser notification received                                                            |
| 32  | Transcription: upload call with `transcriptionEnabled` + binary present → transcript appears within 60s; searchable via transcript text              |
| 33  | GPU Docker image builds and Whisper uses CUDA when `--gpus` flag passed                                                                              |
| 34  | Activity dashboard shows correct calls/hour sparkline and top TGs                                                                                    |
| 35  | All extended features gracefully degrade when disabled (no errors, UI elements hidden)                                                               |

---

## Scope

### Included

Everything rdio-scanner v6.6.x does:

- All 6 recorder integrations (Trunk Recorder, SDRTrunk, RTLSDR-Airband, DSDPlus, ProScan, voxcall)
- SDRTrunk HTTP API ingest (in addition to DirWatch)
- DirWatch with all recorder parsers, meta-mask, polling fallback for CIFS/NFS
- Downstream call forwarding
- API keys with per-system/TG grants
- Full TG selection (ON/OFF/PARTIAL), live feed, archive search
- Avoid (30/60/120 min), hold system/TG, patched talkgroups, AFS systems
- Unit aliases, CSV import, JSON config export/import
- Auto-populate, duplicate detection, call pruning
- Audio conversion with 4 modes (disabled / enabled / norm / loudnorm)
- CLI management commands (login, config-get/set, user-add/remove, change-password)
- System service daemon (systemd, Windows Service, launchd)
- SSL / Let's Encrypt, Docker, reverse proxy support
- Server configuration via CLI flags, environment variables, INI file
- Default groups and tags seeded on first run
- Keypad beep audio assets (Uniden/Motorola)
- PWA manifest (installable on mobile — browser-only, no native app)

**Improvements beyond rdio-scanner:**

- SQLite WAL mode for concurrent read performance
- Batch call pruning (500-row batches with yields to avoid long DB locks)
- Additional call index (`system_id, talkgroup_id`) for joins and lookups
- Frontend embedded into Go binary via `go:embed` (single-file deployment)
- Structured logging with `log/slog` (JSON output, structured key-value pairs)
- Graceful shutdown (`context.WithCancel` + `srv.Shutdown`) instead of `os.Exit`
- Bounded FFmpeg worker pool (`runtime.NumCPU()` workers, channel queue)
- Health check endpoint (`GET /api/health`) for Docker HEALTHCHECK and monitoring
- Binary WebSocket audio frames (audio bytes sent as binary frame after CAL JSON)
- permessage-deflate WebSocket compression via coder/websocket
- Debounced `LSC` listener-count broadcasts (max once per 3 seconds)
- Virtual scrolling via `@tanstack/react-virtual` for large admin panel lists
- Audio preloading (preload next queued call for gapless playback)
- Service Worker PWA (app-shell caching for instant repeat loads)
- CSRF protection (JWT-in-header pattern + SameSite=Strict)
- Per-API-key rate limiting on call upload endpoint
- Hot-reload dev mode (`air` + Vite proxy via single `make dev`)
- Request ID middleware (UUID v4 per request in all logs and response headers)
- RBAC with user accounts (admin and listener roles with per-user system/TG grants)
- Shareable call links with OpenGraph tags for link previews
- Keyboard shortcuts for power-user scanner operation
- Dark / light theme toggle (DaisyUI dual theme)
- OpenAPI 3.1 / Swagger UI at `/api/docs`
- Activity dashboard with calls/hour sparkline and top TGs
- Call bookmarking (per-user for authenticated, per-session for public)
- Browser push notifications (Web Push / VAPID) for specific TGs
- Webhook integration (generic JSON + HMAC, Discord embeds) for call events
- Call transcription via local Whisper binary (CPU or GPU) with full-text search

### Excluded

- Native iOS/Android apps (browser PWA covers mobile)
- MySQL/MariaDB support (SQLite only)
- Fine-grained permissions beyond admin/listener (two roles is sufficient)
- Cloud-hosted STT APIs (transcription is local binary only — no data leaves the server)
