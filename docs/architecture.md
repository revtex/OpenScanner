# OpenScanner — Architecture

> **Implementation status:** Phases 1–12 are complete (Foundation through CLI/Daemon/SSL/Docker/Deployment). Phases 13+ remain planned and are documented for future work. Packages marked _(stub)_ below exist as empty package declarations and will be implemented in later phases.

## Overview

OpenScanner is a modern web-based radio call manager inspired by rdio-scanner. It uses a Go backend (Gin + SQLite) with a React frontend (TypeScript + Tailwind CSS 4.2.2 + DaisyUI 5.5.19), connected via WebSocket for real-time call streaming.

**Frontend styling:** Tailwind CSS 4 (4.2.2) uses CSS-first configuration — `@import "tailwindcss"` and `@plugin "daisyui"` directives in `index.css`, with the `@tailwindcss/vite` Vite plugin. There is no `tailwind.config.ts` or `postcss.config.js`. Custom themes (`openscanner-dark`, `openscanner-light`) are defined via `@plugin "daisyui/theme"` blocks in `index.css`. Icons are provided by `lucide-react`.

## System Diagram

The diagram below shows the full planned architecture. Solid lines and green fills indicate implemented components; dashed lines and grey fills indicate stubs not yet implemented.

```mermaid
graph TD
    Recorder["Radio Recorder<br/>(Trunk Recorder / SDRTrunk)"] -->|POST /api/call-upload| MW
    DirWatch["DirWatch Service<br/>(fsnotify)"] -->|ingest files| Processor

    MW["Middleware<br/>(JWT, API Key, Rate Limit)"] -->|validated request| API["API Handlers<br/>(Gin)"]
    API -->|processor.Store| Processor["Audio Processor<br/>(FFmpeg Worker Pool)"]
    Processor --> FS[("Filesystem<br/>audio files")]
    Processor -.-> Transcriber["Whisper Transcriber<br/>(local binary)"]
    API -->|sqlc queries| DB[(SQLite<br/>WAL mode)]
    Transcriber -.-> DB

    Pruner["Call Pruner<br/>(hourly background)"] -->|delete old records| DB
    Pruner -->|delete old audio| FS

    Seed["Seed<br/>(runs at startup)"] -->|default data| DB

    API --> Hub["WebSocket Hub"]
    Hub -->|CAL / CFG / LSC| Listeners["Browser Clients"]
    API --> Downstream["Downstream Pusher"]
    Downstream -->|POST /api/call-upload| RemoteInstance["Remote OpenScanner"]
    API -.-> Webhooks["Webhook Delivery"]
    Webhooks -.->|POST| External["Discord / Generic"]
    API -.-> Push["Push Notifications"]
    Push -.->|Web Push| Browser["Browser Push"]

    style MW fill:#b5e6b5,stroke:#333,color:#000
    style API fill:#b5e6b5,stroke:#333,color:#000
    style Processor fill:#b5e6b5,stroke:#333,color:#000
    style FS fill:#b5e6b5,stroke:#333,color:#000
    style DB fill:#b5e6b5,stroke:#333,color:#000
    style Seed fill:#b5e6b5,stroke:#333,color:#000
    style Pruner fill:#b5e6b5,stroke:#333,color:#000
    style Hub fill:#b5e6b5,stroke:#333,color:#000
    style Listeners fill:#b5e6b5,stroke:#333,color:#000
    style Transcriber fill:#bbb,stroke:#555,color:#000,stroke-dasharray: 5 5
    style DirWatch fill:#b5e6b5,stroke:#333,color:#000
    style Downstream fill:#b5e6b5,stroke:#333,color:#000
    style Webhooks fill:#bbb,stroke:#555,color:#000,stroke-dasharray: 5 5
    style Push fill:#bbb,stroke:#555,color:#000,stroke-dasharray: 5 5
```

## Components

### Implemented

