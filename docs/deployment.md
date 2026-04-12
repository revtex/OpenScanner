# OpenScanner — Deployment Guide

> **Partial.** Server configuration and build commands are implemented. Docker, reverse proxy, and TLS auto-cert documentation will be expanded as those features are tested.

## Server Configuration

OpenScanner is configured via CLI flags, environment variables, or an optional INI file (`openscanner.ini`).

**Precedence:** CLI flags > environment variables > INI file > built-in defaults.

| Flag               | Env Var                      | Default           | Description                                           |
| ------------------ | ---------------------------- | ----------------- | ----------------------------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`         | `:3000`           | HTTP listen address                                   |
| `--db-file`        | `OPENSCANNER_DB_FILE`        | `openscanner.db`  | SQLite database file path                             |
| `--recordings-dir` | `OPENSCANNER_RECORDINGS_DIR` | executable dir    | Directory for call audio recordings                   |
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

Multi-stage build: Go 1.25 + Node 22 build stages → Alpine 3.21 runtime with FFmpeg.

```bash
docker build -t openscanner .
docker run -p 3000:3000 -v ./data:/data -e OPENSCANNER_RECORDINGS_DIR=/data openscanner
```

### docker-compose

```yaml
services:
  openscanner:
    build: .
    ports:
      - "3000:3000"
    volumes:
      - ./data:/data
    environment:
      - OPENSCANNER_RECORDINGS_DIR=/data
      - OPENSCANNER_LISTEN=0.0.0.0:3000
    restart: unless-stopped
```

## Reverse Proxy, Caddy, Let's Encrypt

> Planned — to be documented when TLS and reverse proxy configurations are tested.
