# Agent-Unleashed (agt-ul) Windows Installer
$ErrorActionPreference = "Stop"

Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  🚀 Installing Agent-Unleashed (agt-ul) for Windows" -ForegroundColor Green
Write-Host "============================================================" -ForegroundColor Cyan

$InstallDir = "$env:LOCALAPPDATA\agy\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$ExePath = "$InstallDir\agt-ul.exe"
$AliasPath = "$InstallDir\agy-ul.exe"

# If repository is cloned locally, build from source
if (Test-Path ".\main.go") {
    Write-Host "🔨 Compiling from local source..." -ForegroundColor Yellow
    go build -o $ExePath .
    Copy-Item -Force $ExePath $AliasPath
} else {
    Write-Host "⬇️ Downloading latest agt-ul binary..." -ForegroundColor Yellow
    # When deployed, download binary directly
    # Invoke-WebRequest -Uri "https://agent.subimpact.net/downloads/agt-ul.exe" -OutFile $ExePath
}

# Ensure PATH contains $InstallDir
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "✅ Added $InstallDir to user PATH" -ForegroundColor Green
}

Write-Host "✅ Successfully installed agt-ul to $ExePath!" -ForegroundColor Green
Write-Host "Run 'agt-ul setup' or 'agt-ul' to get started." -ForegroundColor Cyan