- **backend/cmd/server** — Application entry point; loads config, opens DB, runs migrations, seeds defaults, starts Gin HTTP server with timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`); graceful shutdown via `signal.NotifyContext` + error channel
- **backend/internal/api** — Gin route handlers: health check (`GET /api/health`), first-run setup (`GET /api/setup/status`, `POST /api/setup`), auth (`POST /api/auth/login`, `POST /api/auth/logout`, `PUT /api/auth/password`, `GET /api/auth/me`), bookmarks (`GET /api/bookmarks`, `POST /api/bookmarks` — JWT required), admin CRUD (full CRUD for users, systems, talkgroups, units, groups, tags, apikeys, dirwatches, downstreams, webhooks), admin config (`GET/PUT /api/admin/config`), admin logs (`GET /api/admin/logs`), CSV import (`POST /api/admin/import/talkgroups`, `POST /api/admin/import/units`), JSON config export/import (`GET /api/admin/export/config`, `POST /api/admin/import/config`), system/apikey reorder (`PUT /api/admin/systems/reorder`, `PUT /api/admin/apikeys/reorder`), directory listing (`GET /api/admin/fs/directories`), missing audio tools (`GET /api/admin/tools/audio-missing`, `POST /api/admin/tools/audio-missing/cleanup`); response DTOs use pointer fields (`*string`/`*int64`) for proper JSON serialization of nullable values; API keys are stored hashed (SHA-256) and returned as truncated fingerprints
- **backend/internal/auth** — JWT HS256 (32-byte random secret, 24h expiry, UUID v4 JTI); bcrypt cost 12; `TokenTracker` with max-5 tokens per user (oldest evicted); `RateLimiter` (3 failures → 10-min lockout per IP); timing-safe login with `DummyHash`
- **backend/internal/config** — Server startup configuration (CLI flags, env vars, optional INI file); precedence: CLI > env > INI > defaults
- **backend/internal/middleware** — Gin middleware: `RequestID` (UUID v4), `Logger` (structured slog), `CORS` (same-origin with localhost dev exception), `JWTAuth` (validates token + checks revocation), `OptionalJWTAuth` (extracts JWT if present but allows unauthenticated access for public endpoints), `RequireAdmin` (role-based 403), `APIKeyAuth` (header or query param), `RateLimit` (429 on lockout), `MaxBodySize` (request body size limiter)
- **backend/internal/seed** — First-run database seeding: 1 `app_state` row, 31 settings, 8 groups (Air/Common/EMS/Fire/Interop/Law/Public Works/Unknown), 21 tags (ATC, Corrections, Emergency Ops, EMS Dispatch/Tac/Talk, Fire Dispatch/Tac/Talk, Hospital, Interop, Law Dispatch/Tac/Talk, Public Works, Schools, Security, Service, Transportation, Untagged, Utilities); all idempotent (`INSERT OR IGNORE`) in a single transaction
- **backend/internal/db** — SQLite WAL mode, embedded migrations (23 migration files, including 3 separate 019-prefix migrations: `019_add_call_metadata.sql` adds `site`/`channel`/`decoder` columns to calls, `019_add_password_need_change.sql` adds `password_need_change` to users, `019_drop_accesses.sql` drops the legacy accesses table; `020_create_shared_links.sql` creates shareable call links table; `021_create_shared_links.sql` creates shareable call links table; `021_add_call_error_spike.sql` adds `error_count`/`spike_count` to calls), `SetMaxOpenConns(1)`; sqlc-generated type-safe query layer

- **backend/internal/audio** — FFmpeg audio conversion pipeline: `Processor.Store` writes uploaded multipart file to `{audioDir}/{YYYY}/{MM}/{DD}/`, submits a `ConversionJob` to the bounded `WorkerPool` (`runtime.NumCPU()` goroutines), waits for completion, then returns the relative `.m4a` path; `IsDuplicate` queries the last call per system+talkgroup within the configured time window; `PruneLoop` runs on a 1-hour ticker deleting old calls and audio files in 500-row batches
- **backend/internal/api/calls.go** — `PostCallUpload` handler: validates API key (via `APIKeyAuth` middleware), applies per-API-key sliding-window rate limiter (default 60 req/min), parses multipart fields (including optional `errorCount`/`spikeCount` for P25 signal quality metrics), resolves or auto-populates system+talkgroup, runs duplicate check, stores audio via `Processor.Store`, inserts call DB record, broadcasts `CAL` + binary audio to WS hub, returns `{"id": <callID>}`; also registered as `POST /api/trunk-recorder-call-upload`
- **backend/internal/api/calls.go** — `GetCalls` handler: paginated call archive search via `GET /api/calls`; supports filtering by `system_id`, `talkgroup_id`, `date_from`, `date_to` with `page`/`limit` pagination and `sort` direction (asc/desc); uses `OptionalJWTAuth` middleware (returns bookmark status when JWT is present); returns `{"calls": [...], "total": N}`
- **backend/internal/api/calls.go** — `GetCallAudio` handler: streams call audio files via `GET /api/calls/:id/audio`; uses `OptionalJWTAuth` middleware; requires valid JWT or `publicAccess=true` (returns 401 otherwise); reads audio from the recordings directory with path traversal protection
- **backend/internal/api/share.go** — Shareable call link handlers: `POST /api/calls/:id/share` (create share, returns UUID token + URL), `GET /api/calls/:id/share` (share status, JWT required), `DELETE /api/calls/:id/share` (remove share), `GET /api/shared/:token` (public metadata), `GET /api/shared/:token/audio` (public audio stream); shares stored in `shared_links` table with UUID tokens
- **backend/internal/api/shared_links.go** — Admin shared-links management: `GET /api/admin/shared-links` (list all), `DELETE /api/admin/shared-links/:id` (remove); shared calls are excluded from automatic pruning

- **backend/internal/ws** — Real-time WebSocket hub for call streaming and client management:
  - **hub.go** — `Hub` struct runs a single-goroutine event loop processing `register`, `unregister`, and `broadcast` channels; non-blocking sends (slow clients dropped); `BroadcastCAL()` sends text + binary frames atomically per client (mutex-protected); `debounceLSC()` limits listener-count broadcasts to max 1 per 3 seconds via `time.AfterFunc`; graceful shutdown via context cancellation + `closeAll()`
  - **client.go** — `Client` struct with `readPump`/`writePump` goroutines; `HandleListenerWS` supports two auth flows: public access (no auth needed) or listener JWT sent as first message; enforces `maxClients` and per-user `limit`; `HandleAdminWS` validates admin JWT via `?token=` query param; `CanReceive(systemID, talkgroupID)` filters per-client grants
  - **messages.go** — Command constants (`CAL`, `CFG`, `VER`, `LSC`, `XPR`, `MAX`, `LFM`, `LCL`, `TRN`) with typed builder functions; `ParseCommand` extracts command + payload from JSON array messages
  - **Routes:** `GET /ws` (listener), `GET /api/admin/ws` (admin); registered via `gin.WrapF` in `api/routes.go`
  - **Compression:** permessage-deflate via `websocket.CompressionContextTakeover`

- **backend/internal/downstream** — Call forwarding service that pushes accepted calls to remote OpenScanner instances:
  - **pusher.go** — `Service` struct with fan-out pattern: one goroutine per active (non-disabled) downstream config, each with a buffered channel (1000 events); `Start` loads downstream configs from DB and spawns goroutines; `Reload` stops all pushers and restarts from DB (triggered by admin CRUD create/update/delete); `Stop` cancels context and drains goroutines after HTTP server shutdown
  - **Grant filtering:** `systems_json` column on each downstream controls which calls are forwarded — only calls matching the downstream's system/TG grants are enqueued
  - **Multipart POST:** Each call is re-posted as `multipart/form-data` to the remote instance's `/api/call-upload` endpoint with `X-API-Key` header authentication; audio file is read from the local filesystem
  - **Retry with backoff:** Exponential backoff on HTTP failure: 1s → 2s → 4s → 8s → 30s cap, max 5 retries per event, with random jitter to avoid thundering herd
  - **Security:** HTTP client configured with `CheckRedirect` returning error (SSRF protection — prevents following redirects to internal services); audio file paths validated against path traversal (`../`)
  - **Graceful shutdown:** `dsService.Stop()` is called after the HTTP server has completed shutdown, ensuring in-flight pushes complete before exit

- **backend/internal/dirwatch** — Directory watching service for automatic call ingest from local recorder output directories:
  - **watcher.go** — `Service` struct managing one goroutine per active dirwatch config; `Start` loads configs from DB, `Reload` stops all watchers and restarts fresh (called by admin CRUD after config changes); `runWithFsnotify` uses kernel inotify/kqueue `Create` events; `runWithPolling` scans on a configurable ticker (≥500 ms floor, suitable for CIFS/NFS mounts); `handleFile` enforces path traversal checks and extension filtering before dispatching to the parser; `ingestCall` mirrors the HTTP upload pipeline: system/TG resolution (with `autoPopulate`), duplicate check, `Processor.StoreFile`, DB insert, WS `CAL` broadcast, optional source-file deletion (`delete_after=1`)
  - **parsers.go** — `ParsedCall` struct + one `ParseFunc` per recorder type: `trunk-recorder` (JSON sidecar + audio file pair), `sdrtrunk` (`<sysID>_<tgID>_<ts>.<ext>` filename pattern), `rtlsdr-airband` (audio file, IDs from dirwatch config), `dsdplus` (audio file, IDs from dirwatch config), `proscan` (audio file, IDs from dirwatch config), `voxcall` (audio file, IDs from dirwatch config); unrecognised types fall back to `parseGeneric`
  - **mask.go** — `MaskTokens` struct + `ExpandMask`/`TokensFromCall`: expands `#DATE`, `#TIME`, `#ZTIME`, `#GROUP`, `#SYSLBL`, `#TAG`, `#TGAFS`, `#UNIT`, `#TGLBL`, `#TGHZ`, `#TGKHZ`, `#TGMHZ`, `#TGID` tokens in directory mask strings

