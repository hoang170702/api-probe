<#
.SYNOPSIS
Builds API Case Runner into one program: backend\bin\caserunner.exe.

.DESCRIPTION
Windows counterpart of build.sh. Checks Go and Node.js, builds the frontend,
embeds it in the Go binary and compiles backend\bin\caserunner.exe.
Set $env:VERSION to stamp a version. $env:CASERUNNER_* settings set in the
same window apply to -Run.

.PARAMETER Run
Start the tool right after the build. Ctrl+C stops it. An API Case Runner
already listening on that port is stopped first; any other program there
stops the build instead.

.EXAMPLE
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run
Build, then start the tool at http://127.0.0.1:8080.

.EXAMPLE
$env:VERSION = '1.0.0'; powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
Build version 1.0.0 without starting it.
#>
[CmdletBinding()]
param([switch]$Run)
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root 'backend\internal\adapter\web\dist'
$exe = Join-Path $root 'backend\bin\caserunner.exe'

function Stop-Build([string]$Message, [string]$Fix) {
  Write-Host "error: $Message" -ForegroundColor Red
  Write-Host $Fix
  exit 1
}

$nodeFix = @"
Install Node.js 22 LTS, then open a new PowerShell window:
  winget install OpenJS.NodeJS.LTS
or download it from https://nodejs.org/
"@
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { Stop-Build 'node was not found on PATH' $nodeFix }
$nodeVersion = (& node -p 'process.versions.node').Trim()
$major, $minor = $nodeVersion.Split('.')[0..1] | ForEach-Object { [int]$_ }
if (-not ($major -gt 22 -or ($major -eq 22 -and $minor -ge 12) -or ($major -eq 20 -and $minor -ge 19))) {
  Stop-Build "Node.js v$nodeVersion is too old; Vite needs 20.19+ or 22.12+" $nodeFix
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Stop-Build 'go was not found on PATH' "Install Go 1.27+, then open a new PowerShell window:`n  winget install GoLang.Go`nor download it from https://go.dev/dl/"
}

# The port -Run will listen on (CASERUNNER_BIND, default 127.0.0.1:8080).
$bind = if ($env:CASERUNNER_BIND) { $env:CASERUNNER_BIND } else { '127.0.0.1:8080' }
$port = [int]$bind.Substring($bind.LastIndexOf(':') + 1)

# Find-Running returns the API Case Runner processes to stop: those listening
# on $port (with -CheckPort) and any running backend\bin\caserunner.exe, which
# Windows locks so the build could not replace it. Another program on $port
# stops the build: it is never killed.
function Find-Running([switch]$CheckPort) {
  $found = @{}
  if ($CheckPort) {
    foreach ($conn in @(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue)) {
      $proc = Get-Process -Id $conn.OwningProcess -ErrorAction SilentlyContinue
      if (-not $proc) { continue }
      if ($proc.ProcessName -ne 'caserunner') {
        Stop-Build "port $port is used by $($proc.ProcessName) (PID $($proc.Id)), not API Case Runner" "Stop that program, or choose another port, e.g.:`n  `$env:CASERUNNER_BIND = '127.0.0.1:9090'"
      }
      $found[$proc.Id] = $proc
    }
  }
  foreach ($proc in @(Get-Process caserunner -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe })) {
    $found[$proc.Id] = $proc
  }
  return @($found.Values)
}

# Fail before the build when the port is taken by something else.
if ($Run) { [void](Find-Running -CheckPort) }

# npm and go are native commands: a failure sets $LASTEXITCODE instead of throwing.
function Invoke-Native([string]$What, [scriptblock]$Command) {
  & $Command
  if ($LASTEXITCODE -ne 0) { Stop-Build "$What failed (exit code $LASTEXITCODE)" 'See the output above.' }
}

Write-Host '==> frontend'
Push-Location (Join-Path $root 'frontend')
try {
  if (-not (Test-Path node_modules)) { Invoke-Native 'npm ci' { npm ci } }
  Invoke-Native 'npm run build' { npm run build }
} finally {
  Pop-Location
}

Write-Host '==> embed'
Get-ChildItem $web -Force | Where-Object { $_.Name -ne '.gitkeep' } | Remove-Item -Recurse -Force
Copy-Item (Join-Path $root 'frontend\dist\*') $web -Recurse -Force

$running = @(Find-Running -CheckPort:$Run)
if ($running.Count -gt 0) {
  $ids = ($running | ForEach-Object { $_.Id }) -join ', '
  if (-not $Run) {
    Stop-Build "API Case Runner is running (PID $ids), so the build cannot replace caserunner.exe" "Stop it first (Ctrl+C in its window, or Stop-Process -Id $ids), or build with -Run to restart it."
  }
  foreach ($proc in $running) {
    Write-Host "==> stopping the running API Case Runner (PID $($proc.Id))"
    Stop-Process -Id $proc.Id -Force
    [void]$proc.WaitForExit(10000)
  }
}

Write-Host '==> backend'
$version = if ($env:VERSION) { $env:VERSION } else { 'dev' }
Push-Location (Join-Path $root 'backend')
try {
  Invoke-Native 'go build' { go build -ldflags "-X main.version=$version" -o bin\caserunner.exe .\cmd\caserunner }
} finally {
  Pop-Location
}

Write-Host "Built $exe"
if ($Run) {
  Write-Host '==> run (Ctrl+C to stop)'
  & $exe
  exit $LASTEXITCODE
}
Write-Host 'Run it and open http://127.0.0.1:8080, or build and run in one go with: powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1 -Run'
