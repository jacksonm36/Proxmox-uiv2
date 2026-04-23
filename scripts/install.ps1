# One-shot install on Windows: tools check, optional Postgres, .env, Go + web builds.
# Run from repo root:  powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
# Optional: -SkipDb

param(
    [switch]$SkipDb
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $Root

function Need-Cmd([string]$Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "install: required command not found: $Name"
    }
}

Write-Host "==> cloudmanager — install in $Root"

Need-Cmd go
Need-Cmd npm
Need-Cmd node

$goVer = (& go env GOVERSION 2>$null) ; Write-Host "==> using $goVer"

# Secrets
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$sb = New-Object byte[] 36
$rng.GetBytes($sb)
$SessionSecret = [Convert]::ToBase64String($sb)
$eb = New-Object byte[] 32
$rng.GetBytes($eb)
$EncHex = ($eb | ForEach-Object { $_.ToString("x2") }) -join ""

$Data = Join-Path $Root "data"
$Work = Join-Path $Data "work"
$WebDist = Join-Path $Root "apps\web\dist"
$Out = Join-Path $Root "dist"
New-Item -ItemType Directory -Force -Path $Work, $Out, $Data | Out-Null

$EnvEx = Join-Path $Root ".env.example"
$EnvFile = Join-Path $Root ".env"
if (Test-Path $EnvFile) {
    $Bak = "$EnvFile.bak.$(Get-Date -Format 'yyyyMMddHHmmss')"
    Write-Host "==> backup .env to $Bak"
    Copy-Item -Force $EnvFile $Bak
}
$raw = [System.IO.File]::ReadAllText($EnvEx)
$raw = $raw -replace '(?m)^CM_SESSION_SECRET=.*', "CM_SESSION_SECRET=$SessionSecret"
$raw = $raw -replace '(?m)^CM_ENCRYPTION_KEY=.*', "CM_ENCRYPTION_KEY=$EncHex"
$raw = $raw -replace '(?m)^CM_WORKDIR=.*', "CM_WORKDIR=$($Work -replace '\\','/')"
$raw = $raw -replace '(?m)^CM_WEB_ROOT=.*', "CM_WEB_ROOT=$($WebDist -replace '\\','/')"
[System.IO.File]::WriteAllText($EnvFile, $raw)

if (-not $SkipDb) {
    $sql = Join-Path $Root "scripts\init-db.sql"
    Write-Host "==> PostgreSQL (once, as superuser):"
    Write-Host "    psql -U postgres -h 127.0.0.1 -f `"$sql`""
    Write-Host "    createdb -U postgres -h 127.0.0.1 -O cloudmanager cloudmanager"
    Write-Host "    (or run the SQL in pgAdmin and create database cloudmanager owned by cloudmanager)"
} else {
    Write-Host "==> skipped -SkipDb"
}

Write-Host "==> go mod tidy + build"
Push-Location $Root
& go mod tidy
& go build -trimpath -ldflags "-s -w" -o (Join-Path $Out "cloudmanager-api.exe") .\cmd\api
& go build -trimpath -ldflags "-s -w" -o (Join-Path $Out "cloudmanager-worker.exe") .\cmd\worker
Pop-Location

Write-Host "==> web: npm install + build"
Push-Location (Join-Path $Root "apps\web")
& npm install
& npm run build
Pop-Location

Write-Host @"

----------------------------------------------------------------
Install complete.

  $Out\cloudmanager-api.exe
  $Out\cloudmanager-worker.exe
  Web: $WebDist
  .env: $EnvFile
  work: $Work

  Run in separate terminals (after loading env — use set from .env or a tool):
    `$json = Get-Content .env -Raw; ...  # or copy vars manually

  Example:
    go run ./cmd/api   (or .\dist\cloudmanager-api.exe)
    go run ./cmd/worker
    cd apps\web; npm run dev
----------------------------------------------------------------
"@
