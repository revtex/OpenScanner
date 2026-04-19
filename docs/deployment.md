# OpenScanner Deployment Guide

## Overview

OpenScanner runs as a single Go binary that serves:

- Embedded frontend SPA
- REST API under `/api`
- WebSocket endpoints (`/ws`, `/api/admin/ws`)
- Local audio file streaming

No external database is required. SQLite is embedded and uses WAL mode.

Supported platforms: **Linux**, **macOS**, **Windows**.

---

## Configuration

### Precedence

CLI flags > environment variables > JSON config file > built-in defaults

### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--listen` | HTTP listen address | `:3022` |
| `--db-file` | SQLite database file path | `openscanner.db` |
| `--recordings-dir` | Directory for call audio recordings | (executable directory) |
| `--ssl-listen` | HTTPS listen address | (disabled) |
| `--ssl-cert` | TLS certificate file (PEM) | |
| `--ssl-key` | TLS private key file (PEM) | |
| `--ssl-auto-cert` | Domain for Let's Encrypt auto-cert | |
| `--timezone` | IANA timezone for recorder timestamps | `UTC` |
| `--admin-password` | Reset first admin user's password on startup | |
| `--config` | Path to JSON config file | `openscanner.json` |
| `--config-save` | Write current flags to JSON config and exit | |
| `--version` | Print version and exit | |
| `--service` | Service command: install, uninstall, start, stop, restart | |

### Environment Variables

| Variable | Maps to flag |
|----------|--------------|
| `OPENSCANNER_LISTEN` | `--listen` |
| `OPENSCANNER_DB_FILE` | `--db-file` |
| `OPENSCANNER_RECORDINGS_DIR` | `--recordings-dir` |
| `OPENSCANNER_SSL_LISTEN` | `--ssl-listen` |
| `OPENSCANNER_SSL_CERT` | `--ssl-cert` |
| `OPENSCANNER_SSL_KEY` | `--ssl-key` |
| `OPENSCANNER_SSL_AUTO_CERT` | `--ssl-auto-cert` |
| `OPENSCANNER_ADMIN_PASSWORD` | `--admin-password` |
| `OPENSCANNER_TIMEZONE` | `--timezone` |
| `TZ` | `--timezone` (fallback) |

### JSON Config File

When `--config-save` is used, a JSON file is written containing persistent settings:

```json
{
  "listen": ":3022",
  "db_file": "/var/lib/openscanner/openscanner.db",
  "recordings_dir": "/var/lib/openscanner/recordings",
  "ssl_listen": "",
  "ssl_cert_file": "",
  "ssl_key_file": "",
  "ssl_auto_cert": "",
  "timezone": ""
}
```

Transient flags (`--admin-password`, `--config-save`, `--version`, `--service`) are never persisted.

---

## Quick Start (Recommended)

### Guided Setup

```bash
sudo ./openscanner setup --interactive
```

The setup command:
1. Prompts for listen address, database path, recordings directory, config file, and install path
2. Creates all required directories
3. Writes and validates the JSON config file
4. Copies the executable to the install path
5. Installs and starts the system service

Without `--interactive`, setup uses platform defaults (see below).

### Verify Installation

```bash
curl -f http://127.0.0.1:3022/api/health
openscanner service doctor
openscanner config validate
```

---

## Platform Defaults

Setup paths are computed per-platform at runtime:

### Linux

| Setting | Default Path |
|---------|-------------|
| Config | `/etc/openscanner/openscanner.json` |
| Database | `/var/lib/openscanner/openscanner.db` |
| Recordings | `/var/lib/openscanner/recordings` |
| Executable | `/usr/local/bin/openscanner` |
| Service | systemd / SysV / OpenRC (auto-detected) |

### macOS

| Setting | Default Path |
|---------|-------------|
| Config | `/usr/local/etc/openscanner/openscanner.json` |
| Database | `/usr/local/var/lib/openscanner/openscanner.db` |
| Recordings | `/usr/local/var/lib/openscanner/recordings` |
| Executable | `/usr/local/bin/openscanner` |
| Service | launchd |

### Windows

| Setting | Default Path |
|---------|-------------|
| Config | `%ProgramData%\OpenScanner\openscanner.json` |
| Database | `%ProgramData%\OpenScanner\openscanner.db` |
| Recordings | `%ProgramData%\OpenScanner\recordings` |
| Executable | `%ProgramFiles%\OpenScanner\openscanner.exe` |
| Service | Windows Service Control Manager |

All defaults can be overridden with setup flags:

```bash
openscanner setup \
  --listen 0.0.0.0:3022 \
  --db-file /opt/openscanner/data.db \
  --recordings-dir /opt/openscanner/recordings \
  --config /opt/openscanner/config.json \
  --install-binary /opt/openscanner/openscanner
```

---

## Service Management

### Subcommands

| Command | Description |
|---------|-------------|
| `openscanner setup` | Full guided install (create dirs, write config, install service, start) |
| `openscanner setup --interactive` | Interactive setup with prompts |
| `openscanner setup --force` | Overwrite existing setup / reinstall service |
| `openscanner upgrade` | Replace installed binary, restart service if running |
| `openscanner config validate` | Validate JSON config file and check paths are writable |
| `openscanner service doctor` | Print service status diagnostics |

### Low-Level Service Control

For manual control without the setup wrapper:

```bash
openscanner --service install --config /path/to/openscanner.json
openscanner --service start
openscanner --service stop
openscanner --service restart
openscanner --service uninstall
```

