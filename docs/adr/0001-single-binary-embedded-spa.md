# 0001. Single binary with embedded SPA

- **Status:** Accepted
- **Date:** 2026-09-19 (recorded retroactively; decision predates this file)

## Context

Squelch is self-hosted, usually on a homelab box or a Raspberry Pi alongside the
Recorder feeding it. The people deploying it are radio operators, not
sysadmins. Every moving part in the deployment is a part that can be
misconfigured, and a reverse proxy in front of a separate frontend is the
classic way a self-hosted install ends up half-broken.

## Decision

We ship one executable. `make build` compiles the React app, embeds
`frontend/dist/` into the Go binary via `go:embed` (`backend/internal/static`),
and links it in. The Go process serves both the API and the SPA. There is no
separate web server in production.

## Consequences

- Deployment is "copy one file and run it". The release workflow publishes that
  file for five platforms.
- The frontend cannot be updated without rebuilding and redeploying the binary.
  Acceptable: they version together anyway.
- The binary carries the whole frontend, so it is not small.
- A stale browser bundle is a real failure mode even so — the service worker
  caches the shell, so "still broken after deploy" is often a cached bundle
  rather than a bad build. Confirm a hard reload before diagnosing further.

## Alternatives considered

- **Separate frontend served by nginx/Caddy.** Rejected: it doubles the number
  of things an operator has to configure correctly, and the most common support
  question becomes a proxy misconfiguration rather than anything about radio.
- **Serving the SPA from disk next to the binary.** Rejected: a path that can
  drift out of sync with the binary, and one more thing to get wrong when
  upgrading.
