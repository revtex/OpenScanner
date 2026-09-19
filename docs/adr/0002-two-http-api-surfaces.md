# 0002. Two HTTP API surfaces

- **Status:** Accepted
- **Date:** 2026-09-19 (recorded retroactively; decision predates this file)

## Context

Squelch is a from-scratch reimplementation of rdio-scanner. Its users already
run Recorders configured to upload to rdio-scanner's API. If those uploads stop
working, migrating means reconfiguring every Recorder — which for a multi-site
setup is the difference between trying Squelch and not.

At the same time, rdio-scanner's error shape (`{"error":"<string>"}`) and its
flat namespace are not what we want to build the rest of the product on.

## Decision

Two surfaces:

- **`/api/v1/*` is canonical.** It is what the SPA uses. Its error envelope is
  `{"error":{"code","message","details"}}` with stable string codes. All new
  endpoints go here.
- **`/api/*` is deprecated** and exists only for Recorder upload
  compatibility. It emits RFC 8594 `Deprecation` / `Sunset` headers via
  `middleware.Deprecated`, and legacy error bodies are rewritten into the v1
  envelope by `middleware.V1ErrorEnvelope`.

## Consequences

- Existing rdio-scanner Recorders work against Squelch with a URL change and
  nothing else.
- Every legacy route is a route we have to keep working, and the deprecation
  headers are the only pressure toward retiring them.
- Two surfaces means two places a route can be added by mistake. The rule is
  absolute — new endpoints are v1-only — precisely because the exception would
  be invisible.

## Alternatives considered

- **Only the rdio-scanner API.** Rejected: it constrains everything the admin
  dashboard and the listener client need to do, forever, for the sake of one
  ingest path.
- **Only v1, with a migration guide.** Rejected: it makes adoption cost
  proportional to how many Recorders someone runs, which penalises exactly the
  users with the most to gain.
