#!/usr/bin/env bash
set -euo pipefail

# Fix ownership of GOPATH (named volume mounts default to root)
sudo chown -R vscode:vscode /home/vscode/go

echo "==> Installing backend Go dependencies..."
cd /workspaces/openscanner/backend
go mod download

echo "==> Generating sqlc code..."
sqlc generate || echo "  (sqlc generate skipped — queries may not be ready yet)"

echo "==> Installing frontend dependencies..."
cd /workspaces/openscanner/frontend
pnpm install

echo "==> Installing E2E dependencies..."
cd /workspaces/openscanner/e2e
pnpm install
npx -y playwright install --with-deps chromium

echo ""
echo "✔ Dev container ready. Run 'make dev' from the workspace root to start."