### Stubs (package declaration only — not yet implemented)

- **backend/internal/audio/transcriber.go** — Whisper transcription worker pool
- **backend/internal/notify** — Web Push notification delivery

### Frontend — Scanner UI (Phase 9)

#### State Management

- **frontend/src/app/store.ts** — Redux store combining `scannerSlice`, `authSlice`, and RTK Query `api` reducers
- **frontend/src/app/slices/scannerSlice.ts** — Full scanner state reducers for playback control (`togglePause`, `setPaused`, `toggleLive`), active call/history tracking (`setCurrentCall`, `clearCurrentCall`), hold/avoid logic, config updates, TG selection (`toggleTG`, `setAllTGs`, `setTGsBySystem`), and transcript updates
- **frontend/src/app/slices/authSlice.ts** — Auth state: `setCredentials` (JWT + user), `clearCredentials`, `setSetupStatus`; token stored in `localStorage` (key `os_auth`) for persistence across browser tabs/windows
- **frontend/src/app/slices/callsSlice.ts** — Search filter state: system, talkgroup, group, tag, date range, sort direction, bookmarked-only; drives SearchPanel query params
- **frontend/src/app/slices/shareSlice.ts** — RTK Query mutation for creating shareable call links
- **frontend/src/app/api.ts** — RTK Query base API with `getSetupStatus`, `postSetup`, `postLogin` endpoints; `getCalls` query for paginated call archive search; base query wraps `fetchBaseQuery` with a global 401 interceptor that dispatches `clearCredentials()` on any unauthorized response

