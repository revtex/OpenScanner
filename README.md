# OpenScanner

OpenScanner is a web-based radio call manager for monitoring, searching, and administering scanner traffic in real time.

It is a modern reimplementation of [rdio-scanner](https://github.com/chuot/rdio-scanner), built as a single Go application with an embedded React frontend.

## Why OpenScanner

- Real-time live scanner feed over WebSocket
- Historical call archive with fast filtering
- Built-in admin dashboard for systems, talkgroups, users, API keys, and tools
- Flexible ingest options: HTTP uploader and directory watch
- Simple operations model: SQLite + filesystem, no external database required
- Easy deployment: one binary or Docker

## What It Does

OpenScanner is designed to sit between your recorder(s) and your listeners:

1. Ingests calls from recorder uploads or watched directories
2. Processes audio with configurable FFmpeg modes
3. Stores metadata in SQLite and audio on disk
4. Streams live calls to browser clients
5. Provides an admin UI for configuration and operations

## Key Features

- Live scanner interface with queue/history behavior and playback controls
- Call archive search by system, talkgroup, date range, and sort direction
- Role-based auth with admin and listener permissions
- Admin CRUD for users, systems, talkgroups, units, groups, tags, API keys, dirwatches, downstreams, and webhooks
- Tools for CSV import, full JSON config import/export, and missing-audio cleanup
- Configurable public access mode for listener endpoint behavior
- Built-in health endpoint for orchestration checks

## Quick Start

### Option 1: Docker Compose

```bash
docker compose up -d
```

Then open http://localhost:3000.

### Option 2: Build and Run Locally

```bash
make build
./build/openscanner --listen 0.0.0.0:3000 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

Then open http://localhost:3000.

On first run, complete the setup flow at /setup to create your admin account.

## Recorder Integration

OpenScanner supports both:

- HTTP upload endpoints for recorder integrations
- DirWatch ingestion for recorders that write files locally

See [docs/recorder-integration.md](docs/recorder-integration.md) for recorder-specific setup and examples.

## Admin Dashboard

The admin UI is available at /admin and includes:

- User and role management
- Radio system, talkgroup, and unit management
- API key management for recorder uploads
- DirWatch and downstream configuration
- Settings, logs, and maintenance tools

See [docs/admin-guide.md](docs/admin-guide.md) for a full walkthrough.

## Deployment

Supported deployment styles:

- Docker and Docker Compose
- Single binary on Linux/macOS/Windows
- Service mode via operating system service manager
- Optional TLS via cert files or auto-cert mode

See [docs/deployment.md](docs/deployment.md) for production deployment guidance.

## Documentation

- [docs/architecture.md](docs/architecture.md): System architecture and data flow
- [docs/api.md](docs/api.md): API behavior guide and integration flow
- [docs/admin-guide.md](docs/admin-guide.md): Admin dashboard usage
- [docs/deployment.md](docs/deployment.md): Deployment and operations
- [docs/recorder-integration.md](docs/recorder-integration.md): Recorder setup and DirWatch
- [docs/plan.md](docs/plan.md): Implementation roadmap and project phases

Swagger UI is available at `/api/admin/docs` after creating a docs session via `POST /api/admin/docs/session` as an authenticated admin.

## Development

```bash
make dev
make build
make test
make lint
```

## Tech Stack

- Backend: Go, Gin, WebSocket, SQLite, sqlc
- Frontend: React, TypeScript, Vite, Tailwind CSS, DaisyUI, Redux Toolkit
- Storage: SQLite metadata + filesystem audio

## Project Status

Core ingestion, streaming, admin dashboard, sharing, and deployment paths are implemented and actively maintained.

For detailed phase tracking, see [docs/plan.md](docs/plan.md).
