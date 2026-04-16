# Recorder Setup Guide

Step-by-step setup for connecting recorders to OpenScanner.

## Prerequisites

1. OpenScanner reachable on your network
2. API key created in Admin -> API Keys (for HTTP upload recorders)
3. Systems/talkgroups created or autoPopulate enabled in Admin -> Options

## Trunk Recorder

### HTTP Upload (rdioscanner_uploader)

Configure plugin server URL to OpenScanner and provide API key per system.

Endpoint used:

- /api/trunk-recorder-call-upload (alias)
- /api/call-upload also accepted

### DirMonitor

Create monitor:

- type: trunk-recorder
- directory: recorder output path
- extension: optional (blank is fine)

Trunk recorder ingest expects sidecar JSON and matching audio stem.

## SDRTrunk

### HTTP Upload

Use SDRTrunk Rdio Scanner streaming target with:

- host: http://<openscanner>/api/call-upload
- API key: from OpenScanner
- system id: your OpenScanner system id

### DirMonitor

Create monitor:

- type: sdrtrunk
- directory: SDRTrunk recordings path
- extension: mp3 recommended

Parser reads ID3 metadata first, then filename fallback.

## RTLSDR-Airband

DirMonitor only.

Create monitor:

- type: rtlsdr-airband
- directory: output directory
- system id: required in practice
- talkgroup id: required in practice
- frequency: optional override

## DSDPlus

DirMonitor only.

Create monitor:

- type: dsdplus
- directory: parent directory containing date folders
- extension: mp3 or wav

System and talkgroup can be parsed from filenames, but config overrides are supported.

## ProScan

DirMonitor only.

Create monitor:

- type: proscan
- directory: recordings directory
- extension: wav (typical)
- optional mask for metadata extraction

## voxcall

HTTP upload only.

Use endpoint:

- /api/call-upload

Provide API key and required call metadata fields.

## Generic/Custom Recorder

If recorder type is unknown, use a DirMonitor type not mapped to known parsers (falls back to generic).

For generic ingest, configure at least:

- directory
- system id (or derivable mask)
- talkgroup id (or derivable mask)

Optionally configure mask and extension.

## Common Settings (Admin -> Options)

- autoPopulate
- audioConversion
- detectDuplicates
- duplicateTime
- apiKeyCallRate
- publicAccess
- shareableLinks

## Accepted Audio Extensions

- .mp3
- .wav
- .m4a
- .aac
- .ogg
- .flac
- .opus

## Mask Tokens

Supported tokens:

- #DATE
- #TIME
- #ZTIME
- #GROUP
- #SYSLBL
- #TAG
- #TGAFS
- #UNIT
- #TGLBL
- #TGMHZ
- #TGKHZ
- #TGHZ
- #TGID
- #TG
- #SYS
- #MHZ
- #KHZ
- #HZ

Mask parsing is applied after parser extraction and fills missing values.
