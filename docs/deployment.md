# OpenScanner — Deployment Guide

> Full deployment documentation to be completed during Phase 12 / 14.

## Server Configuration

OpenScanner is configured via CLI flags, environment variables, or an optional INI file (`openscanner.ini`).

**Precedence:** CLI flags > environment variables > INI file > built-in defaults.

| Flag               | Env Var                      | Default           | Description                         |
| ------------------ | ---------------------------- | ----------------- | ----------------------------------- |
| `--listen`         | `OPENSCANNER_LISTEN`         | `:3000`           | HTTP listen address                 |
| `--db-file`        | `OPENSCANNER_DB_FILE`        | `openscanner.db`  | SQLite database file path           |
| `--base-dir`       | `OPENSCANNER_BASE_DIR`       | executable dir    | Base directory for data files       |
| `--ssl-listen`     | `OPENSCANNER_SSL_LISTEN`     | —                 | HTTPS listen address                |
| `--ssl-cert`       | `OPENSCANNER_SSL_CERT`       | —                 | TLS certificate file (PEM)          |
| `--ssl-key`        | `OPENSCANNER_SSL_KEY`        | —                 | TLS private key file (PEM)          |
| `--ssl-auto-cert`  | `OPENSCANNER_SSL_AUTO_CERT`  | —                 | Domain for Let's Encrypt auto-cert  |
| `--admin-password` | `OPENSCANNER_ADMIN_PASSWORD` | —                 | Reset admin password on startup     |
| `--config`         | —                            | `openscanner.ini` | Path to INI config file             |
| `--config-save`    | —                            | —                 | Write current flags to INI and exit |
| `--version`        | —                            | —                 | Print version and exit              |

## Development

```bash
make dev    # Starts Go (air hot-reload) + Vite dev server
make build  # Builds production binary + frontend
make test   # Runs all tests
```

## Docker

Covers: bare metal, Docker, docker-compose, nginx reverse proxy, Caddy, Let's Encrypt auto-cert, environment variables, data directory layout.
