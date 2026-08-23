# Agent-Unleashed (agt-ul) Windows installer
#
# Usage:  irm https://agent.subimpact.net/install.ps1 | iex
#
# Downloads the published release binary, verifies its SHA-256 against the
# release checksum, and installs it on PATH. Falls back to building from source
# only when run inside a checkout. It never reports success without installing.

$ErrorActionPreference = "Stop"

$Repo       = "subimpact/agent-unleashed"
$InstallDir = Join-Path $env:LOCALAPPDATA "agt-ul\bin"
$ExePath    = Join-Path $InstallDir "agt-ul.exe"
$AliasPath  = Join-Path $InstallDir "agy-ul.exe"

function Write-Step($msg) { Write-Host $msg -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "  OK  $msg" -ForegroundColor Green }
function Fail($msg) {
    Write-Host "  FAILED  $msg" -ForegroundColor Red
    exit 1
}

Write-Host "============================================================"
Write-Host "  Installing Agent-Unleashed (agt-ul) for Windows"
Write-Host "============================================================"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Refuse to overwrite a running binary - the copy silently fails on Windows.
$running = Get-Process -Name "agt-ul" -ErrorAction SilentlyContinue
if ($running) {
    Fail "agt-ul is currently running (PID $($running.Id -join ', ')). Stop it and re-run this installer."
}

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "agt-ul_windows_$arch.exe"

if (Test-Path ".\main.go") {
    Write-Step "`n[1/3] Local checkout detected - building from source..."
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Fail "Go toolchain not found. Install Go from https://go.dev/dl/ or run this script outside a checkout to download a release binary."
    }
    & go build -trimpath -ldflags "-s -w" -o $ExePath .
    if ($LASTEXITCODE -ne 0) { Fail "go build exited with $LASTEXITCODE" }
    Write-Ok "Compiled $ExePath"
} else {
    Write-Step "`n[1/3] Downloading $asset ..."
    $base = "https://github.com/$Repo/releases/latest/download"
    $tmp  = Join-Path ([System.IO.Path]::GetTempPath()) "agt-ul-$([guid]::NewGuid()).exe"

    try {
        Invoke-WebRequest -Uri "$base/$asset" -OutFile $tmp -UseBasicParsing
    } catch {
        Fail "Could not download $base/$asset - $($_.Exception.Message)"
    }
    if (-not (Test-Path $tmp) -or (Get-Item $tmp).Length -eq 0) {
        Fail "Downloaded file is empty."
    }

    Write-Step "[2/3] Verifying checksum..."
    try {
        $sums = (Invoke-WebRequest -Uri "$base/checksums.txt" -UseBasicParsing).Content
    } catch {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "Could not fetch checksums.txt - refusing to install an unverified binary."
    }

    $expected = ($sums -split "`n" | Where-Object { $_ -match [regex]::Escape($asset) + '\s*$' } |
                 Select-Object -First 1) -split '\s+' | Select-Object -First 1
    if (-not $expected) {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "No checksum entry for $asset - refusing to install an unverified binary."
    }

    $actual = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLower()
    if ($actual -ne $expected.ToLower()) {
        Remove-Item $tmp -Force -ErrorAction SilentlyContinue
        Fail "Checksum mismatch. Expected $expected, got $actual."
    }
    Write-Ok "SHA-256 verified"

    Move-Item -Force $tmp $ExePath
    Write-Ok "Installed $ExePath"
}

Copy-Item -Force $ExePath $AliasPath

# PATH, added once. -notlike on a bare string matches substrings, so compare
# the parsed entries instead to stay idempotent.
Write-Step "[3/3] Ensuring $InstallDir is on PATH..."
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$entries  = @($userPath -split ';' | Where-Object { $_ })
if ($entries -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", (($entries + $InstallDir) -join ';'), "User")
    Write-Ok "Added to user PATH (restart your terminal to pick it up)"
} else {
    Write-Ok "Already on PATH"
}
$env:Path = "$env:Path;$InstallDir"

if (-not (Test-Path $ExePath)) { Fail "Installation did not produce $ExePath" }

Write-Host ""
Write-Host "Installed: $(& $ExePath version)" -ForegroundColor Green
Write-Host "Run 'agt-ul setup' to configure, or 'agt-ul' to start." -ForegroundColor Cyan
