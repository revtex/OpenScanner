# Changelog

All notable changes to OpenScanner will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- Refresh-token rotation now tolerates a 30-second replay grace window for already-rotated tokens (per OAuth 2.0 Security BCP §4.13). When the same refresh cookie is presented twice within the window — parallel browser tabs, service-worker retries, page reload mid-rotation, or any scenario the per-tab single-flight cannot cover — the server returns the cached successor tokens issued on the original rotation instead of revoking the entire token family. Replays after the grace window still revoke the family, preserving the theft-detection signal.

### Fixed

- Audio-element auth recovery now shares the same single-flighted `/api/v1/auth/refresh` promise as the rest of the app. Previously, a 401 on an `<audio>` fetch fired a parallel refresh that bypassed the RTK Query coalescing introduced in 1.3.1; if it raced the scheduled refresh it would replay the single-use refresh cookie and revoke the token family, logging the user out roughly every 25 minutes (and randomly when audio playback failed near a refresh tick).

## [1.3.1] — 2026-04-29

### Fixed

- Idle browser tabs no longer get logged out when the access JWT expires. The frontend now single-flights `POST /api/v1/auth/refresh` so simultaneous 401s on tab wake (config, calls list, listener TG selection, WebSocket reauth, scheduled refresh, etc.) coalesce onto one network request. Previously, parallel refresh attempts presented the same single-use refresh token; the server treated the second attempt as a replay and revoked the entire token family, forcing a re-login despite the 30-day refresh cookie.
- Release workflow now creates the GitHub Release when a tag is pushed without one, instead of failing with `release not found` when uploading binaries. Future `git push --tags` releases publish artifacts without a manual `gh release create` step.

## [1.3.0] — 2026-04-29

### Added

- Native `/api/v1/*` REST surface alongside the existing legacy routes. All v1 responses use a structured error envelope (`{"error":{"code","message","details"}}`) with stable string codes (`validation_failed`, `unauthorized`, `forbidden`, `not_found`, `conflict`, `unprocessable`, `rate_limited`, `internal`); 5xx envelopes include the request ID under `details.requestId`.
- v1 call-upload endpoint (`POST /api/v1/calls`) with native multipart field names (`systemId`, `talkgroupId`, `startedAt`, `frequencyHz`, `durationMs`, `unitId`) and RFC 3339 `startedAt` enforcement (unix timestamps no longer accepted on v1). Companion `POST /api/v1/calls/test` returns 204 on a valid API key.
- v1 listener endpoints: `GET/PUT /api/v1/listener/tg-selection` (renamed from `/api/auth/tg-selection`), `GET /api/v1/calls`, `GET /api/v1/calls/:id/audio`, `GET /api/v1/calls/:id/transcript`, share/bookmark endpoints, and unauthenticated `/api/v1/health`, `/api/v1/setup/*`, `/api/v1/auth/{login,refresh,logout,password,me}`.
- v1 admin endpoints under `/api/v1/admin/*` for talkgroup/unit/group/tag imports, RadioReference preview (path simplified — no `/csv` suffix), transcription status, and Swagger session bootstrap.
- Native JSON-object framed WebSocket protocol on `GET /api/v1/ws/listener` and `GET /api/v1/ws/admin`. Frames carry a `type` discriminator (`connection.welcome`, `scanner.config`, `call.new`, `call.transcript`, `listener.count`, `listener.feedMap.snapshot`/`update`, `session.expired`, `connection.rejected`, `admin.event`, `admin.request`, `admin.response`) instead of the legacy 3-letter array opcodes. Admin error responses mirror the REST `{code,message,details?}` envelope. The frontend connects to the v1 paths; legacy `/ws`, `/api/ws`, and `/api/admin/ws` keep emitting the array-framed protocol unchanged for in-the-wild clients.
- RFC 8594 deprecation headers (`Deprecation: true`, `Sunset`, `Link: <successor>; rel="successor-version"`, `Cache-Control: no-store`) on every legacy `/api/*` and legacy WebSocket route, pointing at the native `/api/v1/*` successor. Per-request structured warn log (`legacy endpoint hit`) records method, path, and a truncated API-key identifier — never the raw key.
- Admin endpoint `GET /api/v1/admin/legacy-usage` returning a 24-hour aggregate of legacy-endpoint hits (`{method, path, apiKeyIdent, count, lastSeen}`), backed by an in-memory ring buffer (no schema change).
- Admin dashboard banner that surfaces legacy-API usage from the new endpoint, with an expandable details table (method, path, API key, count, last seen) and per-session dismiss.

### Changed

- Frontend now talks to the native `/api/v1/*` surface for every REST call (RTK Query base URL, raw `fetch()` for audio downloads and silent token refresh, the service-worker passthrough rules, the dev-server proxy, and the Swagger UI bootstrap). Tg-selection moves from `/api/auth/tg-selection` to `/api/v1/listener/tg-selection`; RadioReference CSV preview moves to `/api/v1/admin/radioreference/preview`; legacy-usage report is consumed at `/api/v1/admin/legacy-usage`. Legacy `/api/*` routes remain available for non-frontend clients with the existing deprecation headers.
- API-key authentication on `/api/v1/*` upload routes accepts only `Authorization: Bearer <api-key>`; the legacy `X-API-Key` header, `?key=` query parameter, and `key=` form field continue to work on legacy routes only. JWT-shaped Bearer tokens on v1 API-key routes are rejected with `invalid_credentials`.
- Swagger UI now documents every native `/api/v1/*` endpoint, not just the three previously annotated handlers. Legacy `/api/*` annotations remain in place until those routes are retired.

