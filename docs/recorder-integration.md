# OpenScanner — Recorder Integration Guide

> Call upload endpoints (`POST /api/call-upload`, `POST /api/trunk-recorder-call-upload`) are fully implemented. DirMonitor (directory-based file ingestion) is also fully implemented.
>
> For step-by-step setup instructions for each recorder, see [Recorder Setup Guide](recorder-setup.md).

## HTTP API Upload

Recorders that support HTTP call upload can send calls to OpenScanner using:

- `POST /api/call-upload` — general-purpose multipart upload (SDRTrunk, voxcall, etc.)
- `POST /api/trunk-recorder-call-upload` — alias endpoint with identical fields

Both endpoints require an `X-API-Key` header or a `key` form field. See [API Reference](api.md) for full request/response details.

## Recorder Support Matrix

| Recorder          | HTTP API                             | DirMonitor  |
| ----------------- | ------------------------------------ | ----------- |
| Trunk Recorder    | POST /api/trunk-recorder-call-upload | Implemented |
| SDRTrunk          | POST /api/call-upload                | Implemented |
| RTLSDR-Airband    | —                                    | Implemented |
| DSDPlus Fast Lane | —                                    | Implemented |
| ProScan           | —                                    | Implemented |
| voxcall           | POST /api/call-upload                | Implemented |

---

## DirMonitor Service

The DirMonitor service monitors local directories for new audio files and automatically ingests them into OpenScanner — no HTTP upload needed. Configure one or more **DirMonitor entries** via the admin dashboard (`Admin → Dir Monitors`) or the API.

### How It Works

1. Each enabled DirMonitor entry spawns a dedicated goroutine watching its configured `directory`.
2. When a new file appears (via fsnotify or polling), the service:
   - Validates the path is inside the watched directory (path-traversal protection)
   - Applies the extension filter if `extension` is set
   - Runs the recorder-type parser to extract call metadata
   - Applies the mask (if configured) to fill in any remaining fields
   - Runs the same ingest pipeline as an HTTP upload (duplicate check → FFmpeg conversion → DB insert → WS broadcast)
   - Optionally deletes the source file and any sidecar files if `deleteAfter = 1`
3. Config changes take effect immediately — creating, updating, or deleting a DirMonitor entry via the admin API triggers a full service reload (all watchers stop and restart from the DB).

### Configuration Fields

