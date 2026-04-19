$ErrorActionPreference = "Stop"

$Repo = "channyeintun/nami"
$BinaryName = "nami"
$EngineName = "nami-engine"

function Get-WindowsArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported Windows architecture: $arch" }
    }
}

function Ensure-BunAvailable {
    if (Get-Command bun -ErrorAction SilentlyContinue) {
        return
    }

    Write-Host ""
    Write-Host "Install failed: this Nami release uses a Bun launcher, but 'bun' was not found on PATH."
    Write-Host "Install Bun first, then rerun this installer:"
    Write-Host "  https://bun.sh"
    Write-Host ""
    throw "bun is required"
}

function Add-ToUserPath {
    param([string]$PathEntry)

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ([string]::IsNullOrWhiteSpace($currentPath)) {
        [Environment]::SetEnvironmentVariable("Path", $PathEntry, "User")
        return
    }

    $entries = $currentPath.Split(';', [System.StringSplitOptions]::RemoveEmptyEntries)
    if ($entries -contains $PathEntry) {
        return
    }

    [Environment]::SetEnvironmentVariable("Path", "$PathEntry;$currentPath", "User")
}

$Arch = Get-WindowsArch
$Platform = "windows-$Arch"
$Archive = "$BinaryName-$Platform.zip"
$ArchiveUrl = "https://github.com/$Repo/releases/latest/download/$Archive"
$InstallDir = if ($env:INSTALL_DIR) {
    $env:INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "Programs\nami\bin"
}

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nami-install-" + [System.Guid]::NewGuid().ToString("N"))
$ArchivePath = Join-Path $TempDir $Archive

New-Item -ItemType Directory -Path $TempDir | Out-Null
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

try {
    Write-Host "Detected platform: $Platform"
    Write-Host "Downloading $Archive..."
    Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath

    Write-Host "Expanding release archive..."
    Expand-Archive -Path $ArchivePath -DestinationPath $TempDir -Force

    $ReleaseDir = Join-Path $TempDir "$BinaryName-$Platform"
    $LauncherPath = Join-Path $ReleaseDir $BinaryName
    $WrapperPath = Join-Path $ReleaseDir "$BinaryName.cmd"
    $EnginePath = Join-Path $ReleaseDir "$EngineName.exe"

    foreach ($required in @($LauncherPath, $WrapperPath, $EnginePath)) {
        if (-not (Test-Path $required)) {
            throw "Release archive is missing required file: $required"
        }
    }

    Ensure-BunAvailable

    Write-Host "Installing to $InstallDir..."
    Copy-Item -Path $LauncherPath -Destination (Join-Path $InstallDir $BinaryName) -Force
    Copy-Item -Path $WrapperPath -Destination (Join-Path $InstallDir "$BinaryName.cmd") -Force
    Copy-Item -Path $EnginePath -Destination (Join-Path $InstallDir "$EngineName.exe") -Force

    Add-ToUserPath -PathEntry $InstallDir

    Write-Host ""
    Write-Host "nami installed successfully!"
    Write-Host "Installed to: $InstallDir"
    Write-Host ""
    Write-Host "Open a new PowerShell window, then verify installation:"
    Write-Host "  nami --help"
    Write-Host ""
    Write-Host "If you use a model provider that needs an API key, set it before starting Nami."
} finally {
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force
    }
}