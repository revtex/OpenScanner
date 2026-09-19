# 0006. Rotate the key-derivation inputs with an external tool

- **Status:** Accepted
- **Date:** 2026-09-19
- **Supersedes:** [0003](0003-secrets-at-rest.md)

## Context

[ADR 0003](0003-secrets-at-rest.md) froze the HKDF salt and info string at
their pre-rebrand values (`openscanner`, `openscanner-secrets-v1`) because
changing them derives a different AES key and makes every stored `enc::`
secret unreadable. That was the right call when the alternative was doing it
silently during a global rename.

It left the old product name permanently embedded in the crypto, along with a
set of compatibility shims elsewhere — `OPENSCANNER_*` environment fallback, a
guard against the old database filename, a legacy CLI token path, and a browser
storage-key migration. Each one is a second code path that exists only to
support a name nobody uses any more.

The objection in 0003 was never that rotation is impossible. It was that a
rename must not *silently* rotate. A deliberate rotation, with a tool that
re-encrypts and a version bump that says so, is a different act.

## Decision

The derivation inputs become `squelch` and `squelch-secrets-v1`, and every
compatibility shim listed above is deleted.

Re-encryption is performed by **`squelch-rekey`** (`backend/cmd/rekey`), a
separate binary the operator runs once with the server stopped. The server
never re-encrypts secrets on its own.

The tool:

- Scans every text column of every table for `enc::` values, rather than a
  hard-coded list of the three places secrets live today — a column added later
  would otherwise be skipped silently.
- Classifies each value: readable under the current scheme (skip), readable
  under the pre-v3.0.0 scheme (re-encrypt), or neither (abort, naming
  `table.column.rowid`).
- Reports and exits by default. Writing requires `-apply`.
- Takes a backup with `VACUUM INTO` before writing, which is consistent even
  with a `-wal` beside the database.
- Rewrites in one transaction — all values or none.

The pre-v3.0.0 derivation lives **only** in the tool, so the server carries no
legacy crypto path at all.

The server gains one check, not a migration: `internal/secrets.CheckReadable`
refuses to start when a stored secret will not decrypt, naming the tool. Without
it the failure is scattered and misleading — a deployment supplying
`SQUELCH_JWT_SECRET` from the environment starts *successfully* and then fails
only at use time, as a broken downstream push and a Trunk Recorder instance that
will not connect.

## Consequences

- **Upgrading to v3.0.0 requires a manual step.** This is the cost, and it is
  why this is a major version. An operator who skips it gets a server that
  refuses to start with instructions, rather than one that half-works.
- `OPENSCANNER_*` environment variables stop being read. A compose file still
  using them will silently fall back to defaults — which is why the release
  notes lead with it.
- The CLI's pre-rename token file is no longer read: one re-login.
- Browser theme and paused state reset once per browser. Talkgroup selection is
  unaffected — it is stored server-side on the user, and the browser copy is
  only a cache.
- The guard against starting on an `openscanner.db` data directory is gone.
  Upgrading from v1.x directly to v3.0.0 will create an empty database beside
  the old one. The upgrade path is v1 → v2 → v3, or rename the file by hand.
- The old name survives in exactly one place: `cmd/rekey/legacy.go`. When no
  supported deployment still holds pre-v3.0.0 ciphertext, the tool is deleted
  rather than edited.

## Alternatives considered

- **Keep 0003 as-is.** Rejected: it was written to prevent an accident, and this
  is not one. Leaving it in place means keeping four compatibility paths and the
  old name in the crypto indefinitely, with nothing that would ever retire them.
- **Migrate automatically on first start.** Rejected, and this is the load-bearing
  choice. A process that silently rewrites every secret it finds is, from the
  outside, indistinguishable from one corrupting them — and if it guesses wrong
  about the key, it destroys the only copy. Doing it once, deliberately, with a
  dry run and a backup, is the difference between a migration and an accident.
- **Versioned ciphertext prefix (`enc2::`), supporting both schemes forever.**
  Rejected: it is the shim problem again with an extra branch in the hot path,
  and it never ends. A one-time tool has a finish line.
- **A shell or Python script instead of a Go binary.** Rejected: it would
  reimplement HKDF-SHA256 and AES-256-GCM in a second language, where a subtle
  divergence produces a key that is wrong but plausible. The tool imports the
  same `internal/auth` the server uses for the new scheme, and pins the old one
  with the golden key vector that `internal/auth` used to carry.
