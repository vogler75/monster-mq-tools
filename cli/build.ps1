<#
.SYNOPSIS
    Build script for mmq (MonsterMQ CLI) on Windows / PowerShell.

.DESCRIPTION
    Builds the native mmq binary or cross-compiles for all supported platforms.
    Converted from build.sh for Windows PowerShell and PowerShell Core environments.

.PARAMETER Native
    Build native binary for current platform (default).

.PARAMETER All
    Cross-compile binaries for linux, darwin, and windows.

.PARAMETER Clean
    Clean bin/ output directory.

.PARAMETER Help
    Show usage help message.

.EXAMPLE
    .\build.ps1
    Builds native binary for host OS.

.EXAMPLE
    .\build.ps1 -All
    Cross-compiles for all platforms.

.EXAMPLE
    .\build.ps1 -Clean
    Cleans the bin directory.
#>

[CmdletBinding()]
param(
    [switch]$Native,
    [switch]$All,
    [switch]$Clean,
    [Alias("h", "?")]
    [switch]$Help
)

$ErrorActionPreference = "Stop"

# Navigate to the script directory
Set-Location -Path $PSScriptRoot

function Show-Usage {
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Native, --native         Build native binary for current platform (default)"
    Write-Host "  -All, --all               Cross-compile binaries for linux, darwin, and windows"
    Write-Host "  -Clean, --clean           Clean bin/ output directory"
    Write-Host "  -Help, -h, --help, -?     Show this help message"
    Write-Host ""
}

if ($Help) {
    Show-Usage
    exit 0
}

# Resolve version
$Version = "0.1.0"
if (Test-Path "../version.txt") {
    $Version = (Get-Content "../version.txt" -TotalCount 1).Trim()
} elseif (Test-Path "version.txt") {
    $Version = (Get-Content "version.txt" -TotalCount 1).Trim()
}

$ldflags = "-s -w -X main.Version=$Version"
$goflags = "-trimpath"

# Determine build action
$buildNative = $true
$buildAll = $false

if ($Clean) {
    Write-Host "Cleaning bin/ directory..." -ForegroundColor Yellow
    if (Test-Path "bin") {
        Remove-Item -Recurse -Force "bin"
    }
    Write-Host "[OK] Cleaned" -ForegroundColor Green
    exit 0
}

if ($All) {
    $buildNative = $true
    $buildAll = $true
} elseif ($Native) {
    $buildNative = $true
    $buildAll = $false
}

# Ensure bin directory exists
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

# Detect binary extension for current platform
$binaryExt = ""
if ($IsWindows -or ($env:OS -like "*Windows*")) {
    $binaryExt = ".exe"
}
$nativeOut = "bin/mmq$binaryExt"

if ($buildNative) {
    Write-Host "Building native mmq binary (version $Version)..." -ForegroundColor Green
    $env:CGO_ENABLED = "0"
    & go build $goflags "-ldflags=$ldflags" -o $nativeOut .
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Native build failed with exit code $LASTEXITCODE"
        exit $LASTEXITCODE
    }
    Write-Host "[OK] Native binary built at: " -ForegroundColor Green -NoNewline
    Write-Host "cli/$nativeOut" -ForegroundColor Yellow
}

if ($buildAll) {
    Write-Host "Cross-compiling mmq for all targets (version $Version)..." -ForegroundColor Green

    $env:CGO_ENABLED = "0"

    # Linux AMD64
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    Remove-Item env:GOARM -ErrorAction SilentlyContinue
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-linux-amd64" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # Linux ARM64
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    Remove-Item env:GOARM -ErrorAction SilentlyContinue
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-linux-arm64" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # Linux ARMv7
    $env:GOOS = "linux"
    $env:GOARCH = "arm"
    $env:GOARM = "7"
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-linux-armv7" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # macOS AMD64 (Darwin)
    Remove-Item env:GOARM -ErrorAction SilentlyContinue
    $env:GOOS = "darwin"
    $env:GOARCH = "amd64"
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-darwin-amd64" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # macOS ARM64 (Darwin / Apple Silicon)
    $env:GOOS = "darwin"
    $env:GOARCH = "arm64"
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-darwin-arm64" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # Windows AMD64
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    & go build $goflags "-ldflags=$ldflags" -o "bin/mmq-windows-amd64.exe" .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    # Cleanup build environment variables
    Remove-Item env:GOOS -ErrorAction SilentlyContinue
    Remove-Item env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item env:GOARM -ErrorAction SilentlyContinue
    Remove-Item env:CGO_ENABLED -ErrorAction SilentlyContinue

    Write-Host "[OK] All binaries built in: " -ForegroundColor Green -NoNewline
    Write-Host "cli/bin/" -ForegroundColor Yellow
    Get-ChildItem -Path "bin" | Format-Table Name, Length, LastWriteTime
}
