# Deployment Guide

This guide covers everything you need to get OpenScanner running — from Docker to bare-metal installs, TLS setup, and reverse proxies.

## Contents

- [Docker (Recommended)](#docker-recommended)
- [Binary Install](#binary-install)
- [Configuration](#configuration)
- [Service Management](#service-management)
- [TLS / HTTPS](#tls--https)
- [Secrets Encryption](#secrets-encryption)
- [Reverse Proxy](#reverse-proxy)
- [Transcription (Optional)](#transcription-optional)
- [FFmpeg (Optional)](#ffmpeg-optional)
- [First-Run Setup](#first-run-setup)
- [Build from Source](#build-from-source)
- [Verification Checklist](#verification-checklist)

---

## Docker (Recommended)

Docker is the easiest way to get started. OpenScanner provides a pre-built image with FFmpeg included.

### Quick Start

1. Create a directory for your data:

   ```bash
   mkdir -p openscanner/data
   cd openscanner
   ```

2. Create a `docker-compose.yml`:

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
         - TZ=America/New_York # Set to your timezone
       healthcheck:
         test: ["CMD", "wget", "-qO-", "http://localhost:3022/api/health"]
         interval: 30s
         timeout: 5s
         start_period: 10s
         retries: 3
       restart: unless-stopped
   ```

3. Start the container:

   ```bash
   docker compose up -d
   ```

4. Open `http://localhost:3022` in your browser. You'll be guided through initial setup to create your admin account.

> **Tip:** Set the `TZ` environment variable to your local timezone (IANA format, e.g. `America/New_York`) so that recorder timestamps are interpreted correctly.

### Building the Image Locally

If you prefer to build from source instead of using the pre-built image:

```bash
docker compose build
docker compose up -d
```

The Dockerfile uses a multi-stage build: Node.js builds the frontend, Go compiles the backend with the frontend embedded, and the final image is based on Alpine Linux with FFmpeg pre-installed.

---

## Binary Install

OpenScanner can run as a standalone binary on Linux, macOS, or Windows. No external database is needed — SQLite is embedded.

### Guided Setup (Recommended)

The setup command handles everything: creating directories, writing a config file, and installing a system service.

```bash
sudo ./openscanner setup --interactive
```

This walks you through prompts for:

- Listen address
- Database file path
- Recordings directory
- Config file location
- Install path for the binary

Once complete, OpenScanner starts as a system service. Open the listen address in your browser to finish setup.

To use platform defaults without prompts:

```bash
sudo ./openscanner setup
```

### Manual Run

If you'd rather run OpenScanner directly without installing a service:

```bash
./openscanner --listen 0.0.0.0:3022 --db-file ./data/openscanner.db --recordings-dir ./data/recordings
```

Open `http://localhost:3022` to complete initial setup.

### Platform Defaults

When using `openscanner setup`, paths are set automatically based on your OS:

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

You can override any default with setup flags:

```bash
openscanner setup \
  --listen 0.0.0.0:3022 \
  --db-file /opt/openscanner/data.db \
  --recordings-dir /opt/openscanner/recordings \
  --config /opt/openscanner/config.json \
  --install-binary /opt/openscanner/openscanner
```

---

## Configuration

OpenScanner reads configuration from three sources, in this priority order:

**CLI flags > environment variables > JSON config file > built-in defaults**

### CLI Flags

| Flag                    | Description                                               | Default                |
| ----------------------- | --------------------------------------------------------- | ---------------------- |
| `--listen`              | HTTP listen address                                       | `:3022`                |
| `--db-file`             | SQLite database file path                                 | `openscanner.db`       |
| `--recordings-dir`      | Directory for audio recordings                            | (executable directory) |
| `--ssl-listen`          | HTTPS listen address                                      | (disabled)             |
| `--ssl-cert`            | TLS certificate file (PEM)                                |                        |
| `--ssl-key`             | TLS private key file (PEM)                                |                        |
| `--ssl-auto-cert`       | Domain for Let's Encrypt auto-cert                        |                        |
| `--encryption-key`      | AES-256 key for encrypting secrets at rest                |                        |
| `--encryption-key-file` | Path to file containing encryption key                    |                        |
| `--timezone`            | IANA timezone for recorder timestamps                     | `UTC`                  |
| `--admin-password`      | Reset first admin user's password on startup              |                        |
| `--config`              | Path to JSON config file                                  | `openscanner.json`     |
| `--config-save`         | Write current flags to JSON config and exit               |                        |
| `--version`             | Print version and exit                                    |                        |
| `--service`             | Service command: install, uninstall, start, stop, restart |                        |

### Environment Variables

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

### JSON Config File

You can save your settings to a JSON file so you don't need to pass flags every time:

```bash
openscanner --listen 0.0.0.0:3022 --db-file /data/openscanner.db --config-save
```

This writes a file like:

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

Temporary flags (`--admin-password`, `--config-save`, `--version`, `--service`) are never written to the config file. The `--encryption-key-file` flag is also not persisted — only `encryption_key` appears in the JSON.

---

## Service Management

After running `openscanner setup`, use these commands to manage the service:

| Command                                     | What it does                                                     |
| ------------------------------------------- | ---------------------------------------------------------------- |
| `openscanner setup`                         | Full install (create dirs, write config, install service, start) |
| `openscanner setup --interactive`           | Same as above with interactive prompts                           |
| `openscanner setup --force`                 | Overwrite existing setup / reinstall service                     |
| `openscanner upgrade --binary /path/to/new` | Replace installed binary and restart the service                 |
| `openscanner config validate`               | Check your JSON config file for errors                           |
| `openscanner service doctor`                | Print service status and diagnostics                             |

For manual service control:

```bash
openscanner --service install --config /path/to/openscanner.json
openscanner --service start
openscanner --service stop
openscanner --service restart
openscanner --service uninstall
```

### Upgrading

To upgrade to a new version:

```bash
# Download the new binary
curl -L -o /tmp/openscanner-new https://github.com/revtex/OpenScanner/releases/latest/...

# Upgrade (stops service → replaces binary → restarts)
openscanner upgrade --binary /tmp/openscanner-new
```

If the service was stopped before upgrading, it stays stopped after.

---

## TLS / HTTPS

OpenScanner supports TLS in two ways.

### Certificate Files

If you already have a certificate and key:

```bash
openscanner --ssl-listen :443 --ssl-cert /path/to/cert.pem --ssl-key /path/to/key.pem
```

### Automatic Let's Encrypt (Experimental)

> **Warning:** This feature is implemented but has not been tested in production. Use at your own risk. For reliable TLS, consider using certificate files directly or a reverse proxy like Caddy.

If your server is publicly accessible, OpenScanner can obtain and renew TLS certificates automatically using Let's Encrypt.

```bash
openscanner --ssl-auto-cert scanner.example.com
```

**How it works:**

1. OpenScanner creates an ACME client using Go's `autocert` library.
2. When Let's Encrypt needs to verify you control the domain, it sends an HTTP request to `http://your-domain/.well-known/acme-challenge/...`. OpenScanner's HTTP listener handles this automatically.
3. Once verified, Let's Encrypt issues a certificate and OpenScanner starts serving HTTPS on port 443.
4. Certificates are cached locally in an `autocert-cache/` directory (relative to the working directory) and renewed automatically before they expire.

**Requirements:**

- **Port 80** must be reachable from the internet (for the ACME HTTP-01 challenge)
- **Port 443** must be reachable (for HTTPS traffic)
- **DNS** must point your domain to the server's public IP
- The domain passed to `--ssl-auto-cert` must match the DNS record exactly

In both TLS modes, the HTTP listener automatically redirects all non-challenge traffic to HTTPS.

### Adding TLS After Initial Setup

The `openscanner setup` command does not configure TLS — it only sets the listen address, database path, and recordings directory. If you ran setup first and want to add TLS later, edit the JSON config file (default: `openscanner.json`) and add the SSL fields:

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

Or use `--config-save` to merge new flags into your existing config file without editing JSON by hand:

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

The installed service only passes `--config <path>` to OpenScanner, so all settings — including TLS — are read from the config file on every start. No reinstall is needed.

---

## Secrets Encryption

OpenScanner can encrypt sensitive values stored in the database (VAPID private key, downstream API keys) using AES-256-GCM. This protects secrets if the database file is compromised.

### Enabling Encryption

First, generate a strong random key:

```bash
# Option 1: 32-byte hex string (64 characters)
openssl rand -hex 32

# Option 2: 32-byte base64 string
openssl rand -base64 32

# Option 3: if openssl is not available
head -c 32 /dev/urandom | xxd -p -c 64
```

Save the output — you will need it every time the server starts. If you lose the key, encrypted secrets cannot be recovered and must be re-entered.

Provide the key via any config source:

```bash
# CLI flag
openscanner --encryption-key "your-secret-key-here"

# Environment variable
export OPENSCANNER_ENCRYPTION_KEY="your-secret-key-here"

# JSON config file
{"encryption_key": "your-secret-key-here"}

# Read key from a file (Docker secrets / Kubernetes)
openscanner --encryption-key-file /run/secrets/encryption_key
```

### What Gets Encrypted

| Value               | Location            | Purpose                         |
| ------------------- | ------------------- | ------------------------------- |
| VAPID private key   | `settings` table    | Signs web push notifications    |
| Downstream API keys | `downstreams` table | Authenticates to remote servers |

Encrypted values are stored with an `enc::` prefix followed by base64-encoded ciphertext.

### How It Works

- On first startup with a key: existing plaintext secrets are automatically encrypted in place
- On every read: encrypted values are decrypted transparently where needed (e.g. downstream forwarding uses the decrypted key to authenticate)
- The admin UI never displays secret values — API keys are shown as masked dots
- If the key is removed while encrypted values exist: the server refuses to start with a clear error message
- If the wrong key is provided: the server refuses to start with a decrypt error

### Docker Compose Example

```yaml
services:
  openscanner:
    image: ghcr.io/revtex/openscanner:dev
    environment:
      - OPENSCANNER_ENCRYPTION_KEY=change-me-to-a-strong-random-value
```

Or with Docker secrets:

```yaml
services:
  openscanner:
    image: ghcr.io/revtex/openscanner:dev
    environment:
      - OPENSCANNER_ENCRYPTION_KEY_FILE=/run/secrets/encryption_key
    secrets:
      - encryption_key

secrets:
  encryption_key:
    file: ./encryption_key.txt
```

### Without Encryption

If no key is configured, secrets are stored in plaintext and a warning is logged at startup. Everything works normally — encryption is optional.

---

## Reverse Proxy

If you already run a web server (Nginx, Caddy, etc.), you can put OpenScanner behind it for centralized TLS and routing.

**Important requirements:**

- WebSocket upgrade headers must be forwarded for `/ws` and `/api/admin/ws`
- `X-Forwarded-Proto` must be set so OpenScanner can issue secure cookies correctly
- Bind OpenScanner to localhost when proxied: `--listen 127.0.0.1:3022`

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

**Notes:**

- `X-Forwarded-Proto $scheme` is required so OpenScanner sets secure cookies correctly.
- The long `proxy_read_timeout` keeps WebSocket connections alive.
- Increase `client_max_body_size` if you upload large files (audio imports, CSV imports).

### Caddy

Caddy automatically handles TLS, WebSocket upgrades, and forwarded headers:

```caddy
scanner.example.com {
    encode gzip zstd
    reverse_proxy 127.0.0.1:3022
}
```

### Proxy Tips

- Bind OpenScanner to `127.0.0.1:3022` when fronted by a reverse proxy so it's not directly accessible.
- Keep clocks synchronized (NTP) so JWT tokens and cookie expiry work correctly.
- Test both WebSocket endpoints after deployment: `/ws` (listener) and `/api/admin/ws` (admin).

---

## Transcription (Optional)

OpenScanner can automatically transcribe calls using [go-whisper](https://github.com/mutablelogic/go-whisper), a whisper.cpp HTTP sidecar. It runs as a separate service and OpenScanner communicates with it over HTTP.

Add the whisper service to your `docker-compose.yml`:

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

Then in OpenScanner's admin dashboard, go to **Admin → Transcription** and:

1. Set the **Transcription URL** to `http://whisper:8081`
2. **Download a model** — pick one from the model list and click download. Available models:

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

3. **Select the model** you downloaded as the active model
4. Set the **language** (default: `en`) or leave blank for auto-detect
5. Turn **Transcription Enabled** on

> **Note:** Transcription won't work until a model is downloaded and selected. The admin panel shows which models are available on disk. You can download multiple models and switch between them at any time.

Once transcription is active, transcribed text appears in the live scanner player and can be viewed and searched from the **Search** page.

### GPU Acceleration (Highly Recommended)

CPU-only transcription is very slow. On CPU, calls may not finish processing in time for live player transcription to display correctly. GPU acceleration is strongly recommended for any practical use.

A GPU with at least **6 GB of VRAM** is recommended — for example, an NVIDIA GeForce RTX 3050 6GB is a solid budget option. With a GPU, the `ggml-large-v3-turbo` model delivers the best transcription results at reasonable speed.

Available GPU options:

- **NVIDIA CUDA** — use `ghcr.io/mutablelogic/go-whisper-cuda` image with GPU device passthrough
- **Intel iGPU** — mount `/dev/dri` with appropriate group IDs for Vulkan/OpenCL
- **AMD ROCm** — mount ROCm devices with appropriate configuration

See the commented examples in the project's [docker-compose.yml](../docker-compose.yml) for ready-to-use GPU configurations.

For more details on go-whisper setup, supported models, and GPU configuration, see the [go-whisper repository](https://github.com/mutablelogic/go-whisper). Note that go-whisper is a third-party project — OpenScanner does not provide support for go-whisper setup or troubleshooting.

---

## FFmpeg (Optional)

FFmpeg is used for audio format conversion and normalization. It's pre-installed in the Docker image. For binary installs, install FFmpeg separately and make sure it's on your PATH.

OpenScanner supports four conversion modes (configurable in **Admin → Options**):

- **Disabled** — store audio files as-is
- **Enabled** — basic codec conversion
- **Normalize** — conversion with compression filter
- **Loudnorm** — conversion with loudness normalization

FFmpeg is invoked safely using argument arrays (no shell execution).

---

## First-Run Setup

After starting OpenScanner for the first time:

1. Open the listen address in your browser (e.g. `http://localhost:3022`).
2. You'll be redirected to `/setup` to create your first admin account.
3. Enter a username and password, then click **Create**.
4. Log in with your new credentials.
5. Head to **Admin → Systems** to create your first system, or enable **Auto-Populate Systems** to have them created automatically from incoming calls.
6. Get calls flowing — choose one or both methods:
   - **API Upload** — create an **API Key** in **Admin → API Keys** and configure your recorder to upload calls over HTTP (see the [Recorder Guide](recorder-guide.md)).
   - **Directory Monitor** — set up a watch in **Admin → Dir Monitors** to have OpenScanner automatically import calls from a local directory (useful when the recorder writes files to a shared folder).

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

This builds the frontend, embeds it into the Go binary, and outputs `build/openscanner`.

### Development

```bash
make dev    # Hot-reload backend (air) + Vite dev server with proxy
make test   # Run all backend + frontend tests
make lint   # Run linters
```

---

## Verification Checklist

After deploying, verify everything is working:

- [ ] `curl http://localhost:3022/api/health` returns a 200 response
- [ ] Opening the URL in a browser shows the scanner interface or setup page
- [ ] Admin login works and the dashboard loads
- [ ] A test call upload from your recorder appears in OpenScanner
- [ ] The live scanner feed plays audio over WebSocket
- [ ] `openscanner service doctor` reports the service is running (binary installs)
- [ ] `openscanner config validate` passes