#### CSS & Theming

- **frontend/src/index.css** — CSS-first Tailwind 4 config:
  - `@import "tailwindcss"` — loads Tailwind base/components/utilities
  - `@plugin "daisyui" { themes: false; }` — loads DaisyUI with built-in themes disabled
  - `@plugin "daisyui/theme"` blocks define two custom themes: `openscanner-dark` (default, green primary `#00e676`, dark base) and `openscanner-light` (green primary `#2e7d32`, light base)
  - `@utility` directives for `led-branding`, `led-indicator`, `history-row`
  - `.lcd-display` component class — Nokia-style LCD look: green background `#bbc8c0`, dark foreground `#1e2a24`, 3px border, inset bevel shadow, repeating scanline `::after` overlay
  - `.brightness-slider` — 100px inline slider styled to match LCD aesthetic
  - No `tailwind.config.ts` or `postcss.config.js` — Tailwind 4 uses CSS-first configuration exclusively

#### WebSocket Client

- **frontend/src/services/wsClient.ts** — Singleton WebSocket client connecting to `/ws`:
  - Auto-reconnect with exponential backoff (1 s → 30 s cap) plus random jitter
  - Handles text commands (`CAL`, `CFG`, `VER`, `LSC`, `XPR`, `MAX`, `TRN`) and binary audio frames
  - Runtime payload validation before dispatching to Redux
  - Supports two auth modes: public access (no auth needed) or listener JWT; `XPR` handler clears credentials before disconnecting

