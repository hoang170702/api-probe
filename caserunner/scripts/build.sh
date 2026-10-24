#!/usr/bin/env bash
# Builds the frontend, embeds it in the Go binary, and writes backend/bin/caserunner.
# Usage: scripts/build.sh            (VERSION=1.2.3 scripts/build.sh to stamp a version)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
web="$root/backend/internal/adapter/web/dist"

need_node() {
  echo "error: $1" >&2
  echo "Install Node.js 22 inside Linux/WSL, e.g. with nvm (https://github.com/nvm-sh/nvm):" >&2
  echo "  curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash" >&2
  echo "  # open a new terminal, then:" >&2
  echo "  nvm install 22" >&2
  exit 1
}

# Under WSL, a Windows Node.js on PATH (/mnt/c/...) runs npm through cmd.exe,
# which cannot use \\wsl.localhost paths: the build then fails obscurely.
node_bin="$(command -v node || true)"
npm_bin="$(command -v npm || true)"
[ -n "$node_bin" ] || need_node "node was not found on PATH${npm_bin:+ (npm at $npm_bin is the Windows one)}"
case "$node_bin$npm_bin" in
  */mnt/[a-z]/*) need_node "the Windows Node.js is on PATH ($node_bin, $npm_bin); the build needs a Linux Node.js" ;;
esac
node -e 'const [a, b] = process.versions.node.split(".").map(Number); process.exit(a > 22 || (a === 22 && b >= 12) || (a === 20 && b >= 19) ? 0 : 1)' ||
  need_node "Node.js $(node -v) is too old; Vite needs 20.19+ or 22.12+"

echo "==> frontend"
cd "$root/frontend"
[ -d node_modules ] || npm ci
npm run build

echo "==> embed"
find "$web" -mindepth 1 ! -name .gitkeep -delete
cp -r "$root/frontend/dist/." "$web/"

echo "==> backend"
cd "$root/backend"
go build -ldflags "-X main.version=${VERSION:-dev}" -o bin/caserunner ./cmd/caserunner

echo "Built $root/backend/bin/caserunner"
echo "Run it and open http://127.0.0.1:8080"
