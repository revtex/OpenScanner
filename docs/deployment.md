# OpenScanner — Deployment Guide

> **Partial.** Server configuration and build commands are implemented. Docker, reverse proxy, and TLS auto-cert documentation will be expanded as those features are tested.

## Server Configuration

OpenScanner is configured via CLI flags, environment variables, or an optional INI file (`openscanner.ini`).

**Precedence:** CLI flags > environment variables > INI file > built-in defaults.

| Flag               | Env Var                      | Default           | Description                                           |
| ------------------ | ---------------------------- | ----------------- | ----------------------------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`         | `:3000`           | HTTP listen address                                   |
| `--db-file`        | `OPENSCANNER_DB_FILE`        | `openscanner.db`  | SQLite database file path                             |
| `--base-dir`       | `OPENSCANNER_BASE_DIR`       | executable dir    | Base directory for data files                         |
| `--ssl-listen`     | `OPENSCANNER_SSL_LISTEN`     | —                 | HTTPS listen address                                  |
| `--ssl-cert`       | `OPENSCANNER_SSL_CERT`       | —                 | TLS certificate file (PEM)                            |
| `--ssl-key`        | `OPENSCANNER_SSL_KEY`        | —                 | TLS private key file (PEM)                            |
| `--ssl-auto-cert`  | `OPENSCANNER_SSL_AUTO_CERT`  | —                 | Domain for Let's Encrypt auto-cert                    |
| `--admin-password` | `OPENSCANNER_ADMIN_PASSWORD` | —                 | Reset admin password on startup                       |
| `--config`         | —                            | `openscanner.ini` | Path to INI config file                               |
| `--config-save`    | —                            | —                 | Write current flags to INI and exit                   |
| `--version`        | —                            | —                 | Print version and exit                                |
| `--service`        | —                            | —                 | Service command: install/uninstall/start/stop/restart |

## Development

```bash
make dev    # Starts Go (air hot-reload) + Vite dev server
make build  # Builds production binary + frontend
make test   # Runs all tests
```

## Docker

### Dockerfile

Multi-stage build: Go 1.24 + Node 22 build stages → Alpine 3.21 runtime with FFmpeg.

```bash
docker build -t openscanner .
docker run -p 3000:3000 -v ./data:/app openscanner
```

### docker-compose

> **Note:** The docker-compose.yml currently uses `OPENSCANNER_DATA_DIR` and `OPENSCANNER_AUDIO_DIR` environment variables which are not yet recognised by the server. For now, use the supported variables (`OPENSCANNER_LISTEN`, `OPENSCANNER_DB_FILE`, `OPENSCANNER_BASE_DIR`) instead.

```yaml
services:
  openscanner:
    build: .
    ports:
      - "3000:3000"
    volumes:
      - ./data:/app
    environment:
      - OPENSCANNER_LISTEN=0.0.0.0:3000
      - OPENSCANNER_DB_FILE=/app/openscanner.db
      - OPENSCANNER_BASE_DIR=/app
    restart: unless-stopped
```

## Reverse Proxy, Caddy, Let's Encrypt

> Planned — to be documented when TLS and reverse proxy configurations are tested.