#### Audio Player

- **frontend/src/services/audioPlayer.ts** — Singleton audio player:
  - `HTMLAudioElement` for playback with Web Audio API `GainNode` for volume control
  - Bounded call queue (max 50) with preloading of the next queued call
  - Download support and `clearQueue` for memory leak prevention

- **frontend/src/services/beepPlayer.ts** — Scanner keypad beep generator:
  - Uses Web Audio API `OscillatorNode` to synthesize beep tones (no WAV files)
  - Two styles: `uniden` (short high-pitched square wave) and `whistler` (two-tone sine chirp)
  - Controlled by `keypadBeeps` setting (`uniden`, `whistler`, or `disabled`)

#### Hooks

- **useWebSocket** — WS lifecycle tied to auth state (connect/disconnect on login/logout)
- **useAudioPlayer** — Wires audio player callbacks (play, end, error) to Redux actions
- **useTheme** — Dark/light theme toggle with `localStorage` persistence (switches between `openscanner-dark` and `openscanner-light` DaisyUI themes)
- **useScanner** — Composite hook combining WebSocket, audio player, and scanner state
- **useKeyboardShortcuts** — Global keyboard shortcuts (Space, S, R, D, L, B, etc.)

#### Components

```
Scanner.tsx (lazy-loaded page)
├── LEDPanel          — Branding text + theme toggle + colored LED (green=live, orange=paused, pulse=idle)
├── DisplayPanel      — Nokia-style LCD display (.lcd-display CSS class: green background #bbc8c0, scanlines, bevel shadow)
│   │                   7-row monospace display with clock
│   ├── BookmarkButton   — Star toggle on current call (wired to backend bookmarks API)
│   ├── Share button     — Share2 icon, copies call permalink to clipboard
│   ├── Brightness slider — Sun icon toggle, inline horizontal slider (20–120%), persisted to localStorage, default 50%
│   ├── TranscriptPanel  — Collapsible transcript display
│   └── HistoryPanel     — Last 5 calls (TG name + time, system · UID · TGID · freq MHz · E/S when non-zero)
├── ControlToolbar    — Two-row icon toolbar
│   ├── Row 1: play/pause, skip, replay, volume slider (range range-xs range-primary), download, bookmark
│   └── Row 2: LIVE toggle, HOLD (system/TG), AVOID, SELECT, SEARCH, overflow menu (fullscreen, keyboard shortcuts)
├── SelectTGPanel     — Slide-out panel for talkgroup selection (tri-state group toggles, per-system TG toggles, avoid countdown; state persisted to localStorage)
└── SearchPanel       — Slide-out panel for call archive search (RTK Query paginated results via GET /api/calls, filters by system/TG/group/tag/date, per-row play/download/bookmark buttons, E/S display)
```

#### Pages

- **Scanner.tsx** — Main layout assembling all scanner components (lazy-loaded)
- **Login.tsx** — Auth flow with password-change enforcement on first login
- **Setup.tsx** — First-run wizard (`POST /api/setup` → redirect to login)

#### PWA

- **frontend/public/manifest.json** — PWA manifest (`display: standalone`, dark theme color)
- **frontend/sw.ts** — Service Worker: network-first for API requests, cache-first for static assets (HTML, JS, CSS, fonts)

#### Tests

