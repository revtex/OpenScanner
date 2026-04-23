# Admin Guide

This guide covers every panel in the OpenScanner admin dashboard — what each setting does, how panels work, and how to configure your system.

The admin dashboard is at `/admin` and requires signing in with an admin account.

## Contents

- [Navigation](#navigation)
- [Activity](#activity)
- [Users](#users)
- [Systems](#systems)
- [Groups & Tags](#groups--tags)
- [API Keys](#api-keys)
- [Monitors (Directory Monitors)](#monitors-directory-monitors)
- [Downstreams](#downstreams)
- [Shared Links](#shared-links)
- [Transcription](#transcription)
- [Options](#options)
- [Logs](#logs)
- [Tools](#tools)
- [Webhooks (Preview)](#webhooks-preview)

---

## Navigation

The sidebar contains these panels, in order:

1. **Activity** — dashboard overview and stats
2. **Users** — manage user accounts
3. **Systems** — manage systems, talkgroups, and units
4. **Groups & Tags** — organize talkgroups into categories
5. **API Keys** — manage recorder upload keys
6. **Monitors** — set up directory-based call import
7. **Downstreams** — forward calls to other OpenScanner instances
8. **Shared Links** — view and manage shared call links
9. **Transcription** — configure speech-to-text
10. **Options** — general settings and behavior
11. **Logs** — view server logs
12. **Tools** — import, export, and maintenance

The **Scanner** link in the sidebar returns you to the live scanner at `/`. **Sign Out** clears your session. If you have unsaved changes in a panel, you'll be prompted before navigating away.

---

## Activity

The Activity panel gives you a quick overview of your system:

- **Calls Today** — number of calls ingested today
- **This Week** — calls over the last 7 days
- **Total Calls** — all-time call count
- **Active Listeners** — currently connected scanner clients
- **Server Uptime** — how long the server has been running
- **24-Hour Activity Chart** — visual breakdown of call volume by hour
- **Top Talkgroups** — most active talkgroups

---

## Users

Manage who can access OpenScanner.

Each user has:

| Field            | Description                                                     |
| ---------------- | --------------------------------------------------------------- |
| Username         | Login name                                                      |
| Password         | Set on create; leave blank when editing to keep unchanged       |
| Role             | **Admin** (full access) or **Listener** (scanner only)          |
| Disabled         | Temporarily block access without deleting the account           |
| Expiration       | Optional date after which the account is locked out             |
| Connection Limit | Optional cap on simultaneous sessions for this user             |
| System Selection | Optional — restrict which systems/talkgroups this user can hear |

The first admin account cannot be disabled.

---

## Systems

Systems represent your radio systems (e.g. a county trunked system, a conventional channel group). Each system contains **talkgroups** and **units**.

### System Settings

- **Label** — display name shown in the scanner
- **Order** — controls display position (lower numbers appear first)
- **TG Auto-Populate** — when enabled, talkgroups are automatically created from incoming calls for this system. If the incoming call includes additional metadata (talkgroup name, label, group, tag, or unit information), those are created or updated automatically as well.

### Talkgroups

Each talkgroup has:

| Field        | Description                                      |
| ------------ | ------------------------------------------------ |
| Talkgroup ID | Numeric identifier matching your recorder output |
| Label        | Short display label (e.g. "FD Dispatch")         |
| Name         | Longer descriptive name                          |
| Frequency    | Optional frequency in Hz                         |
| Group        | Category grouping (from Groups panel)            |
| Tag          | Classification tag (from Tags panel)             |

You can search talkgroups by ID, label, or name. Large lists are virtualized for performance.

### Units

Each unit has a **Unit ID** and **Label**. Units represent individual radios on a system. You can search by ID or label.

---

## Groups & Tags

Groups and tags organize your talkgroups into categories.

- **Groups** categorize talkgroups by function (e.g. Fire, Law, EMS, Public Works)
- **Tags** provide finer classification (e.g. Law Dispatch, Law Tac, Fire Tac, Emergency Ops)

Both are simple label lists. Assign them to talkgroups in the Systems panel.

OpenScanner ships with sensible defaults (Air, Common, EMS, Fire, Interop, Law, Public Works for groups; ~20 tags covering law, fire, EMS, corrections, and more).

---

## API Keys

API keys authenticate recorders that upload calls to OpenScanner over HTTP.

Each key has:

| Field       | Description                                                                   |
| ----------- | ----------------------------------------------------------------------------- |
| Fingerprint | Auto-generated unique identifier (read-only)                                  |
| Label       | Optional friendly name                                                        |
| Disabled    | Temporarily stop accepting uploads from this key                              |
| Systems     | Restrict which systems this key can upload to (empty = all)                   |
| Rate Limit  | Optional per-key call rate limit (calls/minute); overrides the global default |
| Order       | Display position                                                              |

Copy the full API key when creating it — it's shown once. See the [Recorder Guide](recorder-guide.md) for how to configure your recorder with the key.

---

## Monitors (Directory Monitors)

Directory monitors watch a local folder and automatically import call audio files. This is an alternative to API-based upload — useful when your recorder writes files to a shared directory.

Each monitor has:

| Field              | Description                                                                                        |
| ------------------ | -------------------------------------------------------------------------------------------------- |
| Directory          | Folder path to watch (browseable from the UI)                                                      |
| Type               | Recorder type: **Default**, **DSDPlus**, **SDR-Trunk**, or **Trunk-Recorder**                      |
| Mask               | Filename pattern using tokens to extract metadata (see below)                                      |
| Extension          | File extension filter (e.g. `wav`, `mp3`)                                                          |
| Delay              | Wait time in milliseconds before processing a new file (allows writes to complete)                 |
| Use Polling        | Use polling instead of filesystem events (for network drives or mounts that don't support inotify) |
| Delete After       | Remove the source file after successful import                                                     |
| System Override    | Force all files to a specific system                                                               |
| Talkgroup Override | Force all files to a specific talkgroup                                                            |
| Frequency          | Optional frequency override in Hz                                                                  |
| Disabled           | Temporarily stop watching                                                                          |
| Order              | Display position                                                                                   |

### Filename Mask Tokens

The mask extracts metadata from filenames. Available tokens:

`#DATE`, `#TIME`, `#SYS`, `#TG`, `#HZ`, `#GROUP`, `#TAG`, `#UNIT`

Example: a file named `2025-01-15_143022_101_5200.wav` with mask `#DATE_#TIME_#SYS_#TG` would extract the date, time, system 101, and talkgroup 5200.

The UI includes a help section with the full token reference.

---

## Downstreams

> **Note:** Downstream forwarding is implemented but has not been tested. Use at your own risk.

Downstreams forward ingested calls to other OpenScanner instances. Use this to fan out from a central server to regional or public-facing instances.

Each downstream has:

| Field    | Description                                     |
| -------- | ----------------------------------------------- |
| URL      | Remote server's call-upload endpoint            |
| API Key  | Authentication key for the remote server        |
| Systems  | Restrict which systems to forward (empty = all) |
| Disabled | Temporarily stop forwarding                     |
| Order    | Display position                                |

API keys are encrypted at rest in the database when an [encryption key](deployment-guide.md#secrets-encryption) is configured. The admin UI never displays API keys — they are shown as masked dots. To change a key, enter a new one in the edit form; leave it blank to keep the existing key.

---

## Shared Links

Lists all shared call links created by users. Each entry shows:

- System and talkgroup
- Call date and duration
- Who shared it and when
- Expiration date (or "Never")

You can delete shared links from here. Deleting a link makes the call eligible for normal pruning.

Shared link creation and expiry are controlled in **Options → Sharing & Notifications**.

---

## Transcription

Configure automatic speech-to-text for calls. Transcription uses a separate [go-whisper](https://github.com/mutablelogic/go-whisper) sidecar service — see the [Deployment Guide](deployment-guide.md#transcription-optional) for setup instructions.

This panel shows a connection status indicator and provides these controls:

| Setting                 | Description                                                         |
| ----------------------- | ------------------------------------------------------------------- |
| Transcription Enabled   | Master on/off toggle                                                |
| Live Transcript Display | Show transcription text in the live scanner player                  |
| Transcription URL       | Address of the go-whisper server (default: `http://localhost:8081`) |
| Language                | Target language or auto-detect (15 languages supported)             |
| Diarize                 | Speaker identification — only available with `-tdrz` models         |

### Model Management

Before transcription works, you need to download at least one model. The panel provides:

- **Download** — select from the list of available Whisper models and download
- **Set Active** — choose which downloaded model to use
- **Delete** — remove a downloaded model

The panel also shows transcription statistics when available.

---

## Options

General settings that control how OpenScanner behaves. Settings are organized into groups.

### General

| Setting        | Description                                                           | Default |
| -------------- | --------------------------------------------------------------------- | ------- |
| Branding Label | Short text shown above the scanner (e.g. your county or project name) | (empty) |
| Support Email  | Contact email displayed to users                                      | (empty) |
| Public Access  | Allow unauthenticated users to listen to the scanner                  | Off     |

### Scanner Behavior

| Setting                  | Description                                                           | Default |
| ------------------------ | --------------------------------------------------------------------- | ------- |
| Sort Talkgroups by ID    | Sort talkgroup list by numeric ID instead of display order            | Off     |
| Allow Toggle by Tag      | Let users filter the scanner feed by tag                              | Off     |
| 12-Hour Time Format      | Display times as AM/PM instead of 24-hour                             | Off     |
| Show Listeners Count     | Display the number of active listeners in the scanner                 | Off     |
| Playback Mode Goes Live  | Automatically switch from playback to live when caught up             | Off     |
| AFS Systems              | Comma-separated system IDs that use AFS (Australian) talkgroup format | (empty) |
| Max Simultaneous Clients | Maximum number of WebSocket listeners allowed at once                 | 200     |

> Settings marked with a "Planned" badge in the UI are persisted but not yet wired to runtime behavior. They will activate in a future release.

### Call Processing

| Setting                             | Description                                                                         | Default        |
| ----------------------------------- | ----------------------------------------------------------------------------------- | -------------- |
| Audio Conversion (FFmpeg)           | **Disabled** / Enabled / Normalize / Loudnorm — controls audio processing on ingest | Disabled       |
| Audio Encoding Preset               | Codec and bitrate for converted audio (MP3 or AAC at various bitrates)              | AAC-LC 32 kbps |
| Disable Duplicate Call Detection    | Skip checking for duplicate calls on upload                                         | Off            |
| Duplicate Detection Time Frame (ms) | Window for matching duplicate calls (±milliseconds)                                 | 500            |
| Prune Database After (days)         | Auto-delete calls older than this many days (0 = never prune)                       | 7              |
| Search Patched Talkgroups           | Include patched talkgroups in search results                                        | Off            |

#### Audio Encoding Presets

| Preset         | Description                           |
| -------------- | ------------------------------------- |
| MP3 32 kbps    | Default MP3 quality                   |
| MP3 24 kbps    | Lower bitrate MP3                     |
| MP3 16 kbps    | Smallest MP3                          |
| AAC-LC 32 kbps | Default AAC quality                   |
| AAC-LC 24 kbps | Lower bitrate AAC                     |
| AAC-LC 16 kbps | Smallest AAC-LC                       |
| HE-AAC 12 kbps | High-efficiency AAC, very small files |
| HE-AAC 8 kbps  | Smallest possible, lowest quality     |

### Display

| Setting           | Description                                                   | Default |
| ----------------- | ------------------------------------------------------------- | ------- |
| Keypad Beep Style | Button press sound: **Disabled**, **Uniden**, or **Whistler** | Uniden  |

### Sharing & Notifications

| Setting                   | Description                                             | Default |
| ------------------------- | ------------------------------------------------------- | ------- |
| Shareable Links           | Allow users to create shareable links to specific calls | Off     |
| Shared Link Expiry (days) | How long shared links stay active (0 = never expire)    | 0       |
| Push Notifications        | Enable browser push notifications                       | Off     |

---

## Logs

View and search server logs in real time.

### Filters

- **Date Range** — select a range or use quick shortcuts (last 1, 6, or 24 hours)
- **Level** — filter by debug, info, warn, or error
- **Search** — text search across log messages
- **Limit** — number of entries to load (200 / 500 / 1,000 / 2,500 / 5,000)

### Controls

- **Auto-Refresh** — stream new log entries as they arrive via WebSocket
- **Auto-Scroll** — keep the view scrolled to the latest entry
- **Refresh** — manually reload logs
- **Clear Filters** — reset all filters to defaults
- **Log Level** — change the server's runtime log level (debug, info, warn, error) without restarting

HTTP request logs are color-coded by status: green for 2xx, gray for 3xx, yellow for 4xx, red for 5xx.

Each log row also shows short contextual chips next to the message for common events — for example `call=1234 sys=7 tg=5200 dur=3400` for an ingested call, or `downstream=2 call=1234 try=3` for a failed downstream push. Click a row to open a details panel with the full attribute list.

---

## Tools

Utilities for bulk data management and maintenance.

### CSV Import

- **Import Talkgroups** — upload a CSV to bulk-create or update talkgroups for a system. Choose between overwrite (update existing) or skip (keep existing) for duplicates.
- **Import Units** — same as above for unit records.

Both report how many records were inserted, updated, and skipped.

### CSV Export

- **Export Talkgroups** — download all talkgroups (or filter by system) as CSV.
- **Export Units** — download all units (or filter by system) as CSV.

### JSON Config

- **Export Config** — download the full server configuration as `openscanner-config.json`. Useful for backups or migrating to a new server. If secrets encryption is enabled, exported values retain their `enc::` encrypted form.
- **Import Config** — restore configuration from a previously exported JSON file. If the backup contains encrypted values (`enc::` prefix), the target server must have the same `--encryption-key` configured. The import is rejected if no key is set or the key cannot decrypt the values.

### RadioReference

Preview and apply talkgroup metadata from RadioReference. This lets you pull talkgroup names, groups, and tags from RadioReference data and merge them into your system.

### API Docs

Link to the Swagger API documentation at `/api/admin/docs`.

---

## Webhooks (Preview)

Webhook delivery is wired up but the sidebar entry is not yet exposed. Admins can reach the panel directly at `/admin/webhooks` to manage webhook endpoints.

Each webhook has:

| Field        | Description                                                                                                                            |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| URL          | HTTPS endpoint that receives a JSON POST for each new call                                                                             |
| Type         | `generic` (full call JSON) or `discord` (Discord-compatible embed payload)                                                             |
| Secret       | Optional HMAC-SHA256 secret. When set, OpenScanner signs each payload with `X-OpenScanner-Signature: sha256=<hex>`. Stored masked      |
| Systems JSON | Optional JSON array of system IDs (e.g. `[1, 2]`) that restricts delivery to calls from those systems; leave blank to receive all      |
| Disabled     | Temporarily stop sending events                                                                                                        |
| Order        | Display position                                                                                                                       |

The master toggle is **Options → Webhooks → Webhooks Enabled**. It is currently marked "Planned" in the UI because runtime dispatch is still being finalized; rows created here are persisted and will begin delivering once the toggle goes active.

Outbound webhook requests use OpenScanner's hardened HTTP client — redirects disabled, timeouts enforced, response body capped. See the [Deployment Guide](deployment-guide.md#reverse-proxy) for how to tighten outbound destinations on multi-tenant hosts.
