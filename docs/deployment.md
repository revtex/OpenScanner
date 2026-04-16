# OpenScanner Deployment Guide

## Overview

OpenScanner runs as a single Go binary that serves:

- embedded frontend SPA
- REST API under /api
- WebSocket endpoints
- local audio file streaming

No external database is required. SQLite is embedded and uses WAL mode.

## Configuration

Configuration precedence:

CLI flags > environment variables > INI file > defaults

Key startup flags:

- --listen
- --db-file
- --recordings-dir
- --ssl-listen
- --ssl-cert
- --ssl-key
- --ssl-auto-cert
- --timezone
- --admin-password
- --config
- --config-save
- --version
- --service

Environment variable equivalents are OPENSCANNER_* variants plus TZ for timezone fallback.

## Build and Run

From repository root:

```bash
make build
./build/openscanner --listen 0.0.0.0:3000 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

Root build process:

1. Build frontend
2. Copy frontend dist into backend embed directory
3. Build backend binary

Backend build also regenerates Swagger docs.

## Docker

The Dockerfile uses three stages:

1. node-builder (frontend build)
2. go-builder (backend build with embedded frontend)
3. alpine runtime (ffmpeg + certs + non-root user)

Run with persistent /data volume for DB and recordings.

## Docker Compose

The included compose file maps:

- port 3000
- ./data:/data
- OPENSCANNER_DB_FILE
- OPENSCANNER_RECORDINGS_DIR
- OPENSCANNER_LISTEN

Healthcheck hits /api/health.

## Development Commands

```bash
make dev
make build
make test
make lint
```

## Linux Service

OpenScanner supports service lifecycle commands:

```bash
./openscanner --service install
./openscanner --service start
./openscanner --service stop
./openscanner --service restart
./openscanner --service uninstall
```

kardianos/service is used for OS-specific service integration.

## TLS

Two TLS modes are supported:

- certificate/key files (--ssl-cert, --ssl-key)
- automatic Let's Encrypt (--ssl-auto-cert)

When TLS is enabled, HTTP listener redirects traffic to HTTPS.

## Reverse Proxy

You can run OpenScanner behind nginx/Caddy for centralized TLS and host routing.

When proxying, ensure WebSocket upgrade headers are forwarded for /ws and /api/admin/ws.

## Verification Checklist

- GET /api/health returns 200
- Startup logs show listen address, db path, and recordings path
- /setup appears on first run
- Recorder uploads persist calls and audio
- Admin login and /admin panels load successfully
