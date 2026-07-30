# install.ps1
#
# ecsctl installer for Windows
# Downloads the precompiled ecsctl binary from GitHub Releases, extracts it, and adds it to the User PATH.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$Repo = "Roslaan001/ecsctl"
$Binary = "ecsctl"

# Detect Architecture
$Arch = $env:PROCESSOR_ARCHITECTURE
switch ($Arch) {
    "AMD64" { $ArchName = "amd64" }
    "ARM64" { $ArchName = "arm64" }
    Default {
        Write-Error "Unsupported architecture: $Arch. Only AMD64 and ARM64 are supported."
        Exit 1
    }
}

# Resolve latest release from GitHub API
Write-Host "Resolving the latest release version for $Repo..."
$LatestReleaseUrl = "https://api.github.com/repos/$Repo/releases/latest"
if ([string]::IsNullOrEmpty($Version)) {
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        $ReleaseInfo = Invoke-RestMethod -Uri $LatestReleaseUrl -Headers @{"User-Agent"="ecsctl-installer"} -UseBasicParsing
        $Version = $ReleaseInfo.tag_name
    } catch {
        $Version = "v0.0.1"
    }
}
if ([string]::IsNullOrEmpty($Version) -or $Version -eq "null") {
    $Version = "v0.0.1"
}

$VersionClean = $Version.TrimStart('v')
Write-Host "Selected version: $Version"

# Format download URL
$FileName = "${Binary}_${VersionClean}_windows_${ArchName}.zip"
$DownloadUrl = "https://ecsctl.abdulsomad005.workers.dev/bin/$FileName"

# Create installation directory
$InstallDir = Join-Path $env:USERPROFILE ".ecsctl\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

# Temporary download path
$TempDir = Join-Path $env:TEMP ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TempDir | Out-Null
$ZipPath = Join-Path $TempDir $FileName

Write-Host "Downloading ecsctl from: $DownloadUrl"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath -UseBasicParsing
} catch {
    Write-Error "Failed to download ecsctl. Please verify that version $Version is published and contains the Windows assets."
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
    Exit 1
}

Write-Host "Extracting archive..."
try {
    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
} catch {
    Write-Error "Failed to extract the ZIP archive."
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
    Exit 1
}

$ExeSource = Join-Path $TempDir "${Binary}.exe"
if (!(Test-Path $ExeSource)) {
    Write-Error "Executable ${Binary}.exe not found in the downloaded archive."
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
    Exit 1
}

# Copy binary to install directory
$ExeDestination = Join-Path $InstallDir "${Binary}.exe"
Copy-Item -Path $ExeSource -Destination $ExeDestination -Force
Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue

Write-Host "Successfully installed ecsctl to $ExeDestination"
Write-Host "Tip: Run 'ecsctl completion powershell' to set up shell autocompletions!"

# Add to user PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -split ';' -notcontains $InstallDir) {
    Write-Host "Adding $InstallDir to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", $UserPath + ";" + $InstallDir, "User")
    # Update current session path
    $env:Path += ";$InstallDir"
    Write-Host "PATH updated. Please restart your terminal/IDE for changes to take effect."
} else {
    Write-Host "$InstallDir is already in user PATH."
}

# Verify installation
if (Get-Command "ecsctl" -ErrorAction SilentlyContinue) {
    Write-Host "Verifying installation version..."
    ecsctl version
} else {
    Write-Host "ecsctl is installed, but you will need to restart your terminal session to use it."
}
