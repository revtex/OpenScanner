# Changelog

All notable changes to OpenScanner will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Canonical `GET /api/ws` listener WebSocket route. The existing `GET /ws`
  remains as a compatibility alias that delegates to the same handler;
  retirement of the alias is tracked in the native-API design plan. The
  frontend now connects to `/api/ws`, and the Vite dev proxy covers both
  paths.

### Changed

- HTTP handlers have been decomposed from the monolithic `internal/api`
  package into feature-scoped subpackages under `internal/handler/`
  (`auth`, `calls`, `bookmarks`, `share`, `setup`, `health`,
  `admin/{imports,radioreference,transcriptions}`). Route registration
  now lives in `internal/handler/routes`, and shared swagger DTOs and
  helpers live in `internal/handler/shared`. No route paths, methods,
  middleware ordering, response shapes, or status codes changed.
- Admin CRUD business logic has been extracted from `internal/ws` into a
  new transport-agnostic `internal/admin` package. The WebSocket layer
  now only routes `ADM_REQ` frames to `admin.Operations` methods; the
  wire protocol, action names, and response shapes are unchanged.
- Deployment guide reverse-proxy instructions now list `/api/ws` alongside
  `/ws` and `/api/admin/ws` as paths that need WebSocket-upgrade forwarding.
- Admin Options panel no longer shows an "Active" badge on every wired
  setting; only "Planned" badges are rendered for not-yet-implemented
  options.
- Admin Options "Audio Conversion" description now reads "Convert incoming
  audio with FFmpeg before storing. Select the codec and bitrate below."
  to reflect that MP3 and AAC outputs are both supported via the encoding
  preset.
- Frontend `services/` directory grouped into `services/ws/` (`client.ts`,
  `client.test.ts`, `adminClient.ts`) and `services/audio/` (`player.ts`,
  `beep.ts`). All `@/services/*` imports across components and hooks have
  been updated to the new paths. No runtime behaviour change.
- Frontend `hooks/` directory split into `hooks/shared/` (`useAuthInit`,
  `useTheme`, `useTokenRefresh`, `useWebSocket`), `hooks/scanner/`
  (`useScanner`, `useAudioPlayer`, `useTGSelectionSync`, `useActiveUnit`),
  and `hooks/admin/` (`useAdminWebSocket`, `useAdminWsOps`,
  `useAdminActivity`, `useAdminLogs`, `useWsQuery`), each with a barrel
  `index.ts`. All call sites have been updated to the new specific paths.
  No runtime behaviour change.
- Frontend `types/index.ts` god-file split into topic-scoped modules
  (`call.ts`, `config.ts`, `ws.ts`, `auth.ts`, `api.ts`, `admin.ts`,
  `ui.ts`). The original `index.ts` is now a barrel that re-exports
  everything, so all existing `@/types` imports keep working unchanged.
  New code can also import from a specific module (e.g. `@/types/admin`).
- Frontend layout polish on top of the directory restructure:
  `app/slices/` split into `shared/` (`authSlice`), `scanner/`
  (`scannerSlice`, `callsSlice`, `shareSlice`), and `admin/`
  (`adminSlice`, `activitySlice`); `components/admin/AdminLayout.tsx`
  inlined into `pages/Admin.tsx` (replacing the 5-line shim);
  `components/admin/NavigationGuardContext.tsx` relocated to
  `hooks/admin/useNavigationGuard.tsx`; and `services/downloadFilename.ts`
  moved to `services/util/downloadFilename.ts`. All call sites updated;
  no runtime behaviour change.

### Fixed

- Default `audioEncodingPreset` seeded into the settings table is now
  `mp3_32k` (matching the dropdown's "(default)" label and the Go
  `ParseEncodingPreset` fallback) instead of `aac_lc_32k`. New installs
  enabling audio conversion will now default to MP3 32 kbps as the UI
  advertises.

## [1.1.2] — 2026-04-24

