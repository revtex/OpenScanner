# OpenScanner — Recorder Integration Guide

> Call upload endpoints (`POST /api/call-upload`, `POST /api/trunk-recorder-call-upload`) are fully implemented. DirWatch (directory-based file ingestion) is also fully implemented as of Phase 7.

## HTTP API Upload

Recorders that support HTTP call upload can send calls to OpenScanner using:

- `POST /api/call-upload` — general-purpose multipart upload (SDRTrunk, voxcall, etc.)
- `POST /api/trunk-recorder-call-upload` — alias for the above, accepting identical fields

Both endpoints require an `X-API-Key` header (or `?key=` query parameter). See [API Reference](api.md) for full request/response details.

## Recorder Support Matrix

| Recorder          | HTTP API                             | DirWatch    |
| ----------------- | ------------------------------------ | ----------- |
| Trunk Recorder    | POST /api/trunk-recorder-call-upload | Implemented |
| SDRTrunk          | POST /api/call-upload                | Implemented |
| RTLSDR-Airband    | —                                    | Implemented |
| DSDPlus Fast Lane | —                                    | Implemented |
| ProScan           | —                                    | Implemented |
| voxcall           | POST /api/call-upload                | Implemented |

---

## DirWatch Service

The DirWatch service monitors local directories for new audio files and automatically ingests them into OpenScanner — no HTTP upload needed. Configure one or more **DirWatch entries** via the admin dashboard (`Admin → DirWatch`) or the API.

### How It Works

1. Each enabled dirwatch entry spawns a dedicated goroutine watching its configured `directory`.
2. When a new file appears (via fsnotify or polling), the service:
   - Validates the path is inside the watched directory (path-traversal protection)
   - Applies the extension filter if `extension` is set
   - Runs the recorder-type parser to extract call metadata
   - Runs the same ingest pipeline as an HTTP upload (duplicate check → FFmpeg conversion → DB insert → WS broadcast)
   - Optionally deletes the source file if `delete_after = 1`
3. Config changes take effect immediately — creating, updating, or deleting a dirwatch entry via the admin API triggers a full service reload (all watchers stop and restart from the DB).

### Configuration Fields

| Field          | Type    | Required | Notes                                                                                 |
| -------------- | ------- | -------- | ------------------------------------------------------------------------------------- |
| `directory`    | text    | yes      | Absolute path to the directory to watch                                               |
| `type`         | text    | yes      | Recorder type — see [Recorder Types](#recorder-types) below                           |
| `mask`         | text    | no       | Directory sub-path mask using `#TOKEN` substitution — see [Mask Tokens](#mask-tokens) |
| `extension`    | text    | no       | Only process files with this extension (e.g. `mp3`). Leave empty to process all audio |
| `frequency`    | integer | no       | Frequency override in Hz (used by types that cannot parse it from the filename)       |
| `delay`        | integer | no       | Polling interval in ms when `use_polling = 1` (minimum 500 ms; default 2000 ms)       |
| `delete_after` | 0/1     | no       | If `1`, delete the source audio file after successful ingest                          |
| `use_polling`  | 0/1     | no       | If `1`, use directory polling instead of fsnotify (recommended for CIFS/NFS mounts)   |
| `disabled`     | 0/1     | no       | If `1`, this entry is skipped on startup and reload                                   |
| `system_id`    | int FK  | no       | Override: use this DB system ID for all ingested calls                                |
| `talkgroup_id` | int FK  | no       | Override: use this DB talkgroup ID for all ingested calls                             |
| `order`        | integer | no       | Display order in admin UI                                                             |

### Reload Behaviour

Any create, update, or delete of a dirwatch entry via `POST/PUT/DELETE /api/admin/dirwatches` calls `Service.Reload` immediately. This stops all running watcher goroutines (gracefully, via context cancellation) and restarts fresh from the database — no server restart required.

---

## Recorder Types

### `trunk-recorder`

Trunk Recorder writes a JSON sidecar alongside each audio file (same stem, `.json` extension). The sidecar contains system ID, talkgroup ID, timestamp, frequency, and source unit lists.

**Parser behaviour:**

- Triggers on either the `.json` sidecar or the audio file
- Waits for both files to exist before ingesting (returns skip if the other half has not arrived yet)
- Reads at most 1 MiB from the sidecar to prevent OOM from crafted files

**Sidecar fields used:**

| Sidecar field        | Maps to            |
| -------------------- | ------------------ |
| `start_time`         | `date_time`        |
| `call_length`        | `duration` (ms)    |
| `freq`               | `frequency` (Hz)   |
| `talkgroup`          | `talkgroup_id`     |
| `sys_num`            | `system_id`        |
| `unit`               | `source`           |
| `srcList`            | `sources_json`     |
| `freqList`           | `frequencies_json` |
| `patched_talkgroups` | `patches_json`     |

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
- Parses system ID, talkgroup ID, and timestamp from the filename
- Falls back to file modification time if the timestamp part is missing
- `system_id` and `talkgroup_id` dirwatch overrides take precedence over the filename values

### `rtlsdr-airband`

RTLSDR-Airband does not embed metadata in filenames. The dirwatch entry's `system_id` and `talkgroup_id` fields specify which system and talkgroup all files in this directory belong to.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `frequency` from the `frequency` dirwatch field (or 0 if not set)

### `dsdplus`

DSDPlus Fast Lane mode drops audio files into a directory. System/talkgroup are identified via the dirwatch config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `system_id` and `talkgroup_id` from dirwatch config overrides (required)

### `proscan`

ProScan audio exports are identified via the dirwatch config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `system_id` and `talkgroup_id` from dirwatch config overrides (required)

### `voxcall`

voxcall audio exports are identified via the dirwatch config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `system_id` and `talkgroup_id` from dirwatch config overrides (required)

---

## Mask Tokens

The `mask` field supports token substitution to organise files into subdirectories based on call metadata. Tokens are expanded when a call is ingested.

| Token     | Value                                            | Example     |
| --------- | ------------------------------------------------ | ----------- |
| `#DATE`   | UTC date `YYYYMMDD`                              | `20260411`  |
| `#TIME`   | UTC time `HHMMSS`                                | `142305`    |
| `#ZTIME`  | UTC time `HHMMSS` (zero-padded, same as `#TIME`) | `142305`    |
| `#GROUP`  | Talkgroup group label                            | `Fire`      |
| `#SYSLBL` | System label                                     | `CountyP25` |
| `#TAG`    | Talkgroup tag label                              | `Dispatch`  |
| `#TGAFS`  | AFS/P25 system identifier                        | `12-001`    |
| `#UNIT`   | Source unit ID string                            | `4021`      |
| `#TGLBL`  | Talkgroup label                                  | `Fire Disp` |
| `#TGHZ`   | Talkgroup frequency in Hz                        | `851012500` |
| `#TGKHZ`  | Talkgroup frequency in kHz                       | `851012`    |
| `#TGMHZ`  | Talkgroup frequency in MHz (X.XXX)               | `851.012`   |
| `#TGID`   | Talkgroup radio ID                               | `1234`      |

**Example mask:** `#DATE/#SYSLBL/#TGLBL` → `20260411/CountyP25/Fire Disp`

Unrecognised tokens are left unchanged in the expanded string.
