#!/bin/sh
set -e

# Resolve paths from env (match Dockerfile defaults). The pre-rename
# OPENSCANNER_* names are still honoured here for the same reason the
# binary honours them: an operator's compose file should not silently
# stop taking effect because the project changed its name.
DB_FILE="${SQUELCH_DB_FILE:-${OPENSCANNER_DB_FILE:-/data/squelch.db}}"
REC_DIR="${SQUELCH_RECORDINGS_DIR:-${OPENSCANNER_RECORDINGS_DIR:-/data/recordings}}"
DB_DIR="$(dirname "$DB_FILE")"

# Ensure directories exist.
mkdir -p "$REC_DIR" "$DB_DIR"

# Fix ownership so appuser (1001) can write to bind-mounted dirs.
chown appuser:appuser "$DB_DIR" "$REC_DIR" 2>/dev/null || true
chown appuser:appuser "$DB_FILE" 2>/dev/null || true

# Drop privileges and exec the Go binary.
exec su-exec appuser ./squelch "$@"
