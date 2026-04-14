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
