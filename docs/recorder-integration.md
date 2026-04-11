# OpenScanner — Recorder Integration Guide

> **Planned — not yet implemented.** Call upload endpoints (`POST /api/call-upload`, `POST /api/trunk-recorder-call-upload`) and DirWatch are stub code only. This guide will be written when those features are built in Phase 4+.

## Planned Recorder Support

| Recorder          | HTTP API                             | DirWatch |
| ----------------- | ------------------------------------ | -------- |
| Trunk Recorder    | POST /api/trunk-recorder-call-upload | ✓        |
| SDRTrunk          | POST /api/call-upload                | ✓        |
| RTLSDR-Airband    | —                                    | ✓        |
| DSDPlus Fast Lane | —                                    | ✓        |
| ProScan           | —                                    | ✓        |
| voxcall           | POST /api/call-upload                | —        |
