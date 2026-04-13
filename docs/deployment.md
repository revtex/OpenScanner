# OpenScanner — Deployment Guide

## Overview

OpenScanner ships as a **single self-contained binary** that embeds the entire React frontend. No separate web server or static file hosting is needed. The binary listens on `:3000` by default and serves:

- The React SPA (embedded via `go:embed`)
- REST API at `/api/`
- WebSocket hub at `/ws`
- Call audio files from the recordings directory

Runtime dependencies: **FFmpeg** (audio conversion) and **SQLite** (via `modernc.org/sqlite`, no CGO required).

---

## Server Configuration

OpenScanner is configured via CLI flags, environment variables, or an optional INI file (`openscanner.ini`).

**Precedence:** CLI flags > environment variables > INI file > built-in defaults.

| Flag               | Env Var                      | INI key          | Default           | Description                                           |
| ------------------ | ---------------------------- | ---------------- | ----------------- | ----------------------------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`         | `listen`         | `:3000`           | HTTP listen address                                   |
| `--db-file`        | `OPENSCANNER_DB_FILE`        | `db_file`        | `openscanner.db`  | SQLite database file path                             |
| `--recordings-dir` | `OPENSCANNER_RECORDINGS_DIR` | `recordings_dir` | executable dir    | Directory for call audio recordings                   |
| `--ssl-listen`     | `OPENSCANNER_SSL_LISTEN`     | `ssl_listen`     | —                 | HTTPS listen address (e.g. `:8443`)                   |
| `--ssl-cert`       | `OPENSCANNER_SSL_CERT`       | `ssl_cert_file`  | —                 | TLS certificate file (PEM)                            |
| `--ssl-key`        | `OPENSCANNER_SSL_KEY`        | `ssl_key_file`   | —                 | TLS private key file (PEM)                            |
| `--ssl-auto-cert`  | `OPENSCANNER_SSL_AUTO_CERT`  | `ssl_auto_cert`  | —                 | Domain for Let's Encrypt auto-cert                    |
| `--timezone`       | `OPENSCANNER_TIMEZONE`, `TZ` | `timezone`       | UTC               | IANA timezone for recorder timestamps                 |
| `--admin-password` | `OPENSCANNER_ADMIN_PASSWORD` | —                | —                 | Reset admin password on startup                       |
| `--config`         | —                            | —                | `openscanner.ini` | Path to INI config file                               |
| `--config-save`    | —                            | —                | —                 | Write current flags to INI and exit                   |
| `--version`        | —                            | —                | —                 | Print version and exit                                |
| `--service`        | —                            | —                | —                 | Service command: install/uninstall/start/stop/restart |

### INI file example

```ini
listen          = 0.0.0.0:3000
db_file         = /data/openscanner.db
recordings_dir  = /data/recordings
timezone        = America/New_York
```

Generate an INI from current flags:

```bash
./openscanner --listen 0.0.0.0:3000 --db-file /data/openscanner.db --config-save
```

---

## Development

```bash
make dev    # Starts Go (air hot-reload) + Vite dev server concurrently
make build  # Builds frontend → copies to embed path → builds Go binary
make test   # Runs all tests (Go + frontend)
make lint   # Lints all code
```

---

## Building from Source

Requirements: Go 1.25+, Node 22+, pnpm, FFmpeg (runtime only).

```bash
git clone https://github.com/revtex/openscanner.git
cd openscanner
make build
# Binary: backend/tmp/openscanner  (or backend/openscanner depending on backend Makefile)
```

`make build` does:

1. `pnpm build` in `frontend/` → produces `frontend/dist/`
2. Copies `frontend/dist/` → `backend/internal/static/dist/` (the `go:embed` target)
3. `go build` in `backend/` → produces the binary with the frontend embedded

---

## Docker

The Dockerfile uses a three-stage build:

1. **node-builder** — builds the React frontend
2. **go-builder** — copies the built frontend into `internal/static/dist/` then compiles the Go binary (so `go:embed` picks it up)
3. **runtime** — Alpine 3.21 + FFmpeg; copies only the binary; runs as non-root `appuser`

The frontend is **fully embedded** in the binary — no separate static directory is needed in the container.

### Build and run

```bash
# Build image
docker build -t openscanner .

# Run (data volume holds DB + recordings)
docker run -d \
  --name openscanner \
  -p 3000:3000 \
  -v "$(pwd)/data:/data" \
  -e OPENSCANNER_DB_FILE=/data/openscanner.db \
  -e OPENSCANNER_RECORDINGS_DIR=/data/recordings \
  -e OPENSCANNER_LISTEN=0.0.0.0:3000 \
  openscanner
```

### docker-compose

```yaml
services:
  openscanner:
    image: ghcr.io/revtex/openscanner:main
    # build: .  # uncomment to build locally instead
    ports:
      - "3000:3000"
    volumes:
      - ./data:/data
    environment:
      - OPENSCANNER_DB_FILE=/data/openscanner.db
      - OPENSCANNER_RECORDINGS_DIR=/data/recordings
      - OPENSCANNER_LISTEN=0.0.0.0:3000
      # Set timezone for recorder timestamp interpretation (IANA format).
      # - TZ=America/New_York
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3000/api/health"]
      interval: 30s
      timeout: 5s
      start_period: 10s
      retries: 3
    restart: unless-stopped
