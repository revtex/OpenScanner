# OpenScanner — Recorder Integration Guide

> Call upload endpoints (`POST /api/call-upload`, `POST /api/trunk-recorder-call-upload`) are fully implemented. DirMonitor (directory-based file ingestion) is also fully implemented as of Phase 7.

## HTTP API Upload

Recorders that support HTTP call upload can send calls to OpenScanner using:

- `POST /api/call-upload` — general-purpose multipart upload (SDRTrunk, voxcall, etc.)
- `POST /api/trunk-recorder-call-upload` — alias for the above, accepting identical fields

Both endpoints require an `X-API-Key` header (or `?key=` query parameter). See [API Reference](api.md) for full request/response details.

## Recorder Support Matrix

| Recorder          | HTTP API                             | DirMonitor    |
| ----------------- | ------------------------------------ | ----------- |
| Trunk Recorder    | POST /api/trunk-recorder-call-upload | Implemented |
| SDRTrunk          | POST /api/call-upload                | Implemented |
| RTLSDR-Airband    | —                                    | Implemented |
| DSDPlus Fast Lane | —                                    | Implemented |
| ProScan           | —                                    | Implemented |
| voxcall           | POST /api/call-upload                | Implemented |

---

## DirMonitor Service

The DirMonitor service monitors local directories for new audio files and automatically ingests them into OpenScanner — no HTTP upload needed. Configure one or more **DirMonitor entries** via the admin dashboard (`Admin → DirMonitor`) or the API.

### How It Works

1. Each enabled dirmonitor entry spawns a dedicated goroutine watching its configured `directory`.
2. When a new file appears (via fsnotify or polling), the service:
   - Validates the path is inside the watched directory (path-traversal protection)
   - Applies the extension filter if `extension` is set
   - Runs the recorder-type parser to extract call metadata
   - Runs the same ingest pipeline as an HTTP upload (duplicate check → FFmpeg conversion → DB insert → WS broadcast)
   - Optionally deletes the source file if `deleteAfter = 1`
3. Config changes take effect immediately — creating, updating, or deleting a dirmonitor entry via the admin API triggers a full service reload (all watchers stop and restart from the DB).

### Configuration Fields

