# OpenScanner

OpenScanner is a web-based radio call manager for monitoring, searching, and administering scanner traffic in real time.

It is a modern reimplementation of rdio-scanner, built as a single Go application with an embedded React frontend.

## Why OpenScanner

- Real-time live scanner feed over WebSocket
- Historical call archive with fast filtering
- Built-in admin dashboard for users, radio data, ingest, and operations
- Flexible ingest options: HTTP upload and directory monitoring
- Simple operations model: SQLite + filesystem (no external database)
- Straightforward deployment: one binary or Docker image

## What It Does

OpenScanner sits between your recorder(s) and your listeners:

1. Ingests calls from recorder uploads or watched directories
2. Processes audio with configurable FFmpeg modes
3. Stores metadata in SQLite and audio on disk
4. Streams live calls to browser clients
5. Provides an admin UI for configuration and operations

## Key Features

- Live scanner interface with playback controls, hold/avoid, talkgroup select, bookmarks, and archive search
- Call archive filtering by system, talkgroup, groups, tags, date range, transcript text, and bookmark state
- Role-based auth with admin/listener users
- Admin CRUD for users, systems, talkgroups, units, groups, tags, API keys, dir monitors, downstreams, and webhooks
- Tools for CSV import/export, JSON config import/export, missing-audio cleanup, and RadioReference enrichment
- Shareable call links and admin shared-link management
- Configurable public access mode for listener behavior

## Quick Start

### Docker Compose

```bash
docker compose up -d
```

Then open http://localhost:3000.

### Build and Run Locally

```bash
make build
./build/openscanner --listen 0.0.0.0:3000 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

Then open http://localhost:3000.

On first run, complete setup at /setup to create your first admin account.

## API and Docs

- API base path: /api
- Listener WebSocket: /ws
- Admin WebSocket: /api/admin/ws

Swagger UI is available at /api/admin/docs after creating a docs session with POST /api/admin/docs/session as an authenticated admin.

## Documentation

- docs/architecture.md: Architecture and runtime data flow
- docs/api.md: API behavior and integration workflows
- docs/admin-guide.md: Admin dashboard usage guide
- docs/deployment.md: Build, run, and deployment operations
- docs/recorder-setup.md: Recorder-specific setup steps
- docs/recorder-integration.md: DirMonitor internals and ingest behavior
- docs/plan.md: Project roadmap

## Development

```bash
make dev
make build
make test
make lint
```

## Tech Stack

- Backend: Go, Gin, coder/websocket, sqlite (modernc), sqlc
- Frontend: React, TypeScript, Vite, Tailwind CSS 4, DaisyUI 5, Redux Toolkit
- Storage: SQLite metadata + filesystem audio

## Project Status

Core ingest, search, sharing, streaming, and admin operations are implemented and actively maintained.
