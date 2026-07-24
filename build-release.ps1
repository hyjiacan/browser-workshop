#requires -Version 5.1
<#
.SYNOPSIS
    Cross-compile bws and package into platform/architecture zip files.

.DESCRIPTION
    Compiles bws binaries for specified platform/architecture combinations
    and packages them into zip files.

    Zip file names include version, platform and architecture identifiers,
    while the binary file name itself does not.

    Output example:
      dist/bws_1.0.0_windows_amd64.zip  (contains bws.exe)
      dist/bws_1.0.0_linux_amd64.zip    (contains bws)
      dist/bws_1.0.0_darwin_amd64.zip   (contains bws)

.PARAMETER OutputDir
    Output directory, defaults to dist/ under the project root.

.PARAMETER Targets
    List of target platform/architecture combinations in "GOOS/GOARCH" format.
    Defaults to: windows/amd64, linux/amd64, darwin/amd64

.PARAMETER SkipBuild
    Skip compilation and only package (for already compiled binaries).

.PARAMETER Version
    Version number injected into the binary. Defaults to git tag, or "dev" if no tag exists.

.EXAMPLE
    .\build-release.ps1
    Compile default targets and package to dist/.

.EXAMPLE
    .\build-release.ps1 -Targets @("windows/amd64", "linux/amd64")
    Only compile Windows and Linux amd64 versions.

.EXAMPLE
    .\build-release.ps1 -Version 0.5.0
    Compile with the specified version number.
#>
[CmdletBinding()]
param(
    [string]$OutputDir = "",

    [string[]]$Targets = @(
        "windows/amd64",
        "linux/amd64",
        "darwin/amd64"
    ),

    [switch]$SkipBuild,

    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

# Script is in the project root directory
$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Definition

if (-not $OutputDir) {
    $OutputDir = Join-Path $ProjectRoot "dist"
}

# Create output directory
New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
$OutputDir = Resolve-Path $OutputDir

Write-Host "========================================"
Write-Host "  Browser Workshop Release Builder"
Write-Host "========================================"
Write-Host "  Project root: $ProjectRoot"
Write-Host "  Output dir:   $OutputDir"
Write-Host ""

# Get version number
if (-not $Version) {
    try {
        $Version = git -C $ProjectRoot describe --tags --always 2>$null
        if (-not $Version) {
            $Version = "dev"
        }
    } catch {
        $Version = "dev"
    }
}
Write-Host "  Version:      $Version"
Write-Host ""

# Build parameters
$BuildTime = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
$LdFlags = "-s -w -X github.com/bws/bws/internal/version.Version=$Version -X github.com/bws/bws/internal/version.BuildTime=$BuildTime"

$SuccessCount = 0
$FailCount = 0

foreach ($Target in $Targets) {
    $Parts = $Target -split "/"
    if ($Parts.Length -ne 2) {
        Write-Warning "Skipping invalid target: $Target"
        $FailCount++
        continue
    }

    $GOOS = $Parts[0]
    $GOARCH = $Parts[1]
    $BinaryName = if ($GOOS -eq "windows") { "bws.exe" } else { "bws" }
    $ZipName = "bws_${Version}_${GOOS}_${GOARCH}.zip"
    $TempDir = Join-Path $OutputDir "tmp_$GOOS`_$GOARCH"

    Write-Host "  [$Target] Processing..." -NoNewline

    try {
        if (-not $SkipBuild) {
            # Create temp directory
            New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

            # Compile
            $Env:GOOS = $GOOS
            $Env:GOARCH = $GOARCH
            $Env:CGO_ENABLED = "0"

            $BinaryPath = Join-Path $TempDir $BinaryName
            go build -ldflags "$LdFlags" -o "$BinaryPath" "$ProjectRoot"

            if (-not (Test-Path $BinaryPath)) {
                throw "Build failed, binary not generated"
            }
        } else {
            # SkipBuild mode: assume binary already exists in temp dir
            if (-not (Test-Path (Join-Path $TempDir $BinaryName))) {
                throw "Binary not found in skip-build mode: $(Join-Path $TempDir $BinaryName)"
            }
        }

        # Package as zip
        $ZipPath = Join-Path $OutputDir $ZipName
        if (Test-Path $ZipPath) {
            Remove-Item $ZipPath -Force
        }

        $BinaryPath = Join-Path $TempDir $BinaryName
        Compress-Archive -Path $BinaryPath -DestinationPath $ZipPath -Force

        $ZipSize = (Get-Item $ZipPath).Length
        $ZipSizeStr = if ($ZipSize -gt 1MB) {
            "{0:N1} MB" -f ($ZipSize / 1MB)
        } else {
            "{0:N1} KB" -f ($ZipSize / 1KB)
        }

        Write-Host " Done -> $ZipName ($ZipSizeStr)"
        $SuccessCount++
    } catch {
        Write-Host " Failed: $_" -ForegroundColor Red
        $FailCount++
    } finally {
        # Clean up temp directory
        if (Test-Path $TempDir) {
            Remove-Item $TempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

Write-Host ""
Write-Host "========================================"
Write-Host "  Build complete: $SuccessCount success, $FailCount failed"
Write-Host "========================================"

# List output files
if ($SuccessCount -gt 0) {
    Write-Host ""
    Write-Host "  Output files:"
    Get-ChildItem $OutputDir -Filter "bws_*.zip" | Sort-Object Name | ForEach-Object {
        $Size = if ($_.Length -gt 1MB) {
            "{0:N1} MB" -f ($_.Length / 1MB)
        } else {
            "{0:N1} KB" -f ($_.Length / 1KB)
        }
        Write-Host "    $($_.Name)  ($Size)"
    }
}

exit $FailCount