Startup flags provided alongside `--service install` are persisted into the service definition (except transient flags like `--admin-password`, `--version`, `--config-save`).

### Upgrade Flow

```bash
# Download new binary
curl -L -o /tmp/openscanner-new https://github.com/...

# Upgrade (stops service → copies binary → restarts)
openscanner upgrade --binary /tmp/openscanner-new
```

If the service was stopped before upgrade, it remains stopped after.

---

## Build from Source

### Requirements

- Go 1.25+
- Node.js 22+ with pnpm
- Make

### Build

```bash
make build
```

This:
1. Builds the frontend (`pnpm build`)
2. Copies frontend dist into `backend/internal/static/dist/` for `go:embed`
3. Builds the Go binary to `build/openscanner`

### Run Locally

```bash
./build/openscanner --listen 0.0.0.0:3022 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

### Development

```bash
make dev    # Hot-reload backend (air) + Vite dev server with proxy
make test   # Run all backend + frontend tests
make lint   # Run linters
```

---

## Docker

### Dockerfile

Multi-stage build (Node → Go → Alpine runtime):

- **Stage 1**: `node:22-alpine` — builds frontend
- **Stage 2**: `golang:1.25-alpine` — builds Go binary with embedded frontend
- **Stage 3**: `alpine:3.21` — minimal runtime with ffmpeg, ca-certificates, non-root user

The container exposes port **3000** and stores data under `/data`.

### Docker Compose

```yaml
services:
  openscanner:
    image: ghcr.io/revtex/openscanner:main
    ports:
      - "3000:3000"
    volumes:
      - ./data:/data
    environment:
      - OPENSCANNER_DB_FILE=/data/openscanner.db
      - OPENSCANNER_RECORDINGS_DIR=/data/recordings
      - OPENSCANNER_LISTEN=0.0.0.0:3000
      # - TZ=America/New_York
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3000/api/health"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
    restart: unless-stopped
```

### Custom Build

```bash
docker compose build   # or: docker build -t openscanner .
docker compose up -d
```

---

## TLS

Two modes are supported:

### Certificate Files

```bash
openscanner --ssl-listen :443 --ssl-cert /path/to/cert.pem --ssl-key /path/to/key.pem
```

### Automatic Let's Encrypt

```bash
openscanner --ssl-auto-cert scanner.example.com
```

When any TLS mode is enabled, the HTTP listener automatically redirects to HTTPS.

---

## Reverse Proxy

Run OpenScanner behind nginx or Caddy for centralized TLS and host routing. Key requirements:

- Forward WebSocket upgrade headers for `/ws` and `/api/admin/ws`
- Forward `X-Forwarded-Proto` so secure cookies work correctly
- Bind OpenScanner to localhost (`--listen 127.0.0.1:3022`) when proxied

### Nginx

```nginx
map $http_upgrade $connection_upgrade {
	default upgrade;
	''      close;
}

server {
	listen 80;
	server_name scanner.example.com;
	return 301 https://$host$request_uri;
}

server {
	listen 443 ssl http2;
	server_name scanner.example.com;

	ssl_certificate     /etc/letsencrypt/live/scanner.example.com/fullchain.pem;
	ssl_certificate_key /etc/letsencrypt/live/scanner.example.com/privkey.pem;

	client_max_body_size 100m;

	location / {
		proxy_pass http://127.0.0.1:3022;
		proxy_http_version 1.1;

		proxy_set_header Host $host;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;

		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection $connection_upgrade;

		proxy_read_timeout 3600;
		proxy_send_timeout 3600;
	}
}
```

Notes:

- `X-Forwarded-Proto $scheme` is required so OpenScanner can detect HTTPS and set secure refresh cookies correctly.
- WebSocket upgrade headers must be present for `/ws` and `/api/admin/ws`.
- Increase `client_max_body_size` if you upload larger files (imports/audio).

### Caddy

```caddy
scanner.example.com {
	encode gzip zstd
	reverse_proxy 127.0.0.1:3022
}
```

Caddy automatically handles TLS, WebSocket upgrades, and forwarded headers.

For explicit header control:

```caddy
scanner.example.com {
	encode gzip zstd

	reverse_proxy 127.0.0.1:3022 {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-Host {host}
		header_up X-Forwarded-For {remote_host}
	}
}
```

### Proxy Tips

- Bind OpenScanner to `127.0.0.1:3022` when fronted by a reverse proxy.
- Keep clocks synchronized (NTP) so JWT and cookie expiry are consistent.
- Test both WebSocket endpoints after deployment: `/ws` and `/api/admin/ws`.

---

## External Dependencies

| Dependency | Required | Purpose |
|------------|----------|---------|
| FFmpeg | Optional | Audio format conversion and normalization (4 modes: disabled/enabled/norm/loudnorm) |
| Whisper | Optional | Local speech-to-text transcription (CPU or GPU via NVIDIA CUDA) |

Both are invoked via argument slices (no shell execution) for security.

---

## Verification Checklist

- [ ] `GET /api/health` returns 200
- [ ] Startup logs show listen address, db path, and recordings path
- [ ] `/setup` appears on first run (initial admin creation)
- [ ] Recorder uploads persist calls and audio files
- [ ] Admin login and `/admin` panels load
- [ ] WebSocket at `/ws` delivers live calls
- [ ] `openscanner service doctor` reports running
- [ ] `openscanner config validate` passes
