# OpenScanner

**OpenScanner** is a web-based radio call manager for monitoring, searching, and sharing scanner traffic in real time. It ingests calls from popular radio recorders, processes and stores audio, streams live feeds to browser clients, and provides a full admin dashboard for configuration and operations.

OpenScanner is a modern reimplementation of [rdio-scanner](https://github.com/chuot/rdio-scanner), built from the ground up as a single Go binary with an embedded React frontend. It maintains backward compatibility with rdio-scanner's upload API, so existing recorder configurations (Trunk-Recorder's `rdioscanner_uploader`, SDRTrunk's Rdio Scanner streaming target) work without changes.

---

## Features

### Scanner Interface

- **Live feed** — real-time call streaming over WebSocket with playback controls (play/pause, skip, replay)
- **Hold & avoid** — lock to a system or talkgroup; temporarily avoid talkgroups for 5/15/30 min or indefinitely
- **Talkgroup selection** — search and multi-select talkgroups by system, group, or tag; selection persisted per user
- **Call archive** — search historical calls by system, talkgroup, group, tag, date range, transcript text, or bookmark state
- **Bookmarks** — bookmark calls for later, filter archive to bookmarked only
- **Call sharing** — generate public share links for individual calls with configurable expiry
- **Live transcripts** — view call transcriptions in the live player with speaker diarization segments (requires whisper sidecar)
- **LED indicators** — live/recording status, listener count, now-playing info
- **Dark/light theme** — toggle between themes; preference saved in browser
- **Responsive** — mobile-first layout with drawer navigation, touch-friendly controls, and virtual scrolling

### Call Ingest

- **HTTP upload** — `POST /api/call-upload` with API key auth; backward-compatible alias at `/api/trunk-recorder-call-upload`
- **Directory monitoring** — watch local directories for new recordings with configurable polling, masks, and auto-delete
- **Supported recorders** — Trunk-Recorder, SDRTrunk, DSDPlus, RTLSDR-Airband, ProScan, and generic mask-based sources
- **Auto-populate** — automatically create systems, talkgroups, groups, tags, and units from incoming call metadata
- **Metadata extraction** — Trunk-Recorder JSON sidecars, MP3 ID3 tags, filename masks with tokens (`#SYS`, `#TG`, `#DATE`, `#UNIT`, etc.)
- **Duplicate detection** — configurable time-window dedup to reject redundant uploads
- **Audio processing** — FFmpeg conversion with four modes (disabled, enabled, normalize, loudnorm) and multiple encoding presets (MP3, AAC-LC, HE-AAC)
- **Auto-pruning** — automatically delete calls older than a configurable number of days

### Administration

- **Dashboard** — calls today/week/total, active listeners, uptime, 24-hour activity chart, top talkgroups
- **User management** — create/edit/disable users with admin or listener roles, account expiration, session limits, per-user talkgroup selection, and password-change enforcement
- **Radio data** — CRUD for systems, talkgroups, units, groups, and tags with CSV import/export and RadioReference enrichment
- **API keys** — create/rotate upload keys with per-key system grants and per-key rate limits
- **Directory monitors** — configure ingest paths with type-specific settings, polling vs. filesystem watch, and a server-side directory browser
- **Downstreams** — forward calls to remote OpenScanner instances with per-downstream system grants (experimental, untested)
- **Shared links** — view and manage all active share links with expiry tracking
- **Transcription** — manage whisper models (download, select, delete), configure language and diarization, monitor connection status and stats
- **Options** — grouped settings for general config, scanner behavior, call processing, display, and sharing
- **Tools** — CSV import/export for talkgroups and units, JSON config export/import, RadioReference preview
- **Logs** — query server logs by level, date range, and text search with auto-refresh and runtime log level control
- **Config import/export** — full JSON backup and restore of all configuration data

### Transcription

OpenScanner integrates with [go-whisper](https://github.com/mutablelogic/go-whisper) (a whisper.cpp HTTP sidecar) for automatic call transcription. Features include:

- **Model management** — download, select, and delete Whisper models directly from the admin panel (11 models available from tiny to large-v3-turbo)
- **Live transcript display** — show transcription text in the live scanner player as calls come in
- **Searchable transcripts** — find calls by transcript text from the search page
- **Speaker diarization** — identify who is talking using tinydiarize models (`ggml-small.en-tdrz`)
- **Language support** — 15 languages plus auto-detect
- **GPU acceleration** — CPU, NVIDIA CUDA, Intel iGPU, or AMD ROCm (GPU highly recommended; 6 GB+ VRAM suggested)

### Deployment

- **Single binary** — no external database; SQLite embedded with WAL mode
- **Guided setup** — `openscanner setup --interactive` creates directories, writes config, installs a system service
- **Cross-platform** — Linux (systemd/SysV/OpenRC), macOS (launchd), Windows (SCM) with auto-detected service management
- **Docker** — pre-built Alpine image with FFmpeg included
- **JSON config** — persist settings with `--config-save`; load from file, env vars, or CLI flags
- **Service management** — `setup`, `upgrade`, `config validate`, `service doctor` commands
- **TLS** — certificate files with HTTP auto-redirect to HTTPS; experimental Let's Encrypt auto-cert
- **Reverse proxy** — tested with Nginx and Caddy; WebSocket-aware proxy configs in the docs

### Security

- JWT authentication with refresh token rotation and configurable expiry
- bcrypt password hashing (cost ≥ 12)
- Role-based access control (admin / listener)
- API key auth for uploads (`X-API-Key` header or `?key=` query param)
- Per-IP rate limiting on login and shared link access
- Per-user rate limiting on share creation
- Per-API-key sliding-window rate limiting on uploads
- WebSocket session re-validation with forced disconnect on user disable/delete
- Shared link expiry (configurable in days, enforced on access)
- Public access mode for unauthenticated listening (admin routes always protected)
- Audio path sanitization, no shell injection, no secrets in logs
- Optional secrets-at-rest encryption (AES-256-GCM) for the JWT signing secret and downstream API keys
- Optional TLS with certificate/key files; experimental Let's Encrypt auto-cert (untested)
- Outbound HTTP (transcription, downstreams) goes through a hardened client with redirects disabled, timeouts enforced, and response bodies capped. LAN/loopback destinations are permitted by default (homelab-friendly); set `OPENSCANNER_BLOCK_INTERNAL_HTTP=1` to reject private-network targets

---

## What's New vs. rdio-scanner

OpenScanner is a complete rewrite, not a fork. Everything below is new or significantly improved:

| Feature                      | rdio-scanner  | OpenScanner                                                                                           |
| ---------------------------- | ------------- | ----------------------------------------------------------------------------------------------------- |
| **Automatic transcription**  | Not available | Built-in via go-whisper with GPU support, model management, live display, and search                  |
| **Auto-populate**            | Systems only  | Systems, talkgroups, groups, tags, and units — all created from incoming metadata                     |
| **Call sharing**             | Not available | Generate public share links with configurable expiry                                                  |
| **Bookmarks**                | Not available | Bookmark calls and filter the archive to bookmarked only                                              |
| **Talkgroup selection**      | Basic         | Per-user multi-select by system, group, or tag; persisted server-side                                 |
| **Audio encoding presets**   | Single format | 8 presets across MP3, AAC-LC, and HE-AAC at multiple bitrates                                         |
| **User management & RBAC**   | Access codes  | Named user accounts with admin/listener roles, per-user system grants, expiration, and session limits |
| **Per-key rate limits**      | Not available | Global and per-API-key call rate limiting with sliding window                                         |
| **Downstream forwarding**    | Basic         | Forward calls to other OpenScanner instances with system grants (experimental, untested)              |
| **Service management**       | Manual        | Guided `setup`, `upgrade`, `config validate`, `service doctor` commands                               |
| **Auto-pruning**             | Basic         | Configurable retention with automatic deletion of calls older than N days                             |
| **Let's Encrypt**            | Not available | Automatic certificate provisioning with `--ssl-auto-cert` (experimental, untested)                    |
| **Admin WebSocket**          | REST polling  | Real-time admin operations over WebSocket — instant updates                                           |
| **CSV import/export**        | Limited       | Full CSV import/export for talkgroups and units with duplicate handling                               |
| **JSON config backup**       | Not available | Export and import full server configuration                                                           |
| **Log viewer**               | Basic         | Query logs by level, date, text with auto-refresh and runtime level control                           |
| **Dark/light theme**         | Dark only     | Toggle between themes                                                                                 |
| **RadioReference import**    | Not available | Preview and apply talkgroup metadata from RadioReference directly in admin                            |
| **Secrets encryption**       | Not available | Optional AES-256-GCM encryption for the JWT signing secret and downstream API keys |

---

## Quick Start

### Docker Compose

```bash
docker compose up -d
```

Open `http://localhost:3022` and complete the first-run setup to create your admin account.

### Build from Source

```bash
make build
./build/openscanner --listen 0.0.0.0:3022 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

### Configuration

OpenScanner is configured via CLI flags, environment variables, or a JSON config file:

| Flag               | Env Var                       | Description                                       |
| ------------------ | ----------------------------- | ------------------------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`          | Listen address (default `:3022`)                  |
| `--db-file`        | `OPENSCANNER_DB_FILE`         | SQLite database path                              |
| `--recordings-dir` | `OPENSCANNER_RECORDINGS_DIR`  | Audio file storage directory                      |
| `--ssl-listen`     | `OPENSCANNER_SSL_LISTEN`      | HTTPS listen address                              |
| `--ssl-cert`       | `OPENSCANNER_SSL_CERT`        | TLS certificate file (PEM)                        |
| `--ssl-key`        | `OPENSCANNER_SSL_KEY`         | TLS private key file (PEM)                        |
| `--ssl-auto-cert`  | `OPENSCANNER_SSL_AUTO_CERT`   | Domain for Let's Encrypt auto-cert (experimental) |
| `--encryption-key` | `OPENSCANNER_ENCRYPTION_KEY`  | AES-256 key for encrypting secrets at rest        |
| `--timezone`       | `OPENSCANNER_TIMEZONE` / `TZ` | IANA timezone for recorder timestamps             |

All application settings (audio processing, scanner behavior, sharing, etc.) are managed through the admin dashboard and stored in the database. See the [Deployment Guide](docs/deployment-guide.md) for the full configuration reference.

---

## Recorder Compatibility

OpenScanner works with any radio recorder that produces per-call audio files. It accepts calls via HTTP upload (API) or by watching a local directory (DirMonitor).

| Recorder                                                       | API | DirMonitor |
| -------------------------------------------------------------- | :-: | :--------: |
| [Trunk-Recorder](https://github.com/robotastic/trunk-recorder) |  ✔  |     ✔      |
| [SDRTrunk](https://github.com/DSheirer/sdrtrunk)               |  ✔  |     ✔      |
| [RTLSDR-Airband](https://github.com/szpajder/RTLSDR-Airband)   |     |     ✔      |
| [DSDPlus Fast Lane](https://www.dsdplus.com/)                  |     |     ✔      |
| [ProScan](https://www.proscan.org/)                            |     |     ✔      |
| [voxcall](https://github.com/aaknitt/voxcall)                  |  ✔  |            |

- **API upload** — `POST /api/call-upload` (or `/api/trunk-recorder-call-upload`) with an API key. Compatible with rdio-scanner's upload protocol — existing recorder configs work with just a URL change.
- **DirMonitor** — watch a local directory for new recordings. Supports per-type parsers (Trunk-Recorder JSON sidecars, SDRTrunk ID3 tags, DSDPlus date folders, filename masks with tokens like `#SYS`, `#TG`, `#DATE`, etc.).

See [docs/recorder-guide.md](docs/recorder-guide.md) for detailed setup steps.

---

## rdio-scanner Compatibility

OpenScanner is designed as a drop-in replacement for [rdio-scanner](https://github.com/chuot/rdio-scanner). Key compatibility points:

- The upload endpoint `/api/trunk-recorder-call-upload` accepts the same multipart form fields
- API key authentication works via `X-API-Key` header or `?key=` query parameter
- SDRTrunk's partial-data key verification probe returns the same plain-text responses
- Error messages match rdio-scanner's format for recorder-side log compatibility
- Existing recorder configurations can be pointed at OpenScanner with only a URL change

---

## API & WebSocket

- **REST API** — `/api/*` with JSON request/response; Swagger UI available at `/api/admin/docs` for authenticated admins
- **Listener WebSocket** — `/ws` for real-time call streaming, configuration updates, and listener count
- **Admin WebSocket** — `/api/admin/ws` for live admin dashboard operations (CRUD, events, settings)
- **Health check** — `GET /api/health` returns server status and version

---

## Documentation

| Document                                             | Description                           |
| ---------------------------------------------------- | ------------------------------------- |
| [docs/admin-guide.md](docs/admin-guide.md)           | Admin dashboard usage guide           |
| [docs/deployment-guide.md](docs/deployment-guide.md) | Build, run, and deployment operations |
| [docs/recorder-guide.md](docs/recorder-guide.md)     | Recorder setup guide                  |

---

## Development

```bash
make dev     # Go hot-reload (air) + Vite dev server
make build   # Production build (single binary)
make test    # Run all tests
make lint    # Lint Go + TypeScript
```

## Tech Stack

- **Backend:** Go, Gin, coder/websocket, SQLite (modernc, WAL mode), sqlc, golang-jwt, bcrypt
- **Frontend:** React 18, TypeScript (strict), Vite, Tailwind CSS 4, DaisyUI 5, Redux Toolkit, RTK Query
- **Audio:** FFmpeg (optional), bounded worker pool
- **Transcription:** go-whisper (whisper.cpp HTTP sidecar)
- **Storage:** SQLite for metadata + filesystem for audio files

---

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).
