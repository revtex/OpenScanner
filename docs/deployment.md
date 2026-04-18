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

Environment variable equivalents are OPENSCANNER\_\* variants plus TZ for timezone fallback.

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

### Nginx Configuration

Use this when OpenScanner listens on localhost (example: `127.0.0.1:3000`) and Nginx handles TLS.

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
		proxy_pass http://127.0.0.1:3000;
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

### Caddy Configuration

Use this when OpenScanner listens on localhost (example: `127.0.0.1:3000`) and Caddy handles automatic HTTPS.

```caddy
scanner.example.com {
	encode gzip zstd

	reverse_proxy 127.0.0.1:3000
}
```

Caddy automatically forwards headers needed by OpenScanner (including `X-Forwarded-Proto`) and handles WebSocket upgrades.

If you need to be explicit:

```caddy
scanner.example.com {
	encode gzip zstd

	reverse_proxy 127.0.0.1:3000 {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-Host {host}
		header_up X-Forwarded-For {remote_host}
	}
}
```

### Proxy Deployment Tips

- Run OpenScanner on a private bind address (for example, `--listen 127.0.0.1:3000`) when fronted by a reverse proxy.
- Keep proxy and OpenScanner clocks synchronized (NTP) so JWT and cookie expiry behavior is consistent.
- Test both WebSocket endpoints after deployment:
  - `/ws` (scanner/live data)
  - `/api/admin/ws` (admin channel)

## Verification Checklist

- GET /api/health returns 200
- Startup logs show listen address, db path, and recordings path
- /setup appears on first run
- Recorder uploads persist calls and audio
- Admin login and /admin panels load successfully
