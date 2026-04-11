# OpenScanner — Recorder Integration Guide

> **Partial.** Call upload endpoints (`POST /api/call-upload`, `POST /api/trunk-recorder-call-upload`) are fully implemented. DirWatch (directory-based file ingestion) is planned for a future phase.

## HTTP API Upload

Recorders that support HTTP call upload can send calls to OpenScanner using:

- `POST /api/call-upload` — general-purpose multipart upload (SDRTrunk, voxcall, etc.)
- `POST /api/trunk-recorder-call-upload` — alias for the above, accepting identical fields

Both endpoints require an `X-API-Key` header (or `?key=` query parameter). See [API Reference](api.md) for full request/response details.

## Planned Recorder Support

| Recorder          | HTTP API                             | DirWatch |
| ----------------- | ------------------------------------ | -------- |
| Trunk Recorder    | POST /api/trunk-recorder-call-upload | Planned  |
| SDRTrunk          | POST /api/call-upload                | Planned  |
| RTLSDR-Airband    | —                                    | Planned  |
| DSDPlus Fast Lane | —                                    | Planned  |
| ProScan           | —                                    | Planned  |
| voxcall           | POST /api/call-upload                | —        |