| Field         | Type    | Required | Notes                                                                                 |
| ------------- | ------- | -------- | ------------------------------------------------------------------------------------- |
| `directory`   | text    | yes      | Absolute path to the directory to watch                                               |
| `type`        | text    | yes      | Recorder type — see [Recorder Types](#recorder-types) below                           |
| `mask`        | text    | no       | Directory sub-path mask using `#TOKEN` substitution — see [Mask Tokens](#mask-tokens) |
| `extension`   | text    | no       | Only process files with this extension (e.g. `mp3`). Leave empty to process all audio |
| `frequency`   | integer | no       | Frequency override in Hz (used by types that cannot parse it from metadata/filename)  |
| `delay`       | integer | no       | Polling interval in ms when `usePolling = 1` (minimum 500 ms; default 2000 ms)        |
| `deleteAfter` | 0/1     | no       | If `1`, delete the source audio file after successful ingest                          |
| `usePolling`  | 0/1     | no       | If `1`, use directory polling instead of fsnotify (recommended for CIFS/NFS mounts)   |
| `disabled`    | 0/1     | no       | If `1`, this entry is skipped on startup and reload                                   |
| `systemId`    | int FK  | no       | Override: use this DB system ID for all ingested calls                                |
| `talkgroupId` | int FK  | no       | Override: use this DB talkgroup ID for all ingested calls                             |
| `order`       | integer | no       | Display order in admin UI                                                             |

For API requests, OpenScanner uses the camelCase names above. Database/export fields may still appear as snake_case.

### Reload Behaviour

Any create, update, or delete of a dirmonitor entry via `POST/PUT/DELETE /api/admin/dirmonitors` calls `Service.Reload` immediately. This stops all running watcher goroutines (gracefully, via context cancellation) and restarts fresh from the database — no server restart required.

---

## Recorder Types

### `trunk-recorder`

Trunk Recorder writes a JSON sidecar alongside each audio file (same stem, `.json` extension). The sidecar contains system ID, talkgroup ID, timestamp, frequency, and source unit lists.

**Parser behaviour:**

- Triggers on either the `.json` sidecar or the audio file
- Waits for both files to exist before ingesting (returns skip if the other half has not arrived yet)
- Reads at most 1 MiB from the sidecar to prevent OOM from crafted files

**Sidecar fields used:**

| Sidecar field        | Maps to          |
| -------------------- | ---------------- |
| `start_time`         | `dateTime`       |
| `call_length`        | `duration` (ms)  |
| `freq`               | `frequency` (Hz) |
| `talkgroup`          | `talkgroupId`    |
| `sys_num`            | `systemId`       |
| `unit`               | `source`         |
| `srcList`            | `sources`        |
| `freqList`           | `frequencies`    |
| `patched_talkgroups` | `patches`        |

**Minimal Trunk Recorder config (`config.json`):**

```json
{
  "uploadServer": "http://openscanner:3000",
  "systems": [
    {
      "shortName": "mysite",
      "apiKey": "<your-api-key>"
    }
  ]
}
```

### `sdrtrunk`

SDRTrunk names its audio files with embedded metadata: `<systemID>_<talkgroupID>_<unixTimestamp>.<ext>`

**Parser behaviour:**

- Triggers on audio files only
- For MP3 files, first attempts ID3 parsing (artist/comment/title) for source, date/time, frequency, system label, talkgroup title, site, channel, and decoder
- Falls back to filename parsing for system ID, talkgroup ID, and timestamp
- Falls back to file modification time if the timestamp part is missing
- `systemId`, `talkgroupId`, and `frequency` dirmonitor overrides take precedence over parsed values

### `rtlsdr-airband`

RTLSDR-Airband does not embed metadata in filenames. The dirmonitor entry's `systemId` and `talkgroupId` fields specify which system and talkgroup all files in this directory belong to.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `frequency` from the `frequency` dirmonitor field (or 0 if not set)
- In practice, `systemId` and `talkgroupId` must be configured, otherwise ingest is rejected as missing required IDs

### `dsdplus`

DSDPlus Fast Lane mode drops audio files into a directory. Parser attempts to infer metadata from filename/date structure and applies dirmonitor overrides when set.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp parsed from filename/date structure when available, otherwise file modification time
- `systemId` and `talkgroupId` from dirmonitor config overrides are recommended; ingest requires non-zero resolved IDs

### `proscan`

ProScan audio exports are primarily identified via the dirmonitor config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `systemId` and `talkgroupId` from dirmonitor config overrides are required in practice

### `voxcall`

voxcall audio exports are primarily identified via the dirmonitor config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `systemId` and `talkgroupId` from dirmonitor config overrides are required in practice

---

## Mask Tokens

The `mask` field supports token substitution to organise files into subdirectories based on call metadata. Tokens are expanded when a call is ingested.

| Token     | Value                                                      | Example     |
| --------- | ---------------------------------------------------------- | ----------- |
| `#DATE`   | UTC date `YYYYMMDD`                                        | `20260411`  |
| `#TIME`   | UTC time `HHMMSS`                                          | `142305`    |
| `#ZTIME`  | UTC time `HHMMSS` (zero-padded, same as `#TIME`)           | `142305`    |
| `#GROUP`  | Talkgroup group label                                      | `Fire`      |
| `#SYSLBL` | System label                                               | `CountyP25` |
| `#TAG`    | Talkgroup tag label                                        | `Dispatch`  |
| `#TGAFS`  | Reserved token (currently empty unless populated upstream) | (empty)     |
| `#UNIT`   | Source unit ID string                                      | `4021`      |
| `#TGLBL`  | Talkgroup label                                            | `Fire Disp` |
| `#TGHZ`   | Talkgroup frequency in Hz                                  | `851012500` |
| `#TGKHZ`  | Talkgroup frequency in kHz                                 | `851012`    |
| `#TGMHZ`  | Talkgroup frequency in MHz (X.XXX)                         | `851.012`   |
| `#TGID`   | Talkgroup radio ID                                         | `1234`      |
| `#TG`     | Alias of `#TGID`                                           | `1234`      |
| `#SYS`    | System radio ID                                            | `12`        |
| `#HZ`     | Generic frequency in Hz                                    | `851012500` |
| `#KHZ`    | Generic frequency in kHz                                   | `851012`    |
| `#MHZ`    | Generic frequency in MHz (X.XXX)                           | `851.012`   |

**Example mask:** `#DATE/#SYSLBL/#TGLBL` → `20260411/CountyP25/Fire Disp`

Unrecognised tokens are left unchanged in the expanded string.
