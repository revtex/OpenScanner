# 0003. Secrets at rest with frozen key derivation

- **Status:** Superseded by [0006](0006-rotate-key-derivation-with-an-external-tool.md)
- **Date:** 2026-09-19 (recorded retroactively; the freeze itself was added 2026-09-18)

> **Superseded.** The freeze described here held through v2.x. v3.0.0 rotates
> the inputs deliberately, paired with the `squelch-rekey` tool — see
> [ADR 0006](0006-rotate-key-derivation-with-an-external-tool.md). The reasoning
> below is kept because it is why the rotation needed a tool rather than an edit.

## Context

All application configuration lives in the database, and some of it is secret:
the JWT signing secret, downstream API keys, Trunk Recorder broker passwords.
The database is a file an operator will copy, back up, and occasionally paste
into a support thread.

The encryption key is derived with HKDF-SHA256 from operator-supplied key
material. HKDF takes a salt and an info string, and both are **inputs to the
derivation**, not labels on it.

## Decision

Secrets are stored with an `enc::` prefix, encrypted with AES-256-GCM under a
key derived via HKDF-SHA256.

The HKDF salt and info string are **frozen**. They contain the string
`openscanner` and will keep containing it. `backend/internal/auth/crypto.go`
carries a comment saying so, and `TestKeyDerivationInputsAreFrozen` pins the
derived key against a golden value.

## Consequences

- An operator can hand someone a database file without handing over their JWT
  secret or broker passwords.
- The old project name is permanently visible in the crypto code. This looks
  like a rebrand miss and will keep looking like one. The comment and the test
  exist to stop the next person "fixing" it.
- Rotating the derivation inputs is possible only with a migration that
  decrypts under the old key and re-encrypts under the new one. Nobody should
  do that casually.

## Alternatives considered

- **Renaming the salt and info string during the Squelch rebrand.** Rejected,
  and this is the decision worth recording: changing either derives a different
  key, and every secret already stored with an `enc::` prefix becomes
  permanently undecryptable. There is no error at write time and no obvious
  symptom until something needs a secret back. A global find-and-replace would
  have done it silently, which is why the test exists rather than just the
  comment.
- **Encrypting the whole database (SQLCipher).** Not chosen: it moves the key
  problem to process start for every read, and the threat being addressed is a
  copied file, not a compromised host.
