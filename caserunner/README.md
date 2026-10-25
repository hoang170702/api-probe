# API Case Runner

Paste a `curl`, define test cases in a table, fire them in parallel against the API, and export the results.

- How to use it: [user guide](../docs/user-guide.md) ([Tiếng Việt](../docs/user-guide.vi.md))
- Design and requirements: [BRD-001](../docs/brd/BRD-001.md) ([Tiếng Việt](../docs/brd/BRD-001.vi.md))

Status: Steps 0–9 and 3b of the implementation plan (BRD-001 §18) — optional token steps (with Try), paste a curl, see the request and every field of its JSON body with a Required/Unrequired rule, enter cases with an expect code, run them with live results over SSE, cancel and run again, export/import the configuration; UI in Vietnamese and English. Excel/PDF reports (Step 10) come next.

## Requirements

| Tool | Version |
|---|---|
| Go | 1.27+ |
| Node.js | 20.19+ or 22.12+ (for the frontend); under WSL it must be installed inside Linux (e.g. `nvm install 22`), not the Windows one |
| gcc | only for `go test -race` (cgo) |

## Run as one binary

The frontend is embedded in the Go binary, so one executable serves the whole tool:

```sh
cd caserunner
scripts/build.sh --run      # npm build + go build → backend/bin/caserunner, then (re)start it
backend/bin/caserunner      # later runs: start the built file directly
# then open http://127.0.0.1:8080
```

On Windows use `powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1` (add `-Run` to start it), which writes `backend\bin\caserunner.exe`; `GOOS=windows GOARCH=amd64 scripts/build.sh` cross-builds the same `.exe` from Linux/WSL. Full per-OS instructions: [root README](../README.md#quick-start).

In the background: `nohup backend/bin/caserunner > caserunner.log 2>&1 &` (stop it with `kill <pid>`). A binary built with plain `go build` (without the script) serves the API and explains on `/` that the frontend is not built.

## Develop

Backend (listens on `127.0.0.1:8080`):

```sh
cd caserunner/backend
go run ./cmd/caserunner
```

Frontend (dev server on `http://localhost:5173`, proxies `/api` to the backend):

```sh
cd caserunner/frontend
npm install
npm run dev
```

Open http://localhost:5173 — the page shows **BE OK** when the backend is reachable. To point the proxy elsewhere: `CASERUNNER_API_URL=http://127.0.0.1:9090 npm run dev`.

Fake API to run cases against (development only, BRD-001 §18):

```sh
cd caserunner/backend
go run ./cmd/mockapi   # 127.0.0.1:9999, or MOCKAPI_BIND
```

Endpoints (all `POST` unless noted): `/ok`, `/created`, `/error` (500), `/redirect` (302), `/validate` (400 when `data.amount` is missing, `null`, or `""`), `/echo` (returns the body), `/slow?ms=`, `/login` (new token each call), `/secure` (401 without a login token), `GET /hits`, `/hits/reset`.

## Checks

```sh
cd caserunner/backend && go vet ./... && go test ./...   # add -race when gcc is installed
cd caserunner/frontend && npm run lint && npm run build
```

## Configuration

Environment variables, all optional (BRD-001 §20):

| Variable | Default | Meaning |
|---|---|---|
| `CASERUNNER_BIND` | `127.0.0.1:8080` | Listen address |
| `CASERUNNER_ALLOWED_HOSTS` | (empty = any host) | Target host allowlist, comma-separated, supports `*` |
| `CASERUNNER_ALLOW_INSECURE` | `false` | Allow `-k` to skip TLS verification |
| `CASERUNNER_MAX_CONCURRENCY` | `20` | Concurrency ceiling |
| `CASERUNNER_DEFAULT_TIMEOUT_MS` | `30000` | Default per-request timeout |
| `CASERUNNER_MAX_TIMEOUT_MS` | `120000` | Timeout ceiling |
| `CASERUNNER_MAX_CASES` | `1000` | Maximum cases per run |
| `CASERUNNER_RUN_TTL` | `1h` | How long a finished run is kept in memory |
| `CASERUNNER_MAX_RUNS` | `50` | Maximum runs in memory |

An invalid value stops the service at startup with a message naming the variable.

## Layout

The backend follows Clean Architecture (BRD-001 §14.1): `adapter` → `usecase` → `domain`, wired in `cmd/caserunner`.

```
caserunner/
├── backend/
│   ├── cmd/caserunner/          # composition root
│   ├── cmd/mockapi/             # fake API for local runs (not in the release)
│   └── internal/
│       ├── config/            # env → Config
│       ├── domain/curl/       # curl tokenizer + parser (§6)
│       ├── domain/field/      # ordered JSON tree + field flattening and editing (§7)
│       ├── domain/flow/       # token steps: captures, {{variables}} (§11.3)
│       ├── domain/testcase/   # case validation + request building (§9, §10)
│       ├── domain/run/        # run results + expect classifier (§12)
│       ├── usecase/           # application rules: ParseCurl, prepare steps, runs
│       └── adapter/
│           ├── httpapi/       # HTTP API: routing, handlers, SSE, error JSON
│           ├── sender/        # outgoing HTTP client
│           ├── memstore/      # in-memory runs with TTL
│           ├── broker/        # run events to SSE subscribers
│           ├── system/        # clock, run IDs
│           └── web/           # embedded frontend build (filled by scripts/build.sh)
└── frontend/                  # Vite + React + TypeScript + Ant Design
```
