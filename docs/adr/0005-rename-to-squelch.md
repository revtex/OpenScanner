# 0005. Rename to Squelch with compatibility shims

- **Status:** Accepted
- **Date:** 2026-09-19 (shipped as v2.0.0, 2026-09-18)

## Context

"OpenScanner" described the wrong thing: the project does not scan. It is the
archive and distribution layer for what a Recorder scanned. The name also
invited confusion with the Recorder software itself.

Renaming a self-hosted project is not a cosmetic change. The name is in the
binary, the container image, the default paths, the environment prefix, the Go
module path, and in state the browser and the CLI have already written to disk
on machines we do not control.

## Decision

The project is Squelch. The binary is `squelch`, the image is
`ghcr.io/revtex/squelch`, the database defaults to `squelch.db` under
`/var/lib/squelch`, configuration is read from `SQUELCH_*`, and the module path
is `github.com/revtex/squelch`.

Every piece of pre-existing state gets a forward path rather than a break:

- `OPENSCANNER_*` environment variables are still honoured, with one startup
  warning naming each one in use. `SQUELCH_*` wins when both are set.
- Squelch **refuses to start** on an OpenScanner data directory and prints the
  exact `mv` command, including the `-wal` and `-shm` files when they exist.
- Browser state — theme, paused state, Selection — is read from its pre-rename
  key once and migrated forward.
- The CLI token moved to `~/.squelch-token`; the old file is still read.
- The secrets encryption scheme is deliberately unchanged. See
  [0003](0003-secrets-at-rest.md).

There is no schema migration. It is a file rename and an environment-variable
rename.

## Consequences

- Upgrading is a breaking change for anyone running a container or a service
  unit, which is why it shipped as a major version.
- Every compatibility shim is code that exists only to be deleted later. The
  environment fallback in particular is documented as removable in a future
  release; nothing currently forces that to happen.
- The old name survives in two places on purpose: the crypto key-derivation
  inputs (see 0003) and the historical CHANGELOG entries.

## Alternatives considered

- **A hard break with a migration guide.** Rejected: the failure mode is
  silent. Starting with a fresh empty database beside the old one is
  indistinguishable from total data loss at a glance, and an operator who hits
  it at 2am has no way to tell which happened. The refuse-to-start guard exists
  specifically to make that case loud.
- **Silently renaming the data files on first start.** Rejected: it works until
  it doesn't, and it leaves no way back. Printing the command and stopping puts
  the operator in control of an irreversible step.