### Security

- Call upload now reads audio back through `os.Root` when embedding it in
  the WebSocket broadcast, ensuring the read is confined to the
  recordings directory regardless of the stored path. Addresses a Snyk
  path-traversal taint warning on `os.ReadFile`.
- `GET /api/calls/:id/audio` now opens the recording through `os.Root`
  and streams via `http.ServeContent` instead of letting `c.File` touch
  a joined absolute path. Addresses a Snyk path-traversal taint warning
  where the DB-stored `audio_path` reached `c.File` after only string
  sanitisation.
- `GET /api/shared/:token/audio` uses the same `os.Root` + `ServeContent`
  pattern so the shared-link download path is also confined to the
  recordings directory.
- `openscanner upgrade --binary <path>` now resolves both the source and
  destination paths to absolute cleaned form before any filesystem
  operation, and rejects a source that isn't a regular file before
  opening it. The operator already has full authority here, but the
  validation short-circuits obvious mistakes (directories, device nodes,
  broken symlinks) and addresses Snyk CLI-input path-traversal warnings
  on `os.Open` and `os.Remove`.
- Dirmonitor `delete_after=1` cleanup now deletes via `os.Root.Remove`
  scoped to the watched directory. The existing symlink-resolve + `Rel`
  escape check is retained as defence-in-depth; the structural root
  bound ensures no file outside the watched directory can ever be
  removed regardless of parser output. Addresses Snyk path-traversal
  taint warnings on the dirmonitor cleanup path.
- Dirmonitor ingest now reads the just-ingested audio back through
  `os.Root` when embedding it in the WebSocket broadcast frame,
  confining the read to the recordings directory. Addresses a Snyk
  path-traversal taint warning on `os.ReadFile`.
- The `openscanner` CLI now validates the `--server` /
  `OPENSCANNER_SERVER` URL before it reaches `net/http`: the string
  must parse, use an `http` or `https` scheme, and carry a non-empty
  host. Userinfo and fragments are stripped. The CLI only ever talks to
  a URL the operator supplied, but the explicit validation shuts down
  Snyk SSRF taint warnings and turns typos into a clear error message.
- The bookmarks download button now sanitises the server-supplied
  `audioName` before assigning it to `<a download>` — path separators,
  control characters, quote/angle-bracket characters, and leading dots
  are stripped, and the result is capped at 200 chars. Addresses a Snyk
  DOM-XSS taint warning and also yields safer filenames on Windows.
- The search-panel download button uses the same `sanitizeDownloadFilename`
  helper, extracted to `services/downloadFilename.ts` so both call sites
  share one implementation. Addresses the same Snyk DOM-XSS finding for
  `SearchPanel.tsx`.

## [1.1.1] — 2026-04-24

### Changed

- Docker images are now built for both `linux/amd64` and `linux/arm64`,
  so `docker pull` works on Apple Silicon, Raspberry Pi, and other
  arm64 hosts.
- Docker image tagging no longer produces `sha-<short-sha>` tags on every
  push. Published images now carry only semver (`1.1.1`, `1.1`, `latest`)
  and branch (`main`, `dev`) tags, so `ghcr.io/revtex/openscanner:dev` is
  the canonical pre-release channel.
- New weekly `GHCR cleanup` workflow prunes leftover untagged and
  `sha-*`-only container versions from GHCR.

## [1.1.0] — 2026-04-23

### Added

- Commit GitHub ruleset definitions under `.github/rulesets/` so branch
  and tag protection policy is versioned with the code.
- Release workflow now builds standalone binaries for Linux, macOS, and
  Windows (amd64 + arm64 where applicable) on every `v*` tag and
  attaches them to the GitHub Release alongside a `SHA256SUMS.txt`.
- Release archives ship the user guides (README, admin, deployment,
  recorder) as styled PDFs, and the same PDFs are attached to the
  GitHub Release as standalone downloads.

### Fixed

