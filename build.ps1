<#
.SYNOPSIS
    Master build script for MonsterMQ Tools (cli, i3x, gql, mbp) on Windows / PowerShell.

.DESCRIPTION
    Builds native or cross-compiled binaries for MonsterMQ Tools components.
    Converted from build.sh for Windows PowerShell and PowerShell Core environments.

.PARAMETER All
    Cross-compile all tools for linux, darwin, and windows.

.PARAMETER Native
    Build native binaries for host platform (default).

.PARAMETER Cli
    Build mmq (MonsterMQ CLI) only.

.PARAMETER I3x
    Build i3x CLI only.

.PARAMETER Gql
    Build compare-schemas tool only.

.PARAMETER Mbp
    Build mbp pipeline tool only.

.PARAMETER Clean
    Clean all bin/ output directories.

.PARAMETER Help
    Show this help message.

.EXAMPLE
    .\build.ps1
    Build native binaries for all tools.

.EXAMPLE
    .\build.ps1 -Cli
    Build mmq CLI only.

.EXAMPLE
    .\build.ps1 -All
    Cross-compile all tools for all platforms.

.EXAMPLE
    .\build.ps1 -Clean
    Clean all output directories.
#>

[CmdletBinding()]
param(
    [switch]$All,
    [switch]$Native,
    [switch]$Cli,
    [switch]$I3x,
    [switch]$Gql,
    [switch]$Mbp,
    [switch]$Clean,
    [Alias("h", "?")]
    [switch]$Help
)

$ErrorActionPreference = "Stop"

Set-Location -Path $PSScriptRoot

function Show-Usage {
    Write-Host "Usage: .\build.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -All, --all               Cross-compile all tools for linux, darwin, and windows"
    Write-Host "  -Native, --native         Build native binaries for host platform (default)"
    Write-Host "  -Cli, --cli               Build mmq (MonsterMQ CLI) only"
    Write-Host "  -I3x, --i3x               Build i3x CLI only"
    Write-Host "  -Gql, --gql               Build compare-schemas tool only"
    Write-Host "  -Mbp, --mbp               Build mbp pipeline tool only"
    Write-Host "  -Clean, --clean           Clean all bin/ output directories"
    Write-Host "  -Help, -h, --help, -?     Show this help message"
    Write-Host ""
}

if ($Help) {
    Show-Usage
    exit 0
}

if ($Clean) {
    Write-Host "Cleaning all tools bin/ directories..." -ForegroundColor Yellow
    $dirs = @("cli/bin", "i3x/bin", "gql/bin", "mbp/bin")
    foreach ($d in $dirs) {
        if (Test-Path $d) {
            Remove-Item -Recurse -Force $d
        }
    }
    Write-Host "[OK] All tools cleaned successfully." -ForegroundColor Green
    exit 0
}

$explicitSubtool = $Cli -or $I3x -or $Gql -or $Mbp

$buildCli = if ($explicitSubtool) { $Cli.IsPresent } else { $true }
$buildI3x = if ($explicitSubtool) { $I3x.IsPresent } else { $true }
$buildGql = if ($explicitSubtool) { $Gql.IsPresent } else { $true }
$buildMbp = if ($explicitSubtool) { $Mbp.IsPresent } else { $true }

$targetParams = @{}
if ($All) {
    $targetParams["All"] = $true
    $modeLabel = "--all"
} else {
    $targetParams["Native"] = $true
    $modeLabel = "--native"
}

Write-Host "======================================================" -ForegroundColor Cyan
Write-Host "  Building MonsterMQ Tools ($modeLabel)" -ForegroundColor Cyan
Write-Host "======================================================" -ForegroundColor Cyan

if ($buildCli) {
    Write-Host "`n>>> [MonsterMQ CLI] Building cli..." -ForegroundColor Yellow
    & "$PSScriptRoot\cli\build.ps1" @targetParams
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($buildI3x) {
    Write-Host "`n>>> [i3X CLI] Building i3x..." -ForegroundColor Yellow
    & "$PSScriptRoot\i3x\build.ps1" @targetParams
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($buildGql) {
    Write-Host "`n>>> [GraphQL Tools] Building gql..." -ForegroundColor Yellow
    & "$PSScriptRoot\gql\build.ps1" @targetParams
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

if ($buildMbp) {
    Write-Host "`n>>> [Build Pipeline] Building mbp..." -ForegroundColor Yellow
    & "$PSScriptRoot\mbp\build.ps1" @targetParams
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Green
Write-Host "  All selected MonsterMQ Tools built successfully!    " -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Green
