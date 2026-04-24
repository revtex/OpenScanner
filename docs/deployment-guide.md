# Deployment Guide

This guide walks you through getting OpenScanner running at home. The Docker path is the easiest and works for most people — start there, and only dip into the Advanced section if you need something special.

## Contents

- [Quick Start with Docker](#quick-start-with-docker)
- [First-Time Login](#first-time-login)
- [Your Data Directory](#your-data-directory)
- [Backing Up](#backing-up)
- [Running Behind a Reverse Proxy](#running-behind-a-reverse-proxy)
- [HTTPS Options](#https-options)
- [Keeping Secrets Safe](#keeping-secrets-safe)
- [Transcription (Optional)](#transcription-optional)
- [FFmpeg (Optional)](#ffmpeg-optional)
- [Verification Checklist](#verification-checklist)
- [Advanced](#advanced)

---

## Quick Start with Docker

If you have Docker installed, you can be up and running in a couple of minutes.

1. Create a folder to hold your database and recordings, then step into it:

   ```bash
   mkdir -p openscanner/data
   cd openscanner
   ```

2. Create a file called `docker-compose.yml` with this content:

   ```yaml
   services:
     openscanner:
       image: ghcr.io/revtex/openscanner:dev
       ports:
         - "3022:3022"
       volumes:
         - ./data:/data
       environment:
         - OPENSCANNER_DB_FILE=/data/openscanner.db
         - OPENSCANNER_RECORDINGS_DIR=/data/recordings
         - OPENSCANNER_LISTEN=0.0.0.0:3022
         - TZ=America/New_York # change to your timezone
       healthcheck:
         test: ["CMD", "wget", "-qO-", "http://localhost:3022/api/health"]
         interval: 30s
         timeout: 5s
         start_period: 10s
         retries: 3
       restart: unless-stopped
   ```

3. Start it:

   ```bash
   docker compose up -d
   ```

4. Open <http://localhost:3022> in your browser. OpenScanner will walk you through creating your first admin account.

> **Tip:** Set `TZ` to your local timezone (the [IANA name](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones), like `America/Chicago` or `Europe/London`). That way recorder timestamps come out right.

That's it. Everything below is optional — only read on if you need it.

---

## First-Time Login

The first time you open OpenScanner you'll see a setup page instead of a login page. Pick a username and password and click **Create**. That account becomes the admin user.

There is no default username or password — OpenScanner doesn't ship with one. If you ever forget your admin password, you can reset it on the next startup:

- **Binary:** run once with `--admin-password new-password`, then restart normally.
- **Docker:** add `OPENSCANNER_ADMIN_PASSWORD=new-password` to your compose file, run `docker compose up -d --force-recreate`, then remove the line and recreate again.

Either way, the password is consumed at startup — remove the flag or env var afterwards so it isn't sitting in your config.

Once you're logged in, head to **Admin → Systems** to set up your first trunked or conventional system, then **Admin → API Keys** to generate an upload key for your recorder. The [Recorder Guide](recorder-guide.md) takes it from there.

---

## Your Data Directory

The `./data` folder you mounted in the compose file holds everything OpenScanner needs to remember: the SQLite database (`openscanner.db`), the server log file (`openscanner.log`, written alongside the database), and the audio recordings (`recordings/`). Anything else OpenScanner creates — cached transcription models, temporary files — lives inside the container and can be thrown away without losing your data.

Keep that `./data` folder safe, and you can reinstall, upgrade, or move to a new machine without losing anything.

---

## Backing Up

Backups are small and simple. You only need two things:

1. The database file — `data/openscanner.db`
2. The recordings folder — `data/recordings/`

A plain `tar` or `rsync` of the `data/` directory is enough. You can copy it while OpenScanner is running (SQLite's WAL mode handles that safely), but for a tidy point-in-time backup it's better to stop the container first:

```bash
docker compose stop
tar czf openscanner-backup-$(date +%F).tar.gz data/
docker compose start
```

To restore, unpack the archive into the same location and start the container again.

> **Tip:** If you turn on encryption (see [Keeping Secrets Safe](#keeping-secrets-safe)), also back up your `.env` file — without the key, the encrypted entries in your database can't be read.

---

## Running Behind a Reverse Proxy

Most people already have a web server (Caddy, nginx, Traefik) on their home server. Putting OpenScanner behind it gives you a clean domain name and one place to manage TLS certificates.

Two rules to remember when proxying:

- **Forward WebSocket upgrades** on `/api/ws`, `/ws`, and `/api/admin/ws` — the live audio stream and admin events use them. `/api/ws` is the canonical listener endpoint; `/ws` is a compatibility alias kept for legacy clients and should also be proxied.
- **Send `X-Forwarded-Proto`** so OpenScanner knows whether to mark cookies as secure.

If the proxy is on the same machine, it's also a good idea to bind OpenScanner to localhost only so nothing bypasses the proxy. In your compose file:

```yaml
environment:
  - OPENSCANNER_LISTEN=127.0.0.1:3022
ports:
  - "127.0.0.1:3022:3022"
```

### Caddy

Caddy is the easiest — it handles TLS, WebSockets, and forwarded headers on its own.

```caddy
scanner.example.com {
    encode gzip zstd
    reverse_proxy 127.0.0.1:3022
}
```

### nginx

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

After starting the proxy, open the public URL in a browser and confirm the live scanner still plays audio — that's the quickest way to verify the WebSocket forwarding is working.

> **Tip:** Make sure your server's clock is accurate (NTP is usually on by default). Login tokens have expiry times, and a clock that's off by several minutes will cause confusing login failures.

---

## HTTPS Options

You have two choices for serving OpenScanner over HTTPS:

- **Reverse proxy (recommended):** Let Caddy, nginx, or Traefik terminate TLS and forward plain HTTP to OpenScanner on localhost. This is the standard setup for self-hosted apps and gives you a single place to manage certificates.
- **Built-in TLS:** OpenScanner can serve HTTPS directly if you pass it a certificate and key, or ask it to fetch one from Let's Encrypt. This is useful when you don't want to run a separate proxy — for example, on a small VPS that only runs OpenScanner.

For built-in TLS, see [Built-in TLS](#built-in-tls) under Advanced.

---

## Keeping Secrets Safe

OpenScanner stores a few sensitive values in its database: the signing key used for your login sessions and any downstream scanner API keys you configure. By default these are stored as plain text.

You can turn on an encryption option that **scrambles those values in the database file**, so that someone who steals your `openscanner.db` can't read your API keys or forge logins from it. You provide a key when OpenScanner starts, and that key is the only way to unlock the scrambled values.

**Do I need this?** If your OpenScanner is only reachable from your home network and you trust the people on it, you can skip encryption without any real downside. If you're exposing OpenScanner to the internet, or you share the server with other users, turn it on.

### Turning Encryption On (Docker)

1. Generate a random key and save it to a `.env` file next to your `docker-compose.yml`:

   ```bash
   echo "OPENSCANNER_ENCRYPTION_KEY=$(openssl rand -hex 32)" > .env
   chmod 600 .env
   ```

2. Add `.env` to your `.gitignore` if you version-control your compose file:

   ```gitignore
   .env
   ```

3. Reference the variable in your `docker-compose.yml`:

   ```yaml
   services:
     openscanner:
       image: ghcr.io/revtex/openscanner:dev
       environment:
         - OPENSCANNER_ENCRYPTION_KEY=${OPENSCANNER_ENCRYPTION_KEY}
         # ...your other env vars...
   ```

4. Recreate the container so the new variable takes effect:

   ```bash
   docker compose up -d --force-recreate
   ```

On the next startup OpenScanner will encrypt your existing secrets in place and print `Encryption at rest  yes` in its startup banner. You're done.

> **Important:** Back up your `.env` file, or at least the key inside it, somewhere safe. If you lose the key, the scrambled values in the database can't be recovered — you'll need to re-enter your downstream API keys and everyone will need to log in again.

Putting the key directly inside `docker-compose.yml` works too, but then it ends up in whatever copy of that file you share or commit. The `.env` approach keeps them separate.

### What Gets Encrypted

Here's the short list of what changes when encryption is on. Everything else (system names, talkgroup lists, colors, toggles) stays as plain text.

| Value               | Where it lives                  | What it's used for                                       |
| ------------------- | ------------------------------- | -------------------------------------------------------- |
| Login signing key   | `settings` table (`jwtSecret`)  | Signs your login sessions and API tokens                 |
| Downstream API keys | `downstreams` table (`api_key`) | Lets OpenScanner forward calls to another scanner server |

Encrypted entries are prefixed with `enc::` in the database, so if you're poking around in SQLite you can tell which rows are encrypted at a glance.

### If You Don't Set a Key

OpenScanner still starts fine without `OPENSCANNER_ENCRYPTION_KEY` — it just keeps the values above as plain text. On startup it prints a warning in the log letting you know encryption is off, so you don't forget by accident. For a hobby setup on a trusted home network, that's perfectly reasonable. If you later decide to turn it on, just set the variable and restart — OpenScanner will encrypt the existing values on its own.

---

## Transcription (Optional)

OpenScanner can automatically transcribe calls using [go-whisper](https://github.com/mutablelogic/go-whisper), a whisper.cpp sidecar that runs as its own service.

Add this alongside OpenScanner in your `docker-compose.yml`:

```yaml
whisper:
  image: ghcr.io/mutablelogic/go-whisper
  volumes:
    - whisper-data:/data
  environment:
    - GOWHISPER_DIR=/data
    - GOWHISPER_ADDR=0.0.0.0:8081
  command: ["run"]
  restart: unless-stopped
```

Then in OpenScanner's admin dashboard, open **Admin → Transcription** and:

1. Set **Transcription URL** to `http://whisper:8081`.
2. **Download a model** — pick one from the list and click download.
3. **Select the model** you just downloaded as the active model.
4. Set **Language** (default `en`, or leave blank to auto-detect).
5. Turn **Transcription Enabled** on.

Available models:

| Model                 | Notes                                                   |
| --------------------- | ------------------------------------------------------- |
| `ggml-tiny`           | Fastest, lowest accuracy (multilingual)                 |
| `ggml-tiny.en`        | Fastest, English-only                                   |
| `ggml-base`           | Good balance (multilingual)                             |
| `ggml-base.en`        | Good balance, English-only                              |
| `ggml-small`          | Better accuracy, slower (multilingual)                  |
| `ggml-small.en`       | Better accuracy, English-only                           |
| `ggml-medium`         | High accuracy, needs more resources (multilingual)      |
| `ggml-medium.en`      | High accuracy, English-only                             |
| `ggml-large-v3`       | Best accuracy, most resource-heavy                      |
| `ggml-large-v3-turbo` | Near-best accuracy, faster than large-v3                |
| `ggml-small.en-tdrz`  | Enables speaker diarization (identifies who is talking) |

Once transcription is on, transcribed text appears in the live player and is searchable from the **Search** page.

### GPU Acceleration (Highly Recommended)

CPU-only transcription is very slow — on a typical home CPU, calls may not finish processing in time for the live player to show the transcript. A GPU with at least **6 GB of VRAM** (for example an NVIDIA RTX 3050 6GB) lets you run `ggml-large-v3-turbo` with good accuracy at close to real-time speed.

Options:

- **NVIDIA CUDA** — use the `ghcr.io/mutablelogic/go-whisper-cuda` image with GPU device passthrough
- **Intel iGPU** — mount `/dev/dri` with the right group IDs for Vulkan/OpenCL
- **AMD ROCm** — mount ROCm devices

The project's [docker-compose.yml](../docker-compose.yml) has commented-out examples for each. For more detail, see the [go-whisper repository](https://github.com/mutablelogic/go-whisper) — note that go-whisper is a third-party project and OpenScanner doesn't provide support for it directly.

---

## FFmpeg (Optional)

FFmpeg handles audio conversion and normalization. It's **already installed in the Docker image**, so you don't need to do anything unless you're running from a binary.

In **Admin → Options** you can pick a conversion mode:

- **Disabled** — store audio files as-is
- **Enabled** — basic codec conversion
- **Normalize** — conversion with compression
- **Loudnorm** — conversion with loudness normalization (good for consistent volume across systems)

---

## Verification Checklist

After deploying, check these to confirm everything works:

- [ ] `curl http://localhost:3022/api/health` returns a 200 response
- [ ] The browser URL shows the scanner interface (or the setup page on first run)
- [ ] Admin login works and the dashboard loads
- [ ] A test upload from your recorder appears in OpenScanner
- [ ] The live scanner feed plays audio (this proves WebSockets are working)

---

## Advanced

Everything below is for less-common setups: binary installs, CLI flags, externally-managed secrets, and network hardening. Most users won't need any of it.

### Binary Install

OpenScanner also ships as a single executable for Linux, macOS, and Windows — no Docker, no external database.

#### Guided Setup

The easiest way is the built-in setup command, which creates directories, writes a config file, and installs a system service.

```bash
sudo ./openscanner setup --interactive
```

It asks for:

- Listen address
- Database file path
- Recordings directory
- Config file location
- Install path for the binary

Once it's done, OpenScanner is running as a system service. Open the listen address in your browser to finish setup.

To accept platform defaults without prompting:

```bash
sudo ./openscanner setup
```

#### Manual Run

If you'd rather just run it without installing a service:

```bash
./openscanner --listen 0.0.0.0:3022 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

#### Platform Defaults

When you use `openscanner setup`, paths are chosen for your OS:

**Linux:**

| Setting    | Default                                 |
| ---------- | --------------------------------------- |
| Config     | `/etc/openscanner/openscanner.json`     |
| Database   | `/var/lib/openscanner/openscanner.db`   |
| Recordings | `/var/lib/openscanner/recordings`       |
| Executable | `/usr/local/bin/openscanner`            |
| Service    | systemd / SysV / OpenRC (auto-detected) |

**macOS:**

| Setting    | Default                                         |
| ---------- | ----------------------------------------------- |
| Config     | `/usr/local/etc/openscanner/openscanner.json`   |
| Database   | `/usr/local/var/lib/openscanner/openscanner.db` |
| Recordings | `/usr/local/var/lib/openscanner/recordings`     |
| Executable | `/usr/local/bin/openscanner`                    |
| Service    | launchd                                         |

**Windows:**

| Setting    | Default                                      |
| ---------- | -------------------------------------------- |
| Config     | `%ProgramData%\OpenScanner\openscanner.json` |
| Database   | `%ProgramData%\OpenScanner\openscanner.db`   |
| Recordings | `%ProgramData%\OpenScanner\recordings`       |
| Executable | `%ProgramFiles%\OpenScanner\openscanner.exe` |
| Service    | Windows Service Control Manager              |

You can override any of these with flags:

```bash
openscanner setup \
  --listen 0.0.0.0:3022 \
  --db-file /opt/openscanner/data.db \
  --recordings-dir /opt/openscanner/recordings \
  --config /opt/openscanner/config.json \
  --install-binary /opt/openscanner/openscanner
```

### Service Management

After `openscanner setup`, the following commands manage the installed service:

| Command                                     | What it does                                                     |
| ------------------------------------------- | ---------------------------------------------------------------- |
| `openscanner setup`                         | Full install (create dirs, write config, install service, start) |
| `openscanner setup --interactive`           | Same, with interactive prompts                                   |
| `openscanner setup --force`                 | Overwrite existing setup / reinstall service                     |
| `openscanner upgrade --binary /path/to/new` | Replace the installed binary and restart the service             |
| `openscanner config validate`               | Check your JSON config file for errors                           |
| `openscanner service doctor`                | Print service status and diagnostics                             |

For direct control:

```bash
openscanner --service install --config /path/to/openscanner.json
openscanner --service start
openscanner --service stop
openscanner --service restart
openscanner --service uninstall
```

#### Upgrading

```bash
curl -L -o /tmp/openscanner-new https://github.com/revtex/OpenScanner/releases/latest/...
openscanner upgrade --binary /tmp/openscanner-new
```

If the service was stopped before upgrading, it stays stopped afterwards.

### Configuration Reference

OpenScanner reads settings from three places, in this priority order:

**CLI flags > environment variables > JSON config file > built-in defaults**

Docker users will almost always use environment variables; binary users typically use the JSON config file written by `openscanner setup`.

#### CLI Flags

| Flag                    | Description                                               | Default                |
| ----------------------- | --------------------------------------------------------- | ---------------------- |
| `--listen`              | HTTP listen address                                       | `:3022`                |
| `--db-file`             | SQLite database file path                                 | `openscanner.db`       |
| `--recordings-dir`      | Directory for audio recordings                            | (executable directory) |
| `--ssl-listen`          | HTTPS listen address                                      | (disabled)             |
| `--ssl-cert`            | TLS certificate file (PEM)                                |                        |
| `--ssl-key`             | TLS private key file (PEM)                                |                        |
| `--ssl-auto-cert`       | Domain for Let's Encrypt auto-cert                        |                        |
| `--encryption-key`      | Key for encrypting secrets at rest                        |                        |
| `--encryption-key-file` | Path to a file containing the encryption key              |                        |
| `--timezone`            | IANA timezone for recorder timestamps                     | `UTC`                  |
| `--admin-password`      | Reset the first admin user's password on startup          |                        |
| `--config`              | Path to JSON config file                                  | `openscanner.json`     |
| `--config-save`         | Write current flags to JSON config and exit               |                        |
| `--version`             | Print version and exit                                    |                        |
| `--service`             | Service command: install, uninstall, start, stop, restart |                        |

#### Environment Variables

| Variable                          | Maps to                 |
| --------------------------------- | ----------------------- |
| `OPENSCANNER_LISTEN`              | `--listen`              |
| `OPENSCANNER_DB_FILE`             | `--db-file`             |
| `OPENSCANNER_RECORDINGS_DIR`      | `--recordings-dir`      |
| `OPENSCANNER_SSL_LISTEN`          | `--ssl-listen`          |
| `OPENSCANNER_SSL_CERT`            | `--ssl-cert`            |
| `OPENSCANNER_SSL_KEY`             | `--ssl-key`             |
| `OPENSCANNER_SSL_AUTO_CERT`       | `--ssl-auto-cert`       |
| `OPENSCANNER_ENCRYPTION_KEY`      | `--encryption-key`      |
| `OPENSCANNER_ENCRYPTION_KEY_FILE` | `--encryption-key-file` |
| `OPENSCANNER_ADMIN_PASSWORD`      | `--admin-password`      |
| `OPENSCANNER_TIMEZONE`            | `--timezone`            |
| `TZ`                              | `--timezone` (fallback) |

#### Env-Only Settings

A couple of toggles don't have matching CLI flags or JSON fields — they only exist as environment variables.

| Variable                          | Description                                                                                                                                                                                                                     | Default |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- |
| `OPENSCANNER_BLOCK_INTERNAL_HTTP` | When set to `1`, `true`, or `yes`, OpenScanner refuses outbound HTTP (transcription, downstream push) to private-network, loopback, link-local, and multicast addresses. Off by default so whisper and LAN scanners still work. | unset   |
| `OPENSCANNER_JWT_SECRET`          | Lets you supply the login-session signing key yourself instead of having OpenScanner auto-generate one. See [Externalizing the Login Signing Key](#externalizing-the-login-signing-key) below.                                  | unset   |

#### JSON Config File

You can save your settings to a JSON file so you don't need to pass flags every time:

```bash
openscanner --listen 0.0.0.0:3022 --db-file /data/openscanner.db --config-save
```

That produces:

```json
{
  "listen": "0.0.0.0:3022",
  "db_file": "/data/openscanner.db",
  "recordings_dir": "/data/recordings",
  "ssl_listen": "",
  "ssl_cert_file": "",
  "ssl_key_file": "",
  "ssl_auto_cert": "",
  "encryption_key": "",
  "timezone": ""
}
```

Temporary flags (`--admin-password`, `--config-save`, `--version`, `--service`) are never written to the file. `--encryption-key-file` is also not persisted — only the resolved `encryption_key` value appears in the JSON.

### Built-in TLS

OpenScanner can serve HTTPS itself in two ways.

#### With Your Own Certificate

```bash
openscanner --ssl-listen :443 --ssl-cert /path/to/cert.pem --ssl-key /path/to/key.pem
```

#### Automatic Let's Encrypt (Experimental)

> **Warning:** This is implemented but hasn't been tested widely in production. For reliable TLS, a reverse proxy like Caddy is the safer bet.

```bash
openscanner --ssl-auto-cert scanner.example.com
```

How it works:

1. OpenScanner uses Go's `autocert` library to talk to Let's Encrypt.
2. Let's Encrypt verifies you control the domain by fetching a file from `http://your-domain/.well-known/acme-challenge/...`. OpenScanner's HTTP listener answers automatically.
3. Once verified, a certificate is issued and OpenScanner serves HTTPS on port 443.
4. Certificates are cached in `autocert-cache/` next to the binary and renewed automatically.

Requirements:

- **Port 80** must be reachable from the internet (for the verification challenge)
- **Port 443** must be reachable (for HTTPS)
- **DNS** must point the domain to the server's public IP
- The domain passed to `--ssl-auto-cert` must match the DNS record exactly

In both TLS modes, non-challenge HTTP traffic is redirected to HTTPS.

#### Adding TLS After Setup

`openscanner setup` doesn't configure TLS — it only writes listen address, database path, and recordings directory. To add TLS afterwards, either edit your JSON config file directly:

```json
{
  "listen": ":3022",
  "db_file": "/var/lib/openscanner/openscanner.db",
  "recordings_dir": "/var/lib/openscanner/recordings",
  "ssl_listen": ":443",
  "ssl_cert_file": "/path/to/cert.pem",
  "ssl_key_file": "/path/to/key.pem"
}
```

Or merge new flags into the existing config with `--config-save`:

```bash
openscanner --config /etc/openscanner/openscanner.json \
  --ssl-listen :443 \
  --ssl-cert /path/to/cert.pem \
  --ssl-key /path/to/key.pem \
  --config-save
```

Then restart the service:

```bash
openscanner --service restart
```

The installed service reads `--config <path>` on every start, so no reinstall is needed.

### Externalizing the Login Signing Key

OpenScanner signs login sessions and API tokens with a secret it auto-generates on first startup and stores in the database. If you have your own secret-management setup (Kubernetes secrets, Vault, a `.env` file you already use for other services), you can supply the key yourself with `OPENSCANNER_JWT_SECRET`.

**Binary / shell:**

```bash
export OPENSCANNER_JWT_SECRET="$(openssl rand -hex 32)"
```

**Docker Compose:** add the key to your `.env` file alongside `OPENSCANNER_ENCRYPTION_KEY`, then reference it in `docker-compose.yml`:

```bash
echo "OPENSCANNER_JWT_SECRET=$(openssl rand -hex 32)" >> .env
```

```yaml
environment:
  - OPENSCANNER_JWT_SECRET=${OPENSCANNER_JWT_SECRET}
```

When set:

- OpenScanner uses this value and never reads or writes the database's `jwt_secret` setting.
- The encryption key only protects downstream API keys in that case (you're managing the session key yourself).
- Rotating the secret means changing the variable and restarting — existing sessions are invalidated and everyone logs in again.

For most setups you don't need this — the default (stored in the DB, encrypted if you set an encryption key) works fine.

### Blocking Outbound Traffic to Private Networks

By default, OpenScanner's outbound HTTP (to your transcription sidecar, downstream scanners, webhook targets) is allowed to reach private network addresses. That's what lets `http://whisper:8081` and other LAN services work.

If you're in a more locked-down environment and want to block outbound traffic to private, loopback, link-local, and multicast addresses, set:

```yaml
environment:
  - OPENSCANNER_BLOCK_INTERNAL_HTTP=1
```

Note that this will also block a whisper sidecar running on the same host, so only turn it on if all your downstream targets are on the public internet.

### Build from Source

#### Requirements

- Go 1.25+
- Node.js 22+ with pnpm
- Make

#### Build

```bash
make build
```

This builds the frontend, embeds it into the Go binary, and writes `build/openscanner`.

#### Development

```bash
make dev    # Hot-reload backend (air) + Vite dev server with proxy
make test   # Run all backend + frontend tests
make lint   # Run linters
```