```

```bash
docker compose up -d
docker compose logs -f
```

---

## Bare Metal / Linux

### Install steps

```bash
# 1. Copy binary and create data directories
sudo cp openscanner /usr/local/bin/openscanner
sudo chmod +x /usr/local/bin/openscanner
sudo mkdir -p /var/lib/openscanner/recordings

# 2. Create a dedicated system user
sudo useradd -r -s /bin/false openscanner
sudo chown -R openscanner:openscanner /var/lib/openscanner

# 3. Install and start the systemd service (see unit file below)
sudo cp openscanner.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now openscanner
```

### systemd unit file

Save as `/etc/systemd/system/openscanner.service`:

```ini
[Unit]
Description=OpenScanner radio call manager
After=network.target

[Service]
Type=simple
User=openscanner
Group=openscanner
ExecStart=/usr/local/bin/openscanner \
  --listen 0.0.0.0:3000 \
  --db-file /var/lib/openscanner/openscanner.db \
  --recordings-dir /var/lib/openscanner/recordings
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/openscanner

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl status openscanner
sudo journalctl -u openscanner -f
```

---

## System Service (Windows / macOS)

OpenScanner integrates with the OS service manager via the `--service` flag:

```bash
# Install as a system service (runs with current binary path)
sudo ./openscanner --service install

# Manage the service
sudo ./openscanner --service start
sudo ./openscanner --service stop
sudo ./openscanner --service restart

# Remove the service
sudo ./openscanner --service uninstall
```

On Linux this uses systemd; on macOS it uses launchd; on Windows it uses the Windows Service Manager.

---

## TLS / HTTPS

### Manual certificate (PEM files)

```bash
./openscanner \
  --listen 0.0.0.0:3000 \
  --ssl-listen 0.0.0.0:8443 \
  --ssl-cert /path/to/fullchain.pem \
  --ssl-key /path/to/privkey.pem
```

Both `--listen` (HTTP) and `--ssl-listen` (HTTPS) can be active simultaneously, allowing an HTTP→HTTPS redirect at the reverse proxy level while the app itself handles both.

### Let's Encrypt auto-cert

```bash
sudo ./openscanner \
  --ssl-auto-cert scanner.example.com \
  --ssl-listen 0.0.0.0:443 \
  --listen 0.0.0.0:80
```

- Binds port 443 (requires root or `CAP_NET_BIND_SERVICE`)
- Automatically obtains and renews a certificate via ACME HTTP-01 challenge
- Certificate is cached on disk next to the binary

---

## Reverse Proxy

Use a reverse proxy (nginx or Caddy) when you want:

- A single port 80/443 entry point for multiple services
- Managed TLS (Let's Encrypt via the proxy)
- HTTP→HTTPS redirect

### nginx

```nginx
server {
    listen 80;
    server_name scanner.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name scanner.example.com;

    ssl_certificate     /etc/letsencrypt/live/scanner.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/scanner.example.com/privkey.pem;

    location / {
        proxy_pass         http://127.0.0.1:3000;
        proxy_http_version 1.1;

        # WebSocket support (required for live scanner feed)
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}
```

### Caddy

```caddy
scanner.example.com {
    reverse_proxy localhost:3000 {
        # WebSocket connections are handled automatically by Caddy
        transport http {
            read_buffer_size  16384
        }
    }
}
```

Caddy automatically provisions and renews Let's Encrypt certificates.

---

## Environment Variables Reference

| Variable                     | Equivalent Flag    | Description                             |
| ---------------------------- | ------------------ | --------------------------------------- |
| `OPENSCANNER_LISTEN`         | `--listen`         | HTTP listen address                     |
| `OPENSCANNER_DB_FILE`        | `--db-file`        | SQLite database file path               |
| `OPENSCANNER_RECORDINGS_DIR` | `--recordings-dir` | Call audio recordings directory         |
| `OPENSCANNER_SSL_LISTEN`     | `--ssl-listen`     | HTTPS listen address                    |
| `OPENSCANNER_SSL_CERT`       | `--ssl-cert`       | TLS certificate file (PEM)              |
| `OPENSCANNER_SSL_KEY`        | `--ssl-key`        | TLS private key file (PEM)              |
| `OPENSCANNER_SSL_AUTO_CERT`  | `--ssl-auto-cert`  | Domain for Let's Encrypt auto-cert      |
| `OPENSCANNER_TIMEZONE`       | `--timezone`       | IANA timezone (e.g. `America/New_York`) |
| `TZ`                         | `--timezone`       | Standard timezone env (fallback)        |
| `OPENSCANNER_ADMIN_PASSWORD` | `--admin-password` | Reset admin password on startup         |

---

## Verifying the Deployment

### Health check endpoint

```bash
curl http://localhost:3000/api/health
# 200 OK → {"status":"ok"}
```

### What to look for in logs

On a successful start you should see (via `slog` JSON or text output):

```
INFO  database migrated  path=/data/openscanner.db
INFO  seeded default settings
INFO  listening  addr=0.0.0.0:3000
```

On first run, navigate to `http://<host>:3000` — you will be redirected to the **Setup Wizard** (`/setup`) to configure the admin account and initial settings.