- PDF user guides rendered code blocks with ~50pt of phantom left
  padding and let long lines overflow the right margin. Pandoc's
  built-in skylighting CSS is now neutralised so code aligns with the
  block's left edge and wraps cleanly.

## [1.0.0] — 2026-04-23

Initial public release. OpenScanner is a ground-up reimplementation of
[rdio-scanner](https://github.com/chuot/rdio-scanner) as a single Go binary
with an embedded React frontend. Backward compatible with existing
rdio-scanner upload clients (Trunk-Recorder `rdioscanner_uploader`, SDRTrunk
Rdio Scanner streaming target).

### Added

- **Scanner interface** — live WebSocket streaming with play/pause, skip,
  replay, hold, avoid (5/15/30 min or indefinite), per-user talkgroup
  selection, bookmarks, public call sharing with configurable expiry, live
  transcript display, dark/light theme, mobile-responsive layout with
  virtualized lists.
- **Call ingest** — HTTP upload (`/api/call-upload` + backward-compatible
  `/api/trunk-recorder-call-upload`) and directory monitoring with native
  support for Trunk-Recorder, SDRTrunk, DSDPlus, RTLSDR-Airband, ProScan,
  and generic mask-based sources.
- **Auto-populate** — systems, talkgroups, groups, tags, and units created
  automatically from incoming call metadata.
- **Audio processing** — FFmpeg integration with four conversion modes
  (disabled, enabled, normalize, loudnorm) and 8 encoding presets across
  MP3, AAC-LC, and HE-AAC.
- **Transcription** — optional integration with a
  [go-whisper](https://github.com/mutablelogic/go-whisper) sidecar for
  automatic call transcription with in-UI model management, speaker
  diarization (tinydiarize models), 15 languages plus auto-detect, and GPU
  acceleration.
- **Admin dashboard** — CRUD for users, systems, talkgroups, units, groups,
  tags, API keys, directory monitors, downstreams, shared links, webhooks,
  and settings. Log viewer with level/date/text filters and runtime level
  control. JSON config export/import. CSV import/export for talkgroups and
  units. RadioReference metadata preview and import.
- **Authentication** — JWT login with refresh-token rotation (family
  revocation on reuse), bcrypt password hashing (cost ≥ 12), role-based
  access control (admin/listener), per-user talkgroup selection, session
  limits, account expiration, password-change enforcement.
- **Rate limiting** — per-IP login with 3-strike lockout, per-user share
  creation, per-API-key sliding-window upload limits, per-IP shared-link
  access.
- **Secrets at rest** — optional AES-256-GCM encryption for the JWT signing
  secret and downstream API keys, keyed from `OPENSCANNER_ENCRYPTION_KEY`.
- **TLS** — certificate/key file configuration with HTTP → HTTPS redirect
  and experimental Let's Encrypt auto-cert.
- **Outbound HTTP hardening** — transcription and downstream traffic go
  through a shared client with redirects disabled, timeouts enforced, and
  response bodies capped. Private-network targets allowed by default;
  gateable with `OPENSCANNER_BLOCK_INTERNAL_HTTP=1`.
- **Deployment** — single binary, embedded SQLite (WAL), pre-built Docker
  image with FFmpeg. Guided `openscanner setup --interactive` for
  bare-metal installs; `upgrade`, `config validate`, and `service doctor`
  subcommands. Cross-platform service management (systemd / SysV / OpenRC
  / launchd / Windows SCM).
- **Documentation** — deployment guide, admin guide, recorder integration
  guide, architecture overview, API reference.

### Known limitations

- Let's Encrypt auto-cert is experimental and not yet exercised in
  production.
- Downstream forwarding between OpenScanner instances is experimental and
  untested.
- Transcription requires a separately deployed go-whisper sidecar.

[Unreleased]: https://github.com/revtex/OpenScanner/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/revtex/OpenScanner/releases/tag/v1.0.0
