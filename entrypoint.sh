#!/bin/sh
set -e

# Resolve paths from env (match Dockerfile defaults).
DB_FILE="${OPENSCANNER_DB_FILE:-/data/openscanner.db}"
REC_DIR="${OPENSCANNER_RECORDINGS_DIR:-/data/recordings}"
DB_DIR="$(dirname "$DB_FILE")"

# Ensure directories exist.
mkdir -p "$REC_DIR" "$DB_DIR"

# Fix ownership so appuser (1001) can write to bind-mounted dirs.
chown appuser:appuser "$DB_DIR" "$REC_DIR" 2>/dev/null || true
chown appuser:appuser "$DB_FILE" 2>/dev/null || true

# Drop privileges and exec the Go binary.
exec su-exec appuser ./openscanner "$@"
