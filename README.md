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

- [x] **Phase 1** — Foundation & Scaffolding (Go module, frontend deps, config, dev tooling)
- [ ] **Phase 2** — Database Schema & Seeding
- [ ] **Phase 3** — Backend Auth, RBAC & Setup
- [ ] **Phase 4** — Backend Call Ingest
- [ ] **Phase 5** — WebSocket Hub

See [docs/plan.md](docs/plan.md) for the full implementation plan.

## Documentation

- [Architecture](docs/architecture.md) — System diagram & component descriptions
- [API Reference](docs/api.md) — Endpoint documentation
- [Admin Guide](docs/admin-guide.md) — UI walkthrough
- [Deployment](docs/deployment.md) — Docker, bare metal, reverse proxy
- [Recorder Integration](docs/recorder-integration.md) — Per-recorder setup