## [1.2.1] — 2026-04-25

### Security

- Pin transitive `postcss` to `>=8.5.10` via a pnpm override to address GHSA / CVE: "PostCSS has XSS via Unescaped `</style>` in its CSS Stringify Output" (medium). PostCSS is a dev-only dependency pulled in by Vite/Tailwind and never reaches the production runtime, but the override removes the Dependabot alert and ensures contributors building from source pick up the patched version.

## [1.2.0] — 2026-04-25

### Added

- Session cookie (`os_session`) issued on login and refresh, cleared on
  logout. The `GET /api/calls/:id/audio` route now accepts authentication
  via either the existing `Authorization: Bearer` header or the new
  cookie, so `<audio>` element playback can be authenticated without
  client-side header injection. Cookie is httpOnly, Secure when served
  over HTTPS, `SameSite=Strict`, scoped to `/api`. Cross-site requests
  are rejected via a `Sec-Fetch-Site` check; invalid or expired cookies
  fall through to anonymous so existing `publicAccess=true` deployments
  continue to work unchanged.
- Canonical `GET /api/ws` listener WebSocket route. The existing `GET /ws`
  remains as a compatibility alias that delegates to the same handler. The
  frontend now connects to `/api/ws`, and the Vite dev proxy covers both
  paths.

### Changed

- WebSocket `CAL` messages no longer carry embedded base64 audio. Audio
  is fetched on demand from the existing `GET /api/calls/:id/audio` HTTP
  endpoint, authenticated via the `os_session` cookie introduced in the
  previous release entry. Frontend playback rewrite ships as the
  immediately-following release entry.
- HTTP handlers have been decomposed from the monolithic `internal/api`
  package into feature-scoped subpackages under `internal/handler/`
  (`auth`, `calls`, `bookmarks`, `share`, `setup`, `health`,
  `admin/{imports,radioreference,transcriptions}`). Route registration
  now lives in `internal/handler/routes`, and shared swagger DTOs and
  helpers live in `internal/handler/shared`. No route paths, methods,
  middleware ordering, response shapes, or status codes changed.
- Backend file-level cleanup: `internal/handler/calls/calls.go` (~1500 LOC)
  split into `upload.go`, `audio.go`, `search.go`, `transcript.go`, and a
  slim `calls.go` retaining the `Handler` struct and constructor;
  `internal/middleware/middleware.go` split into `cors.go`, `auth.go`,
  `logging.go`, `limits.go`. Same package, same exports, no behaviour
  change.
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
- Frontend audio playback now uses the platform `<audio>` element backed
  by `MediaElementAudioSourceNode`. Each call is fetched on demand from
  the existing `/api/calls/:id/audio` endpoint authenticated via the
  session cookie. Drops the WebSocket-embedded base64 path, fixing
  Mobile Edge AAC playback and dramatically reducing per-call memory
  pressure on the client.
- Service worker now passes audio fetches (`/api/calls/:id/audio` and
  `/api/shared/:token/audio`) straight through to the network instead of
  intercepting them. Lets the browser handle Range requests natively for
  the `<audio>` element and avoids buffering full call bodies in the
  worker.

### Fixed

- Swagger UI now opens correctly from the admin Tools panel. The short-lived `os_swagger` session cookie was scoped to `/api/admin/docs`, so the browser refused to send it on the v1 docs URL (`/api/v1/admin/docs/index.html`) and the docs route returned `swagger session required`. The cookie path is now `/api`, covering both legacy and v1 docs routes.
- Lock the primary admin's Allowed Systems selector in the user editor; the first user always has access to every system and the badges are now read-only with all systems shown as allowed.
- Default `audioEncodingPreset` seeded into the settings table is now
  `mp3_32k` (matching the dropdown's "(default)" label and the Go
  `ParseEncodingPreset` fallback) instead of `aac_lc_32k`. New installs
  enabling audio conversion will now default to MP3 32 kbps as the UI
  advertises.
- Audio playback now silently recovers from a 401 on `/api/calls/:id/audio`
  by triggering a single token refresh and retrying the same call. Fixes
  the case where a sibling device login (phone, tablet, second tab) pushes
  a desktop's access JWT out of the per-user concurrent-token cap and
  leaves \<audio\> playback failing until the next scheduled refresh.
- Per-user concurrent JWT cap raised from 5 to 20 (`auth.MaxRefreshFamilies`).
  With a 15-minute access TTL refreshing roughly four times per hour, the
  old limit pushed a desktop's session off the active list within an hour
  of normal multi-device use. 20 leaves headroom for desktop + phone +
  tablet without inflating the deny list.
- WebSocket clients (listener and admin) now detach handlers and wait for
  `open` before closing a still-`CONNECTING` socket during reconnect.
  Suppresses the cosmetic "WebSocket is closed before the connection is
  established" browser console warning that appeared after a token-expiry
  reconnect.

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
