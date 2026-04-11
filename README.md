# OpenScanner

A modern reimplementation of [rdio-scanner](https://github.com/chuot/rdio-scanner) built with Go + React.

## Stack

- **Backend:** Go 1.25, Gin, coder/websocket, modernc.org/sqlite, sqlc, kardianos/service, log/slog, go:embed
- **Frontend:** React 18, TypeScript, Vite, DaisyUI, Tailwind CSS, Redux Toolkit, RTK Query, @tanstack/react-virtual
- **Database:** SQLite (WAL mode — config & metadata stored in DB)
- **Server config:** CLI flags, environment variables, or optional INI file (precedence: flags > env > INI > defaults)
- **Audio:** Filesystem storage, FFmpeg for conversion (4 modes), bounded worker pool
- **Deployment:** Single binary (frontend embedded via go:embed), Docker, or system service
- **Dev tooling:** `air` (Go hot-reload) + Vite proxy

## Quick Start

### Development

```bash
make dev    # Starts Go backend (air hot-reload) + Vite dev server
```

### Build

```bash
make build  # Builds Go binary + frontend production bundle
```

### Test

```bash
make test   # Runs Go tests + Vitest frontend tests
```

## Project Status

- [x] **Phase 1** — Foundation & Scaffolding
- [x] **Phase 2** — Database Schema & Seeding
- [x] **Phase 3** — Backend Auth, RBAC & Setup
- [x] **Phase 4** — Backend Call Ingest
- [x] **Phase 5** — WebSocket Hub
- [x] **Phase 6** — Admin CRUD APIs
- [x] **Phase 7** — DirWatch Service
- [x] **Phase 8** — Downstream Pusher
- [x] **Phase 9** — Frontend Scanner UI
- [ ] **Phase 10** — Frontend TG Selection & Search Panels
- [ ] **Phase 11** — Frontend Admin Dashboard
- [ ] **Phase 12** — CLI, Daemon, SSL, Docker & Deployment
- [ ] **Phase 13** — Testing
- [ ] **Phase 14** — Documentation
- [ ] **Phase 15** — Keyboard Shortcuts & Theme Toggle
- [ ] **Phase 16** — Shareable Links, Bookmarks & Activity Dashboard
- [ ] **Phase 17** — Push Notifications & Webhook Integration
- [ ] **Phase 18** — Call Transcription

See [docs/plan.md](docs/plan.md) for the full implementation plan.

## Documentation

- [Architecture](docs/architecture.md) — System diagram & component descriptions
- [API Reference](docs/api.md) — Endpoint documentation
- [Admin Guide](docs/admin-guide.md) — UI walkthrough
- [Deployment](docs/deployment.md) — Docker, bare metal, reverse proxy
- [Recorder Integration](docs/recorder-integration.md) — Per-recorder setup
