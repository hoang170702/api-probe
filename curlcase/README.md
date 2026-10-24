# CurlCase

Paste a `curl`, define test cases in a table, fire them in parallel against the API, and export the results. Design and requirements: [BRD-001](../docs/brd/BRD-001.md).

Status: **Step 0** of the implementation plan (BRD-001 §18) — project skeleton and health check.

## Requirements

| Tool | Version |
|---|---|
| Go | 1.27+ |
| Node.js | 20.19+ or 22.12+ (for the frontend) |
| gcc | only for `go test -race` (cgo) |

## Run locally

Backend (listens on `127.0.0.1:8080`):

```sh
cd curlcase/backend
go run ./cmd/curlcase
```

Frontend (dev server on `http://localhost:5173`, proxies `/api` to the backend):

```sh
cd curlcase/frontend
npm install
npm run dev
```

Open http://localhost:5173 — the page shows **BE OK** when the backend is reachable. To point the proxy elsewhere: `CURLCASE_API_URL=http://127.0.0.1:9090 npm run dev`.

## Checks

```sh
cd curlcase/backend && go vet ./... && go test ./...   # add -race when gcc is installed
cd curlcase/frontend && npm run lint && npm run build
```

## Configuration

Environment variables, all optional (BRD-001 §20):

| Variable | Default | Meaning |
|---|---|---|
| `CURLCASE_BIND` | `127.0.0.1:8080` | Listen address |
| `CURLCASE_ALLOWED_HOSTS` | (empty = any host) | Target host allowlist, comma-separated, supports `*` |
| `CURLCASE_ALLOW_INSECURE` | `false` | Allow `-k` to skip TLS verification |
| `CURLCASE_MAX_CONCURRENCY` | `20` | Concurrency ceiling |
| `CURLCASE_DEFAULT_TIMEOUT_MS` | `30000` | Default per-request timeout |
| `CURLCASE_MAX_TIMEOUT_MS` | `120000` | Timeout ceiling |
| `CURLCASE_MAX_CASES` | `1000` | Maximum cases per run |
| `CURLCASE_RUN_TTL` | `1h` | How long a finished run is kept in memory |
| `CURLCASE_MAX_RUNS` | `50` | Maximum runs in memory |

An invalid value stops the service at startup with a message naming the variable.

## Layout

The backend follows Clean Architecture (BRD-001 §14.1): `adapter` → `usecase` → `domain`, wired in `cmd/curlcase`.

```
curlcase/
├── backend/
│   ├── cmd/curlcase/          # composition root
│   └── internal/
│       ├── config/            # env → Config
│       ├── domain/            # business rules, no I/O (from Step 1)
│       ├── usecase/           # application rules + ports (from Step 3)
│       └── adapter/httpapi/   # HTTP API: routing, handlers, error JSON
└── frontend/                  # Vite + React + TypeScript + Ant Design
```
