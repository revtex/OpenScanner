#!/usr/bin/env bash
set -euo pipefail

# Fix ownership of named volumes (they mount as root by default)
sudo chown -R vscode:vscode /home/vscode/go
sudo chown -R vscode:vscode /home/vscode/.local/share/pnpm

# pnpm virtual-store-dir (see frontend/.npmrc) — persistent across /tmp wipes
mkdir -p /home/vscode/.cache/pnpm-vstore

echo "==> Installing backend Go dependencies..."
cd /workspaces/OpenScanner/backend
go mod download

echo "==> Generating sqlc code..."
cd /workspaces/OpenScanner/backend/sqlc
sqlc generate || echo "  (sqlc generate skipped — queries may not be ready yet)"

echo "==> Installing frontend dependencies..."
cd /workspaces/OpenScanner/frontend

pnpm install

echo ""
echo "Dev container ready. Run 'make dev' from the workspace root to start."
