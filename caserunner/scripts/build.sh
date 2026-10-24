#!/usr/bin/env bash
# Builds the frontend, embeds it in the Go binary, and writes backend/bin/caserunner.
#
# Usage: scripts/build.sh [--run]
#   --run      start the tool right after the build (Ctrl+C stops it); an
#              API Case Runner already listening on that port is stopped first
#   VERSION=1.2.3 scripts/build.sh                  stamp a version
#   GOOS=windows GOARCH=amd64 scripts/build.sh      backend/bin/caserunner.exe for Windows
# CASERUNNER_* settings given before the command apply to --run.
set -euo pipefail

usage() { sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; }

run=false
for arg in "$@"; do
  case "$arg" in
    --run) run=true ;;
    -h | --help) usage; exit 0 ;;
    *) echo "error: unknown option $arg" >&2; usage >&2; exit 2 ;;
  esac
done

root="$(cd "$(dirname "$0")/.." && pwd)"
web="$root/backend/internal/adapter/web/dist"

command -v go >/dev/null || { echo "error: go was not found on PATH; install Go 1.27+ from https://go.dev/dl/" >&2; exit 1; }
if $run && [ "$(go env GOOS)/$(go env GOARCH)" != "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" ]; then
  echo "error: --run cannot start a binary built for $(go env GOOS)/$(go env GOARCH) on this machine" >&2
  exit 2
fi

# The port --run will listen on (CASERUNNER_BIND, default 127.0.0.1:8080).
bind="${CASERUNNER_BIND:-127.0.0.1:8080}"
port="${bind##*:}"

# port_pids lists the PIDs listening on TCP $port (none when it is free).
port_pids() {
  if command -v lsof >/dev/null; then
    lsof -nP -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u
  elif command -v ss >/dev/null; then
    ss -ltnpH "sport = :$port" 2>/dev/null | grep -o 'pid=[0-9]*' | cut -d= -f2 | sort -u
  fi
}

# find_running sets running_pids to the API Case Runner processes on $port.
# Any other program there stops the build: it is never killed.
find_running() {
  running_pids=()
  local pid name
  for pid in $(port_pids); do
    name="$(ps -o comm= -p "$pid" 2>/dev/null || true)"
    if [ "$name" = caserunner ]; then
      running_pids+=("$pid")
    else
      echo "error: port $port is used by ${name:-another program} (PID $pid), not API Case Runner" >&2
      echo "Stop that program, or choose another port, e.g. CASERUNNER_BIND=127.0.0.1:9090 scripts/build.sh --run" >&2
      exit 1
    fi
  done
}

# stop_running stops each process in running_pids: Ctrl+C first so it shuts
# down cleanly, then a hard kill if it is still there after 10 seconds.
stop_running() {
  local pid
  for pid in ${running_pids[@]+"${running_pids[@]}"}; do
    echo "==> stopping the running API Case Runner (PID $pid)"
    if ! kill -INT "$pid" 2>/dev/null; then
      echo "error: cannot stop PID $pid (started by another user?); stop it yourself and run again" >&2
      exit 1
    fi
    for _ in $(seq 50); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.2
    done
    if kill -0 "$pid" 2>/dev/null; then kill -KILL "$pid"; fi
  done
}

# Fail before the build when the port is taken by something else.
if $run; then find_running; fi

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
bin="bin/caserunner$(go env GOEXE)" # .exe when GOOS=windows
go build -ldflags "-X main.version=${VERSION:-dev}" -o "$bin" ./cmd/caserunner

echo "Built $root/backend/$bin"
if $run; then
  find_running # again: an instance may have started during the build
  stop_running
  echo "==> run (Ctrl+C to stop)"
  exec "./$bin"
fi
echo "Run it and open http://127.0.0.1:8080, or build and run in one go with: scripts/build.sh --run"