- 145 unit tests across 10 test files: `scannerSlice.test.ts` (29), `LEDPanel.test.tsx` (11), `ControlToolbar.test.tsx` (24), `Login.test.tsx` (7), `Setup.test.tsx` (4), `AdminLayout.test.tsx` (4), `api.test.ts` (1), `callsSlice.test.ts` (31), `SearchPanel.test.tsx` (14), `SelectTGPanel.test.tsx` (20) (Vitest + React Testing Library)

### Frontend — Admin Dashboard (Phase 11)

#### Routing & Auth Guard

- **frontend/src/main.tsx** — `/admin/*` route with `React.lazy` loading; nested routes for each panel
- **frontend/src/components/admin/AdminLayout.tsx** — Auth guard (redirects to `/login` if no JWT or non-admin role); responsive sidebar using DaisyUI drawer (hamburger on mobile, icons on `md`, icons + labels on `lg`); 9 nav items + Scanner link (Home icon) + Sign Out button; each admin panel includes a help description paragraph below its heading
- **frontend/src/pages/Admin.tsx** — Wrapper rendering `AdminLayout` with `<Outlet>`

#### State Management

- **frontend/src/app/slices/adminSlice.ts** — RTK Query endpoints for all admin CRUD operations (Users, Systems, Talkgroups, Units, Groups, Tags, ApiKeys, Dirwatches, Downstreams, Webhooks, Config, Logs, Import/Export, Password, server directory listing, missing audio tools); tag-based cache invalidation
- **frontend/src/app/slices/authSlice.ts** — Added `selectToken`, `selectRole`, `selectUsername` selectors
- **frontend/src/app/api.ts** — Extended `tagTypes` for admin cache invalidation
- **frontend/src/types/index.ts** — 15 admin types (`AdminUser`, `AdminSystem`, `AdminTalkgroup`, `AdminUnit`, `AdminGroup`, `AdminTag`, `AdminApiKey`, `AdminApiKeyCreateResponse`, `AdminDirwatch`, `AdminDownstream`, `AdminWebhook`, `AdminSetting`, `AdminLog`, `ChangePasswordRequest`, `CreateUserPayload`, `UpdateUserPayload`)

#### Admin Panels

| Panel              | Key Features                                                                                               |
| ------------------ | ---------------------------------------------------------------------------------------------------------- |
| `UsersPanel`       | CRUD table, role badges, disabled toggle, expiration, create/edit modal                                    |
| `SystemsPanel`     | Drag-to-reorder (`@dnd-kit`), expandable rows with nested talkgroups (`@tanstack/react-virtual`) and units |
| `GroupsTagsPanel`  | Two side-by-side CRUD tables                                                                               |
| `OptionsPanel`     | Settings form with 6 sections, boolean toggles, conditional transcription fields                           |
| `ApiKeysPanel`     | Drag-to-reorder, copy-to-clipboard, UUID generation                                                        |
| `DirWatchPanel`    | Directory watches CRUD with type dropdown                                                                  |
| `DownstreamsPanel` | Downstream servers CRUD                                                                                    |
| `LogsPanel`        | Virtualized log viewer (`@tanstack/react-virtual`) with date/level filters                                 |
| `ToolsPanel`       | CSV import (`talkgroups`/`units`), JSON config export/import, password change, missing-audio audit/cleanup |
| `WebhooksPanel`    | Webhooks CRUD with type badges                                                                             |

#### Dependencies

- `@dnd-kit/core`, `@dnd-kit/sortable`, `@dnd-kit/utilities` — Drag-to-reorder in SystemsPanel and ApiKeysPanel
- `@tanstack/react-virtual` — Virtualized scrolling in SystemsPanel (nested talkgroups) and LogsPanel
- `lucide-react` — Icon library used across all scanner and admin components

#### Navigation Guard

- **frontend/src/components/admin/NavigationGuardContext.tsx** — Context provider that wraps the admin layout; warns users about unsaved changes before navigating away from a panel with uncommitted edits

#### Tests

- 15 admin-related tests: `Login.test.tsx` (7), `Setup.test.tsx` (4), `AdminLayout.test.tsx` (4)
- Total: 145 tests (10 test files)

### Frontend — Stubs (not yet implemented)

- No frontend stub files are currently tracked in the repository for phases 1–12.
