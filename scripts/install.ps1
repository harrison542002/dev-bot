$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$Repo = 'harrison542002/dev-bot'
$Binary = 'devbot'

function Write-Info([string]$Message) {
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Success([string]$Message) {
    Write-Host "OK  $Message" -ForegroundColor Green
}

function Write-Warn([string]$Message) {
    Write-Host "!  $Message" -ForegroundColor Yellow
}

function Fail([string]$Message) {
    Write-Host "X  $Message" -ForegroundColor Red
    exit 1
}

function Get-Arch {
    $arch = if ($env:PROCESSOR_ARCHITEW6432) {
        $env:PROCESSOR_ARCHITEW6432
    } else {
        $env:PROCESSOR_ARCHITECTURE
    }

    switch ($arch) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "Unsupported architecture: $arch" }
    }
}

function Get-LatestVersion {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    if (-not $release.tag_name) {
        Fail 'Could not determine latest version.'
    }
    return [string]$release.tag_name
}

function Test-PathContainsDir {
    param(
        [string]$PathValue,
        [string]$TargetDir
    )

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $false
    }

    $parts = $PathValue -split ';' | Where-Object { $_ }
    foreach ($part in $parts) {
        if ([string]::Equals($part.TrimEnd('\'), $TargetDir.TrimEnd('\'), [System.StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }

    return $false
}

function Ensure-UserPathContainsDir {
    param(
        [string]$TargetDir
    )

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')

    if (-not (Test-PathContainsDir -PathValue $userPath -TargetDir $TargetDir) -and -not (Test-PathContainsDir -PathValue $machinePath -TargetDir $TargetDir)) {
        $updatedUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
            $TargetDir
        } else {
            "$userPath;$TargetDir"
        }
        [Environment]::SetEnvironmentVariable('Path', $updatedUserPath, 'User')
        Write-Success "Added $TargetDir to your user PATH"
    }

    if (-not (Test-PathContainsDir -PathValue $env:Path -TargetDir $TargetDir)) {
        $env:Path = "$TargetDir;$env:Path"
    }
}

$arch = Get-Arch
$version = if ($env:DEVBOT_VERSION) { $env:DEVBOT_VERSION } else { Get-LatestVersion }
$archiveName = "$Binary-windows-$arch.zip"
$archiveUrl = "https://github.com/$Repo/releases/download/$version/$archiveName"
$installDir = if ($env:DEVBOT_INSTALL_DIR) { $env:DEVBOT_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\DevBot\bin' }
$binaryName = "$Binary-windows-$arch.exe"
$targetPath = Join-Path $installDir "$Binary.exe"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("devbot-install-" + [System.Guid]::NewGuid().ToString('N'))
$archivePath = Join-Path $tempDir $archiveName
$extractedBinary = Join-Path $tempDir $binaryName

Write-Info 'Fetching latest release...'
Write-Info "Version: $version"

New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    Write-Info "Downloading $archiveName..."
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath

    Write-Info 'Extracting...'
    Expand-Archive -LiteralPath $archivePath -DestinationPath $tempDir -Force

    if (-not (Test-Path -LiteralPath $extractedBinary)) {
        Fail "Binary not found in archive: $binaryName"
    }

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Move-Item -LiteralPath $extractedBinary -Destination $targetPath -Force
    Write-Success "Installed $Binary -> $targetPath"

    Ensure-UserPathContainsDir -TargetDir $installDir

    Write-Host ''
    Write-Success "DevBot $version installed successfully!"
    Write-Host ''
    Write-Host "  Run:          devbot"
    Write-Host ''
}
catch {
    $message = $_.Exception.Message
    Fail "Download or install failed. Check that $version includes $archiveName.`n  URL: $archiveUrl`n  Error: $message"
}
finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}