| Field         | Type    | Required | Notes                                                                                                                                                                             |
| ------------- | ------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `directory`   | text    | yes      | Absolute path to the directory to watch                                                                                                                                           |
| `type`        | text    | yes      | Recorder type — see [Recorder Types](#recorder-types) below                                                                                                                       |
| `mask`        | text    | no       | Directory sub-path mask using `#TOKEN` substitution — see [Mask Tokens](#mask-tokens)                                                                                             |
| `extension`   | text    | no       | Only process files with this extension (e.g. `mp3`). Leave empty to process all audio                                                                                             |
| `frequency`   | integer | no       | Frequency override in Hz (used by types that cannot parse it from metadata/filename)                                                                                              |
| `delay`       | integer | no       | Delay in ms. In polling mode: polling interval (min 500 ms, default 2000 ms). In fsnotify mode: debounce delay (min 2000 ms, default 2000 ms; only effective when set above 2000) |
| `deleteAfter` | 0/1     | no       | If `1`, delete the source audio file (and any sidecar) after successful ingest                                                                                                    |
| `usePolling`  | 0/1     | no       | If `1`, use directory polling instead of fsnotify (recommended for CIFS/NFS mounts)                                                                                               |
| `disabled`    | 0/1     | no       | If `1`, this entry is skipped on startup and reload                                                                                                                               |
| `systemId`    | int FK  | no       | Override: use this DB system ID for all ingested calls                                                                                                                            |
| `talkgroupId` | int FK  | no       | Override: use this DB talkgroup ID for all ingested calls                                                                                                                         |
| `order`       | integer | no       | Display order in admin UI                                                                                                                                                         |

For API requests, OpenScanner uses the camelCase names above. Database/export fields may still appear as snake_case.

### Reload Behaviour

Any create, update, or delete of a DirMonitor entry via the admin API calls `Service.Reload` immediately. This stops all running watcher goroutines (gracefully, via context cancellation) and restarts fresh from the database — no server restart required.

---

## Recorder Types

### `trunk-recorder`

Trunk Recorder writes a JSON sidecar alongside each audio file (same stem, `.json` extension). The sidecar contains system ID, talkgroup ID, timestamp, frequency, and source unit lists.

**Parser behaviour:**

- Triggers on either the `.json` sidecar or the audio file
- Waits for both files to exist before ingesting (returns skip if the other half has not arrived yet)
- Reads at most 1 MiB from the sidecar to prevent OOM from crafted files

**Sidecar fields used:**

| Sidecar field           | Maps to          |
| ----------------------- | ---------------- |
| `start_time`            | `dateTime`       |
| `call_length`           | `duration` (ms)  |
| `freq`                  | `frequency`      |
| `talkgroup`             | `talkgroupId`    |
| `sys_num`               | `systemId`       |
| `short_name`            | `systemLabel`    |
| `talkgroup_alpha_tag`   | `talkgroupLabel` |
| `talkgroup_tag`         | `talkgroupTag`   |
| `talkgroup_description` | `talkgroupName`  |
| `talkgroup_group`       | `talkgroupGroup` |
| `source_num`            | `source`         |
| `srcList`               | `sources`        |
| `freqList`              | `frequencies`    |
| `patched_talkgroups`    | `patches`        |

### `sdrtrunk`

SDRTrunk records MP3 files with verbose filenames (e.g. `20260414_153022_System_Site_Channel__TO_12345_FROM_67890.mp3`). Metadata is primarily extracted from embedded **ID3 tags**, not the filename.

**Parser behaviour:**

- Triggers on audio files only
- For MP3 files, first attempts ID3 parsing (artist/comment/title) for source, date/time, frequency, system label, talkgroup label, site, channel, and decoder
- Falls back to filename parsing (`<systemID>_<talkgroupID>_<unixTimestamp>.<ext>`) for non-MP3 files or files without ID3 tags
- Falls back to file modification time if the timestamp part is missing
- `systemId`, `talkgroupId`, and `frequency` DirMonitor config overrides take precedence over parsed values

### `rtlsdr-airband`

RTLSDR-Airband outputs MP3 files with a timestamp in the filename. The DirMonitor entry's `systemId` and `talkgroupId` fields specify which system and talkgroup all files in this directory belong to.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp parsed from filename (`_YYYYMMDD_HHMMSS`, `_YYYYMMDD_HHMM`, or `_YYYYMMDD_HH`); falls back to file modification time
- Frequency parsed from filename suffix when `include_freq` is enabled in RTLSDR-Airband (only used if config frequency is unset)
- In practice, `systemId` and `talkgroupId` must be configured, otherwise ingest is rejected as missing required IDs

### `dsdplus`

DSDPlus Fast Lane mode drops audio files into date-organised directories.

**File structure:**

```
<date folder YYYYMMDD>/
  HHMMSS_[data]_MODE_CHANNEL_[TALKGROUP label]_[SOURCE label].<ext>
```

**Parser behaviour:**

- Triggers on audio files only
- Date extracted from parent folder name (must end with YYYYMMDD); time from filename prefix (HHMMSS)
- Falls back to file modification time if date/time extraction fails
- System ID decoded from MODE + CHANNEL segments (supports ConP, DMR, P25, NEXEDGE modes)
- Talkgroup ID extracted from second-to-last bracket segment
- Source unit ID extracted from last bracket segment
- `systemId` and `talkgroupId` DirMonitor config overrides take precedence over parsed values

### `proscan`

ProScan audio exports are primarily identified via the DirMonitor config.

**Parser behaviour:**

- Triggers on audio files only
- Timestamp from file modification time
- `systemId` and `talkgroupId` from DirMonitor config overrides are required in practice

---

## Mask Tokens

The `mask` field supports token substitution to extract metadata from the file's path relative to the watched directory. Tokens are matched when parsing incoming files, and expanded when building output paths.

| Token     | Value                                                          | Regex pattern | Example     |
| --------- | -------------------------------------------------------------- | ------------- | ----------- |
| `#DATE`   | UTC date `YYYYMMDD`                                            | `\S+?`        | `20260411`  |
| `#TIME`   | UTC time `HHMMSS`                                              | `\S+?`        | `142305`    |
| `#ZTIME`  | UTC time `HHMMSS` (zero-padded, same as `#TIME`)               | `\S+?`        | `142305`    |
| `#GROUP`  | Talkgroup group label                                          | `.+?`         | `Fire`      |
| `#SYSLBL` | System label                                                   | `.+?`         | `CountyP25` |
| `#TAG`    | Talkgroup tag label                                            | `.+?`         | `Dispatch`  |
| `#TGAFS`  | AFS system identifier (always empty in current implementation) | `.+?`         | (empty)     |
| `#UNIT`   | Source unit ID string                                          | `\d+`         | `4021`      |
| `#TGLBL`  | Talkgroup label                                                | `.+?`         | `Fire Disp` |
| `#TGMHZ`  | Talkgroup frequency in MHz (X.XXX)                             | `[\d.]+`      | `851.012`   |
| `#TGKHZ`  | Talkgroup frequency in kHz                                     | `\d+`         | `851012`    |
| `#TGHZ`   | Talkgroup frequency in Hz                                      | `\d+`         | `851012500` |
| `#TGID`   | Talkgroup radio ID                                             | `\d+`         | `1234`      |
| `#TG`     | Alias of `#TGID`                                               | `\d+`         | `1234`      |
| `#SYS`    | System radio ID                                                | `\d+`         | `12`        |
| `#MHZ`    | Generic frequency in MHz (X.XXX)                               | `[\d.]+`      | `851.012`   |
| `#KHZ`    | Generic frequency in kHz                                       | `\d+`         | `851012`    |
| `#HZ`     | Generic frequency in Hz                                        | `\d+`         | `851012500` |

**Example mask:** `#DATE/#SYSLBL/#TGLBL` → `20260411/CountyP25/Fire Disp`

Unrecognised tokens are left unchanged in the expanded string.
