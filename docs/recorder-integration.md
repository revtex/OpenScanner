# OpenScanner Recorder Integration

This guide describes recorder ingest behavior and service internals.

For recorder-specific setup steps, use docs/recorder-setup.md.

## Upload Endpoints

- POST /api/call-upload
- POST /api/trunk-recorder-call-upload

Both routes execute the same ingest handler.

Auth:

- X-API-Key header (preferred)
- key multipart field compatibility

## Supported Ingest Modes

1. HTTP upload by recorder
2. DirMonitor file ingestion from local/shared directories

## Recorder Support Matrix

| Recorder | HTTP Upload | DirMonitor |
| --- | --- | --- |
| Trunk Recorder | Yes | Yes |
| SDRTrunk | Yes | Yes |
| RTLSDR-Airband | No | Yes |
| DSDPlus | No | Yes |
| ProScan | No | Yes |
| voxcall | Yes | No |

## DirMonitor Service Behavior

DirMonitor watches configured directories and feeds parsed calls into the same ingest pipeline used by HTTP upload.

Flow:

1. Detect file (fsnotify or polling mode)
2. Validate file path and optional extension filter
3. Parse metadata based on configured recorder type
4. Apply optional mask extraction (fills missing fields only)
5. Validate file size and timestamp
6. Resolve system/talkgroup (with autoPopulate as configured)
7. Deduplicate, process audio, store call, broadcast WS events
8. Optionally delete source files

Config updates trigger service reload without server restart.

## Parser Types

Known parser types:

- trunk-recorder
- sdrtrunk (or sdr-trunk)
- rtlsdr-airband
- dsdplus
- proscan
- unknown type falls back to generic parser

## Important Parser Notes

- trunk-recorder expects audio + JSON sidecar pairing
- sdrtrunk prefers MP3 ID3 metadata, then filename fallback
- rtlsdr-airband relies on dirmonitor config IDs and filename timestamps
- dsdplus parses metadata from folder/file naming conventions
- proscan is mostly config/mask driven
- generic parser relies on config overrides and optional mask

## Mask Behavior

The mask is matched against the filename stem (basename without extension).

Mask extraction only fills zero/empty fields and does not overwrite parser or explicit config values.

See docs/recorder-setup.md for token reference and examples.
