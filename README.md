# APIProbe

**API Case Runner** is a tool for testers: paste one API request as a `curl` command, list many test cases for it in a table, fire them against the API and get a result per case — without writing code.

![API Case Runner: configuration on the left, test cases and live results on the right](docs/images/09-overview.png)

## Features

| Feature | Status |
|---|---|
| Paste a curl from Chrome / Firefox DevTools ("Copy as cURL", bash or cmd) or Postman; see method, URL, headers (sensitive values masked) and warnings | Available |
| Every field of the JSON body listed in order, with its type, sample value and a Required / Unrequired rule | Available |
| Optional **token steps**: run a login curl first, capture the token automatically, put `Authorization: Bearer <token>` on the main request; `{{name}}` variables for multi-step flows; **Try steps** to check it | Available |
| Host allowlist and `-k` policy for every outgoing request | Available |
| User interface in Vietnamese and English | Available |
| Case table (`+` to add a case), each case judged by an **expect** code: the exact HTTP status, or text in the response body or an application header | Available |
| Running cases (parallel, or one at a time with a fresh token each), real-time results, case details, cancel and run again | Available |
| Export / import the whole configuration (token steps, curl, rules, cases, options) as a JSON file | Available |
| Excel and PDF reports | Planned |

Progress by step: [BRD-001 §18](docs/brd/BRD-001.md#18-implementation-plan-small-steps).

## Quick start

Works on **Windows**, **Linux**, **macOS** and **WSL**. The result is one program that serves both the web page and its API.

### 1. Requirements

| Tool | Version | Needed for |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.27+ | building the program |
| [Node.js](https://nodejs.org/) | 20.19+ or 22.12+ | building the web interface |
| Git | any | cloning the repository |
| gcc | any | only for `go test -race` (developers) |

**Windows** (PowerShell):

```powershell
winget install GoLang.Go
winget install OpenJS.NodeJS.LTS
winget install Git.Git
# then open a new PowerShell window so PATH is refreshed
```

**WSL / Linux**: Node.js must be installed inside Linux, not the Windows one (`npm` from `/mnt/c/...` cannot build here). For example with [nvm](https://github.com/nvm-sh/nvm):

```sh
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
# open a new terminal, then
nvm install 22
```

> Use one clone per operating system: `node_modules` installed on Windows does not work in WSL and the other way round. If you switch, delete `caserunner/frontend/node_modules` first.

### 2. Build and run in one command

Clone the repository, then run the build script with the **run** option. It builds everything and starts the tool in the same window.

On Windows the script is started through `powershell -ExecutionPolicy Bypass -File …`: Windows blocks local scripts by default (*running scripts is disabled on this system*), and this form allows this one run without changing any system setting.

**Windows** (PowerShell):

```powershell
git clone https://github.com/hoang170702/api-probe.git
cd api-probe\caserunner
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run
```

**Linux / macOS / WSL**:

```sh
git clone https://github.com/hoang170702/api-probe.git
cd api-probe/caserunner
scripts/build.sh --run
```

When the window shows `caserunner listening url=http://127.0.0.1:8080`, open **http://127.0.0.1:8080** in a browser. The tag at the top right shows **BE OK** when the tool is ready. A tool running in WSL is also reachable from the Windows browser at the same address. Press **Ctrl+C** in that window to stop it.

The first build takes a minute or two (it downloads the frontend packages once); later builds take a few seconds.

### What the script does

| Step | Output line | What happens |
|---|---|---|
| Check tools | `error: …` only on a problem | Go and Node.js must be installed and recent enough; otherwise it stops and prints the install command (`winget …` on Windows, `nvm …` on Linux/WSL) |
| Frontend | `==> frontend` | Installs the frontend packages the first time (`npm ci`), then builds the web interface (`npm run build`) |
| Embed | `==> embed` | Copies that build into the backend so both ship as one program |
| Backend | `==> backend` | Compiles **one file**: `backend\bin\caserunner.exe` on Windows, `backend/bin/caserunner` elsewhere |
| Restart (only with `-Run` / `--run`) | `==> stopping the running API Case Runner (PID …)` | If an API Case Runner is already listening on the port (`CASERUNNER_BIND`, default 8080), it is stopped so the new build can take its place. On Windows this happens just before `==> backend`, because Windows does not let a running `.exe` be replaced |
| Run (only with `-Run` / `--run`) | `==> run (Ctrl+C to stop)` | Starts that file; one process serves both the web interface and the API |

There is no separate frontend or backend server to start: the one file is the whole tool.

**Running the build again restarts the tool**: with `-Run` / `--run` there is no need to stop the old one first. Only API Case Runner itself is stopped (a process named `caserunner`); if another program holds the port, the script stops *before* building, names that program and its PID, and leaves it alone:

```text
error: port 8080 is used by python3 (PID 4242), not API Case Runner
Stop that program, or choose another port, e.g. CASERUNNER_BIND=127.0.0.1:9090 scripts/build.sh --run
```

Stopping the old instance ends any run in progress; its results are kept in memory only, so export them first. On Windows, building **without** `-Run` while `caserunner.exe` is running stops with *"API Case Runner is running (PID …)"*: stop it, or build with `-Run`.

### Build once, run many times

Building is needed only the first time and after pulling new code. Afterwards, start the built file directly; it is faster and needs neither Go nor Node.js:

| | Windows (PowerShell) | Linux / macOS / WSL |
|---|---|---|
| Build only | `powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1` | `scripts/build.sh` |
| Build and run | `powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run` | `scripts/build.sh --run` |
| Run an existing build | `.\backend\bin\caserunner.exe` (or double-click it) | `backend/bin/caserunner` |
| Show the script's options | `powershell -ExecutionPolicy Bypass -Command "Get-Help .\scripts\build.ps1 -Detailed"` | `scripts/build.sh --help` |
| Stamp a version | `$env:VERSION = '1.0.0'` before building | `VERSION=1.0.0 scripts/build.sh` |

**A Windows `.exe` for testers without Go or Node.js**: build it on Linux/WSL with `GOOS=windows GOARCH=amd64 scripts/build.sh`, then hand over `backend/bin/caserunner.exe`; it runs on 64-bit Windows 10 or later with nothing else installed. (`--run` cannot be combined with a build for another system.)

### 3. Run options

Settings are environment variables set before the command; they apply to `-Run` / `--run` too. All of them are listed in [Configuration](#configuration).

| To | Windows (PowerShell) | Linux / macOS / WSL |
|---|---|---|
| Stop it | `Ctrl+C` in its window, or close the window | `Ctrl+C` |
| Keep it running in the background | `Start-Process -WindowStyle Hidden .\backend\bin\caserunner.exe` | `nohup backend/bin/caserunner > caserunner.log 2>&1 &` |
| Stop a background run | `Stop-Process -Name caserunner` | `kill <pid>` |
| Use another port / let teammates open it | `$env:CASERUNNER_BIND = '0.0.0.0:9090'; .\backend\bin\caserunner.exe` | `CASERUNNER_BIND=0.0.0.0:9090 backend/bin/caserunner` |
| Allow only test environments | `$env:CASERUNNER_ALLOWED_HOSTS = 'localhost,*.uat.example.com'; .\backend\bin\caserunner.exe` | `CASERUNNER_ALLOWED_HOSTS='localhost,*.uat.example.com' backend/bin/caserunner` |
| Same, while building | `$env:CASERUNNER_BIND = '127.0.0.1:9090'; powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run` | `CASERUNNER_BIND=127.0.0.1:9090 scripts/build.sh --run` |

With `CASERUNNER_BIND=0.0.0.0:…` Windows may ask to allow the program through **Windows Defender Firewall**; allow it only when teammates should reach the tool. The default `127.0.0.1` address needs no firewall access.

## How to use

The [user guide](docs/user-guide.md) ([Tiếng Việt](docs/user-guide.vi.md)) walks through each screen with screenshots: copying a request as curl, reading the request summary, setting field rules, getting a token first, writing and running test cases, reading the results, and what each error message means.

In short:

1. *(Optional)* **0. Token** — add the login curl, click **Analyze**, then **Try steps**; the token is found automatically.
2. **1. Curl** — paste the request to test and click **Analyze** (or press Ctrl+Enter).
3. **2. Request** — check the method, URL, headers and warnings.
4. **3. Field rules** — mark each body field **Required** (every case must fill it) or **Unrequired** (a case may leave it out of the request).
5. **4. Test cases** — one row per case (**+ Add case**), each with an **Expect** code such as `"code":"00"`; set the parallel requests and timeout, then click **Run**.
6. **5. Results** — SUCCESS / FAILED / EXCEPTION / BLOCKED / CANCELLED arrive live; click a row for the request and response.

## Configuration

All settings are optional environment variables, read at startup. An invalid value stops the service with a message naming the variable. Set them with `$env:NAME = 'value'` in PowerShell (for that window) or `NAME=value` before the command in a Unix shell.

| Variable | Default | Meaning |
|---|---|---|
| `CASERUNNER_BIND` | `127.0.0.1:8080` | Listen address; the default is reachable from this machine only |
| `CASERUNNER_ALLOWED_HOSTS` | *(empty = any host)* | Hosts the tool may call, comma-separated; `*` matches any characters, e.g. `*.uat.example.com,10.10.*` |
| `CASERUNNER_ALLOW_INSECURE` | `false` | Allow curls with `-k` / `--insecure` to skip TLS verification |
| `CASERUNNER_DEFAULT_TIMEOUT_MS` | `30000` | Per-request timeout when none is chosen |
| `CASERUNNER_MAX_TIMEOUT_MS` | `120000` | Upper limit for any timeout |
| `CASERUNNER_MAX_CONCURRENCY` | `20` | Upper limit for parallel requests in a run |
| `CASERUNNER_MAX_CASES` | `1000` | Maximum cases per run |
| `CASERUNNER_RUN_TTL` | `1h` | How long a finished run is kept in memory |
| `CASERUNNER_MAX_RUNS` | `50` | Maximum runs kept in memory |

Nothing is stored on disk and there is no database; the tool logs method, path, status and duration only, never headers or bodies.

> **Safety:** the tool sends real requests to the URLs you paste. Use it against LOCAL, DEV or UAT environments, and set `CASERUNNER_ALLOWED_HOSTS` when others share the instance.

## Development

Run the backend and the Vite dev server side by side; the dev server proxies `/api` to the backend.

```sh
# terminal 1 — backend on 127.0.0.1:8080
cd caserunner/backend
go run ./cmd/caserunner

# terminal 2 — web interface on http://localhost:5173
cd caserunner/frontend
npm install
npm run dev
```

The commands are the same in PowerShell (with `\` in paths). A fake API for trying runs locally is in `cmd/mockapi` (development only, not part of the build): `go run ./cmd/mockapi` from `caserunner/backend` listens on `127.0.0.1:9999` (`MOCKAPI_BIND` to change) with endpoints such as `POST /ok`, `/validate` (400 when `data.amount` is missing, `null`, or `""`), `/slow?ms=`, `/login` and `/secure` (needs the login token).

To point the dev server at another backend: `CASERUNNER_API_URL=http://127.0.0.1:9090 npm run dev`, or `$env:CASERUNNER_API_URL = 'http://127.0.0.1:9090'; npm run dev` in PowerShell.

Checks:

```sh
cd caserunner/backend  && go vet ./... && CGO_ENABLED=1 go test -race ./...
cd caserunner/frontend && npm run lint && npm run build
```

`-race` needs gcc (cgo); without it run `go test ./...`.

### Repository layout

```
.
├── caserunner/                 # API Case Runner
│   ├── backend/                # Go service (Clean Architecture: adapter → usecase → domain)
│   │   ├── cmd/caserunner/     # entry point
│   │   ├── cmd/mockapi/        # fake API for local runs (development only)
│   │   └── internal/
│   │       ├── config/         # environment variables
│   │       ├── domain/         # curl parser, JSON fields, token flow, cases, run results — pure rules, no I/O
│   │       ├── usecase/        # application rules, host policy, runs, ports
│   │       └── adapter/        # HTTP API + SSE, outgoing sender, run store, event broker, embedded web build
│   ├── frontend/               # React 19 + TypeScript + Vite + Ant Design 6
│   └── scripts/                # one-binary build: build.sh (Linux/macOS/WSL), build.ps1 (Windows)
└── docs/
    ├── brd/                    # BRD-001: requirements and design (English + Vietnamese)
    ├── user-guide.md           # tester guide (+ user-guide.vi.md)
    └── images/                 # screenshots
```

More detail for developers: [caserunner/README.md](caserunner/README.md).

### Contributing

- Work on a branch from `dev` named `feature/…`, `fix/…`, `docs/…`, …; open pull requests into `dev`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/), e.g. `feat(caserunner): add case table`.
- Behavior changes start in [BRD-001](docs/brd/BRD-001.md): update the English version and its Vietnamese translation together.

## Documentation

| Document | For |
|---|---|
| [User guide](docs/user-guide.md) · [Tiếng Việt](docs/user-guide.vi.md) | Testers using the tool |
| [BRD-001](docs/brd/BRD-001.md) · [Tiếng Việt](docs/brd/BRD-001.vi.md) | Requirements, design decisions, API, plan |
| [caserunner/README.md](caserunner/README.md) | Developers working on the service |
