# API Case Runner

Paste a `curl`, define test cases in a table, fire them in parallel against the API, and export the results.

- How to use it: [user guide](../docs/user-guide.md) ([Tiếng Việt](../docs/user-guide.vi.md))
- Design and requirements: [BRD-001](../docs/brd/BRD-001.md) ([Tiếng Việt](../docs/brd/BRD-001.vi.md))

Status: Steps 0–3 and 3b of the implementation plan (BRD-001 §18) — optional token steps (with Try), paste a curl, see the request and every field of its JSON body with a Required/Unrequired rule; UI in Vietnamese and English. Cases and runs come next.

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
scripts/build.sh            # npm build + go build → backend/bin/caserunner
backend/bin/caserunner        # then open http://127.0.0.1:8080
```

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
│   └── internal/
│       ├── config/            # env → Config
│       ├── domain/curl/       # curl tokenizer + parser (§6)
│       ├── domain/field/      # ordered JSON tree + field flattening (§7)
│       ├── usecase/           # application rules: ParseCurl
│       └── adapter/
│           ├── httpapi/       # HTTP API: routing, handlers, error JSON
│           └── web/           # embedded frontend build (filled by scripts/build.sh)
└── frontend/                  # Vite + React + TypeScript + Ant Design
```
