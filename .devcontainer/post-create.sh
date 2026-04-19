#!/usr/bin/env bash
set -euo pipefail

# Fix ownership of GOPATH (named volume mounts default to root)
sudo chown -R vscode:vscode /home/vscode/go

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
