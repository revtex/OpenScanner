# Recorder Setup Guide

Step-by-step instructions for connecting radio recording software to OpenScanner. Each section covers **API upload** (HTTP push) and **DirMonitor** (local directory watching) where applicable.

> **Prerequisites** — Before configuring any recorder you need:
>
> 1. OpenScanner running and accessible (e.g. `http://openscanner:3000`)
> 2. An **API key** created in **Admin → API Keys**
> 3. (Recommended) **Auto-Populate** enabled in **Admin → Settings** — this lets OpenScanner create systems and talkgroups automatically from incoming calls

---

## Trunk Recorder

[Trunk Recorder](https://github.com/robotastic/trunk-recorder) is a Linux-based P25/DMR trunked radio recorder. It supports two methods of sending calls to OpenScanner.

### Method 1: API Upload (rdioscanner_uploader plugin)

The built-in `rdioscanner_uploader` plugin pushes completed calls over HTTP.

**Trunk Recorder `config.json`:**

```json
{
  "sources": [
    /* ... your SDR sources ... */
  ],
  "systems": [
    {
      "shortName": "mysite",
      "systemType": "p25",
      "control_channels": [851012500]
    }
  ],
  "plugins": [
    {
      "name": "rdioscanner_uploader",
      "server": "http://openscanner:3000",
      "systems": [
        {
          "shortName": "mysite",
          "apiKey": "your-api-key-here"
        }
      ]
    }
  ]
}
```

| Config field | Description                                                         |
| ------------ | ------------------------------------------------------------------- |
| `server`     | OpenScanner base URL (plugin appends the upload path automatically) |
| `apiKey`     | The API key you created in Admin → API Keys (per-system)            |
| `shortName`  | Must match the system's `shortName` — used as system label          |

The plugin sends calls as `multipart/form-data` with these fields:

| Field            | Description                                                      |
| ---------------- | ---------------------------------------------------------------- |
| `key`            | API key (form field, not header)                                 |
| `system`         | System numeric ID (from plugin config `systemId`)                |
| `systemLabel`    | System short name (used as label when auto-creating systems)     |
| `talkgroup`      | Talkgroup numeric ID                                             |
| `dateTime`       | Unix timestamp (seconds)                                         |
| `audio`          | Audio file (WAV or M4A if `compress_wav` is enabled)             |
| `audioName`      | Audio filename                                                   |
| `audioType`      | MIME type (`audio/wav` or `audio/mp4`)                           |
| `frequency`      | Frequency in Hz                                                  |
| `frequencies`    | JSON array of frequency/error data per transmission              |
| `sources`        | JSON array of source units with positions                        |
| `patches`        | JSON array of patched talkgroup IDs                              |
| `talkgroupLabel` | Talkgroup alpha tag — human-readable label (from talkgroups CSV) |
| `talkgroupTag`   | Talkgroup tag — category like "Law Dispatch" (auto-creates tag)  |
| `talkgroupGroup` | Talkgroup group — e.g. "Police" (auto-creates group)             |
| `talkgroupName`  | Talkgroup description — longer descriptive name                  |

When auto-populate is enabled, `systemLabel` is used as the label for auto-created systems, `talkgroupLabel` sets the talkgroup label (alpha tag), `talkgroupTag` creates and assigns a tag category, `talkgroupGroup` creates and assigns a group, and `talkgroupName` sets the descriptive name.

OpenScanner accepts both `system`/`talkgroup` (Trunk Recorder format) and `systemId`/`talkgroupId` (canonical format).

### Method 2: DirMonitor (Directory Watching)

If Trunk Recorder writes files to a local directory (or a shared mount), OpenScanner can watch that directory and ingest calls automatically.

Trunk Recorder writes two files per call with the same filename stem:

- **Audio file** — e.g. `12345-1689012345_851012500.wav`
- **JSON sidecar** — e.g. `12345-1689012345_851012500.json`

The DirMonitor waits for both files to exist before ingesting.

**Admin → Dir Monitors → Add:**

| Field        | Value                                               |
| ------------ | --------------------------------------------------- |
| Directory    | `/path/to/trunk-recorder/recordings`                |
| Type         | `trunk-recorder`                                    |
| Delete After | `Yes` if you want files removed after ingest        |
| Extension    | Leave empty (parser handles both `.json` and audio) |

**What the parser extracts from the JSON sidecar:**

| Sidecar JSON field      | OpenScanner field                                      |
| ----------------------- | ------------------------------------------------------ |
| `start_time`            | Date/Time                                              |
| `call_length`           | Duration                                               |
| `freq`                  | Frequency                                              |
| `talkgroup`             | Talkgroup ID                                           |
| `short_name`            | System Label                                           |
| `talkgroup_alpha_tag`   | Talkgroup Label (alpha tag)                            |
| `talkgroup_tag`         | Talkgroup Tag (category — auto-creates tag)            |
| `talkgroup_description` | Talkgroup Name (description)                           |
| `talkgroup_group`       | Talkgroup Group (auto-creates group)                   |
| `source_num`            | Source Unit                                            |
| `srcList`               | Sources (JSON)                                         |
| `freqList`              | Frequencies (JSON)                                     |
| `patched_talkgroups`    | Patches (JSON)                                         |
| `sys_num`               | System ID (numeric — used when `short_name` is absent) |

System ID and Talkgroup ID config overrides on the DirMonitor entry take precedence over sidecar values if set.

---

## SDRTrunk

[SDRTrunk](https://github.com/DSheirer/sdrtrunk) is a cross-platform Java application for decoding P25, DMR, LTR, and other trunked radio protocols. It has a built-in rdio-scanner streaming integration.

### Method 1: API Upload (Built-in Streaming)

SDRTrunk has a native rdio-scanner broadcast integration. Configure it from the Playlist Editor.

**SDRTrunk → Playlist Editor → Streaming:**

1. Click **Add** and select **Rdio Scanner**
2. Fill in the configuration:

| SDRTrunk field | Value                                                  |
| -------------- | ------------------------------------------------------ |
| Host           | `http://openscanner:3000/api/call-upload`              |
| API Key        | Your API key from Admin → API Keys                     |
| System ID      | The numeric system ID matching your OpenScanner system |

3. Click **Save**, then **Start** the broadcast

**Connection test:** SDRTrunk sends a `test=1` request on startup to verify the connection. OpenScanner responds with the expected string so SDRTrunk reports `CONNECTED`.

**Fields SDRTrunk sends:**

| Field            | Description                                                 |
| ---------------- | ----------------------------------------------------------- |
| `key`            | API key                                                     |
| `system`         | System numeric ID                                           |
| `talkgroup`      | Talkgroup numeric ID                                        |
| `dateTime`       | Unix timestamp (seconds)                                    |
| `audio`          | MP3 audio file                                              |
| `source`         | Source radio unit ID                                        |
| `frequency`      | Frequency in Hz                                             |
| `talkgroupLabel` | Talkgroup label from alias config                           |
| `talkgroupGroup` | Talkgroup group name (used to auto-create/assign groups)    |
| `talkerAlias`    | DMR/P25 talker alias (stored per-call, shown in scanner UI) |
| `systemLabel`    | System name (used as label when auto-creating systems)      |
| `patches`        | JSON array of patched talkgroup IDs                         |

When auto-populate is enabled, `systemLabel` is used as the label for auto-created systems, `talkgroupLabel` sets the talkgroup label, and `talkgroupGroup` creates and assigns a group to auto-created talkgroups.

### Method 2: DirMonitor (Directory Watching)

SDRTrunk can also record calls to disk. The DirMonitor parser extracts metadata from either MP3 ID3 tags or the filename.

**SDRTrunk recording filename format:**

SDRTrunk produces verbose filenames (e.g. `20260414_153022_MySystem_Site1_Control_TO_12345_FROM_67890.mp3`). Metadata is extracted from embedded **ID3 tags** in the MP3, not the filename. When ID3 tags are unavailable, the parser falls back to a simpler filename pattern:

```
<systemID>_<talkgroupID>_<unixTimestamp>.<ext>
```

Example: `12_1234_1713120622.mp3`

**Admin → Dir Monitors → Add:**

| Field        | Value                                                                         |
| ------------ | ----------------------------------------------------------------------------- |
| Directory    | SDRTrunk's recording directory (User Preferences → File Storage → Recordings) |
| Type         | `sdrtrunk`                                                                    |
| Extension    | `mp3` (recommended)                                                           |
| Delete After | Your preference                                                               |

**What the parser extracts from MP3 ID3 tags:**

| ID3 Tag        | Content                                                                                                         | Extracted Field                                            |
| -------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| Artist (TPE1)  | `67890` (source radio ID)                                                                                       | Source Unit                                                |
| Title (TIT2)   | `12345 Fire Dispatch` (talkgroup + alias)                                                                       | Talkgroup ID, Talkgroup Label                              |
| Comment (COMM) | `Date:2026-04-14 15:30:22.000;System:MyP25;Frequency:851012500;Site:North;Channel:Control;Decoder:P25 Phase 1;` | Date/Time, System Label, Frequency, Site, Channel, Decoder |

**Filename fallback** (non-MP3 or files without ID3 tags):

Pattern: `<systemID>_<talkgroupID>_<unixTimestamp>.<ext>`

If the timestamp cannot be parsed from either source, file modification time is used.

**Config overrides:** Setting System ID, Talkgroup ID, or Frequency on the DirMonitor entry overrides any values parsed from the file.

---

## RTLSDR-Airband

[RTLSDR-Airband](https://github.com/rtl-airband/RTLSDR-Airband) is a multichannel AM/NFM demodulator for SDR receivers, primarily used for aviation monitoring. It outputs MP3 files with no metadata sidecar — all identification must come from the DirMonitor configuration.

### DirMonitor Only

RTLSDR-Airband does not have an HTTP upload feature. Use DirMonitor to ingest recordings.

**RTLSDR-Airband `rtl_airband.conf` file output:**

```
outputs: (
  {
    type = "file";
    directory = "/recordings/tower";
    filename_template = "TOWER";
    split_on_transmission = true;   # one file per transmission
    include_freq = true;            # append frequency to filename
  }
);
```

This produces files like:

- `TOWER_20260414_153022.mp3` (without `include_freq`)
- `TOWER_20260414_153022_120500000.mp3` (with `include_freq`)
- `TOWER_20260414_15.mp3` (hourly rotation, without `split_on_transmission`)

**Admin → Dir Monitors → Add:**

| Field        | Value                                                                                                                                    |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Directory    | `/recordings/tower` (matches RTLSDR-Airband `directory`)                                                                                 |
| Type         | `rtlsdr-airband`                                                                                                                         |
| System ID    | **Required** — select the system these recordings belong to                                                                              |
| Talkgroup ID | **Required** — select the talkgroup (e.g. "Tower")                                                                                       |
| Frequency    | Optional — override frequency in Hz. If unset and `include_freq` is enabled in RTLSDR-Airband, the frequency is parsed from the filename |
| Extension    | `mp3`                                                                                                                                    |
| Delete After | Your preference                                                                                                                          |

**What the parser extracts from the filename:**

| Filename Component | Extracted Field                                   | Example            |
| ------------------ | ------------------------------------------------- | ------------------ |
| `_YYYYMMDD_HHMMSS` | Date/Time (UTC, full precision)                   | `_20260414_153022` |
| `_YYYYMMDD_HHMM`   | Date/Time (UTC, minute precision)                 | `_20260414_1530`   |
| `_YYYYMMDD_HH`     | Date/Time (UTC, hour only)                        | `_20260414_15`     |
| Trailing `_FREQ`   | Frequency (Hz, only if config frequency is unset) | `_120500000`       |

If no timestamp is found in the filename, file modification time is used as a fallback.

**Important:** Since RTLSDR-Airband is an analog aviation receiver and has no concept of trunked radio systems or talkgroups, you **must** set System ID and Talkgroup ID on the DirMonitor entry. Without these, calls will be rejected during ingest.

### Typical Setup for Multiple Frequencies

If you monitor several aviation frequencies, create one DirMonitor entry per frequency (each pointing to a different output directory or using different `filename_template` values):

| DirMonitor Entry | Directory            | Talkgroup          | Frequency |
| ---------------- | -------------------- | ------------------ | --------- |
| Tower            | `/recordings/tower`  | Tower (120.5 MHz)  | 120500000 |
| Ground           | `/recordings/ground` | Ground (121.9 MHz) | 121900000 |
| ATIS             | `/recordings/atis`   | ATIS (124.85 MHz)  | 124850000 |

Alternatively, if RTLSDR-Airband writes all frequencies to one directory with `include_freq = true`, you can use a single DirMonitor entry without setting a frequency override — the parser will extract it from each filename.

---

## DSDPlus Fast Lane

[DSD+](https://www.dsdplus.com/) (DSD Plus) is a Windows-based digital voice decoder that supports P25, DMR, NEXEDGE, D-STAR, and other protocols. DSD+ Fast Lane (FMP24) is its live monitoring companion. Recordings are saved to date-stamped folders.

### DirMonitor Only

DSD+ does not have an HTTP upload feature. Use DirMonitor to ingest recordings.

**DSD+ recording structure:**

DSD+ saves audio files into date-stamped folders with metadata encoded in the filename:

```
20260414/
  143022_[some data]_P25(BS)_12345-Site1_[54241][Fire Dispatch]_[4424001].mp3
  143155_[some data]_DMR(BS)_100-Site2_[999][---]_[1234].mp3
```

**Filename format:** `HHMMSS_[data]_MODE_CHANNEL_[TGID][TG label]_[SRC][SRC label].ext`

| Segment           | Description                                                                       |
| ----------------- | --------------------------------------------------------------------------------- |
| `HHMMSS`          | Time of recording (local time zone)                                               |
| `[data]`          | Internal DSD+ metadata (ignored)                                                  |
| `MODE`            | Protocol mode: `P25(BS)`, `DMR(BS)`, `ConP(BS)`, `NEXEDGE48(CB)`, `P25`, etc.    |
| `CHANNEL`         | Channel info with system ID prefix (format varies by mode)                        |
| `[TGID][label]`   | Talkgroup ID and optional label in brackets                                       |
| `[SRC][label]`    | Source unit ID and optional label in brackets                                      |
| Parent folder     | Date folder name ending in `YYYYMMDD` (e.g. `20260414`)                           |

**Admin → Dir Monitors → Add:**

| Field        | Value                                                    |
| ------------ | -------------------------------------------------------- |
| Directory    | DSD+ recordings root (parent of the YYYYMMDD folders)    |
| Type         | `dsdplus`                                                |
| Extension    | `mp3` (or `wav` depending on DSD+ config)                |
| Delete After | Your preference                                          |

**What the parser extracts from the filename:**

| Filename Component                          | Extracted Field  | Example                                       |
| ------------------------------------------- | ---------------- | --------------------------------------------- |
| Parent folder `YYYYMMDD` + filename `HHMMSS`| Date/Time        | Folder `20260414` + prefix `143022`            |
| MODE + CHANNEL                              | System ID        | `P25(BS)` + `12345-Site1` → system ID `12345` |
| Second-to-last bracket segment              | Talkgroup ID     | `[54241]` → `54241`                           |
| Second bracket in TG segment                | Talkgroup Label  | `[Fire Dispatch]` → label (punctuation-only filtered) |
| Last bracket segment                        | Source Unit      | `[4424001]` → source `4424001`                |

**System ID extraction by mode:**

| Mode                                          | System ID source                              |
| --------------------------------------------- | --------------------------------------------- |
| `ConP(BS)`, `DMR(BS)`, `P25(BS)`             | Numeric prefix of channel segment (`12345-*`) |
| `NEXEDGE48(*)`, `NEXEDGE96(*)`               | Second number in channel or RAN number        |
| `P25` (non-BS)                                | Hex value parsed from channel segment         |

**Config overrides:** Setting System ID or Talkgroup ID on the DirMonitor entry overrides any values parsed from the filename.

---

## ProScan

[ProScan](https://www.proscan.org/) is a Windows application for controlling and recording from Uniden scanners. It records audio files with metadata in the filename, using the mask system to extract fields.

### DirMonitor Only

ProScan does not have an HTTP upload feature. Use DirMonitor with the **Mask** system to extract metadata from filenames.

The ProScan parser itself is minimal — it uses file modification time for the timestamp and relies entirely on the DirMonitor mask to extract system, talkgroup, group, tag, and other fields from the filename path.

**Example ProScan recording directory:**

```
C:\ProScan\Recordings\
  St. Johns County Public Safety/
    2025-08-17_12-15-16_St. Johns County Fire Rescue_A1 Primary (Dispatch)_10000.wav
```

**Admin → Dir Monitors → Add:**

| Field        | Value                                                                    |
| ------------ | ------------------------------------------------------------------------ |
| Directory    | ProScan's recording directory (e.g. `C:\ProScan\Recordings\SystemName`) |
| Type         | `proscan`                                                                |
| Extension    | `wav`                                                                    |
| Mask         | **Required** — pattern to extract metadata from filenames (see below)    |
| Delete After | Your preference                                                          |

**Mask examples for common ProScan filename patterns:**

| Filename Pattern                                                              | Mask                              |
| ----------------------------------------------------------------------------- | --------------------------------- |
| `2025-08-17_12-15-16_Fire Rescue_A1 Primary (Dispatch)_10000.wav`            | `#DATE_#TIME_#GROUP_#TGLBL_#TG`   |
| `2025-08-17_12-15-16_10000.wav`                                              | `#DATE_#TIME_#TG`                 |
| `Police/2025-08-17_12-15-16_Law Dispatch_10000.wav`                          | `#GROUP/#DATE_#TIME_#TAG_#TG`     |

See the [Mask System](#mask-system) section below for the full list of available tokens.

**Important:** Without a mask, the parser only captures file modification time and any System ID / Talkgroup ID overrides set on the DirMonitor entry. All other metadata (date, time, talkgroup, group, tag, label) must come from the mask.

---

## voxcall

[voxcall](https://github.com/USA-RedDragon/voxcall) is a lightweight call uploader that sends recordings to rdio-scanner-compatible endpoints.

### API Upload Only

voxcall pushes calls via HTTP to `/api/call-upload`. It does not produce local files for DirMonitor.

**voxcall configuration:**

| Config field | Value                                          |
| ------------ | ---------------------------------------------- |
| `server`     | `http://openscanner:3000/api/call-upload`      |
| `apiKey`     | Your API key from Admin → API Keys             |

**Fields voxcall sends:**

| Field         | Description                                      |
| ------------- | ------------------------------------------------ |
| `key`         | API key                                          |
| `systemId`    | System numeric ID                                |
| `talkgroupId` | Talkgroup numeric ID                             |
| `dateTime`    | ISO 8601 timestamp (RFC 3339 format)             |
| `audio`       | Audio file                                       |
| `frequency`   | Frequency in Hz                                  |
| `source`      | Source unit ID                                    |
| `sources`     | JSON array of source units                       |
| `duration`    | Call duration in seconds                         |

**Key difference:** voxcall sends `dateTime` as an ISO 8601 string (e.g. `2026-04-14T15:30:22Z`) rather than a Unix timestamp. OpenScanner handles both formats automatically.

---

## Generic (Custom Recorders)

For recorders not listed above — or any software that writes audio files to a directory — use the **generic** DirMonitor type.

The generic parser does no filename parsing at all. It uses:

- **File modification time** as the call timestamp
- **System ID** and **Talkgroup ID** from the DirMonitor config overrides (required)
- **Mask** for extracting any additional metadata from the filename/path

**Admin → Dir Monitors → Add:**

| Field        | Value                                                                                               |
| ------------ | --------------------------------------------------------------------------------------------------- |
| Directory    | The directory where audio files are written                                                         |
| Type         | (any unrecognized type name, or leave empty — all unknown types fall through to the generic parser) |
| System ID    | **Required** — select the system                                                                    |
| Talkgroup ID | **Required** unless using a mask with `#TG` or `#TGID`                                             |
| Mask         | Optional — extract metadata from filename                                                           |
| Extension    | The audio file extension to watch for                                                               |
| Delete After | Your preference                                                                                     |

**Tip:** Combine the generic type with a mask to handle virtually any recording software. For example, if your recorder writes files as `recordings/System1/TG_12345/20260414_153022.wav`, set:

- Directory: `/recordings`
- Mask: `#SYSLBL/#TGLBL/#DATE_#TIME`

---

## Common Settings

These OpenScanner settings (in **Admin → Settings**) affect all recorder integrations:

| Setting                       | Description                                                     | Default |
| ----------------------------- | --------------------------------------------------------------- | ------- |
| Auto-Populate                 | Automatically create systems and talkgroups from incoming calls | Off     |
| Audio Conversion              | FFmpeg conversion mode: disabled, enabled, normalize, loudnorm  | Enabled |
| Duplicate Detection           | Reject duplicate calls within a time window                     | On      |
| Duplicate Detection Timeframe | Window in milliseconds for duplicate detection                  | 2000    |
| API Key Call Rate             | Max calls per minute per API key                                | 60      |

## Supported Audio Formats

OpenScanner accepts these audio file formats for both API upload and DirMonitor:

`.mp3`, `.wav`, `.m4a`, `.aac`, `.ogg`, `.flac`, `.opus`

When audio conversion is enabled, all files are converted to AAC using FFmpeg.

## Mask System

The optional **Mask** field on a DirMonitor entry lets you extract metadata from the file's path relative to the watched directory. This is useful for recorders that don't embed metadata in the file itself.

**Example:** If your files are organized as `recordings/2026/04/14/Tower_153022.mp3`, set:

- Directory: `/recordings`
- Mask: `#DATE/#TGLBL_#TIME`

The mask parser extracts values by matching `#TOKEN` placeholders against the file path:

| Token     | Extracts        | Pattern  | Example     |
| --------- | --------------- | -------- | ----------- |
| `#DATE`   | Date (YYYYMMDD) | `\S+?`   | `20260414`  |
| `#TIME`   | Time (HHMMSS)   | `\S+?`   | `153022`    |
| `#ZTIME`  | Time (HHMMSS)   | `\S+?`   | `153022`    |
| `#GROUP`  | Talkgroup group | `.+?`    | `Fire`      |
| `#SYSLBL` | System label    | `.+?`    | `CountyP25` |
| `#TAG`    | Talkgroup tag   | `.+?`    | `Dispatch`  |
| `#TGAFS`  | AFS identifier  | `.+?`    | (empty)     |
| `#UNIT`   | Source unit ID  | `\d+`    | `4021`      |
| `#TGLBL`  | Talkgroup label | `.+?`    | `Fire Disp` |
| `#TGMHZ`  | TG freq (MHz)   | `[\d.]+` | `851.012`   |
| `#TGKHZ`  | TG freq (kHz)   | `\d+`    | `851012`    |
| `#TGHZ`   | TG freq (Hz)    | `\d+`    | `851012500` |
| `#TGID`   | Talkgroup ID    | `\d+`    | `1234`      |
| `#TG`     | Talkgroup ID    | `\d+`    | `1234`      |
| `#SYS`    | System ID       | `\d+`    | `12`        |
| `#MHZ`    | Frequency (MHz) | `[\d.]+` | `851.012`   |
| `#KHZ`    | Frequency (kHz) | `\d+`    | `851012`    |
| `#HZ`     | Frequency (Hz)  | `\d+`    | `851012500` |

The mask is applied **after** the recorder-type parser runs. It only fills in fields that the parser left empty (except Date/Time, which the mask always overrides since a filename timestamp is more accurate than file modification time).
