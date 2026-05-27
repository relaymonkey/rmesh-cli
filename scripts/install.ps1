# Install rmesh from GitHub Releases (Windows amd64).
# Usage:
#   irm https://raw.githubusercontent.com/relaymonkey/rmesh-cli/main/scripts/install.ps1 | iex
#   $env:RMESH_VERSION = "v1.0.1"; .\install.ps1
$ErrorActionPreference = "Stop"

$Repo = if ($env:RMESH_REPO) { $env:RMESH_REPO } else { "relaymonkey/rmesh-cli" }
$GitHub = if ($env:GITHUB) { $env:GITHUB } else { "https://github.com" }
$Api = "https://api.github.com/repos/$Repo"

function Get-LatestTag {
    $headers = @{ Accept = "application/vnd.github+json" }
    $release = Invoke-RestMethod -Uri "$Api/releases/latest" -Headers $headers
    return $release.tag_name
}

if ($env:PROCESSOR_ARCHITECTURE -notmatch "64") {
    throw "prebuilt Windows releases are amd64 only"
}

$tag = if ($env:RMESH_VERSION) { $env:RMESH_VERSION } else { Get-LatestTag }
$version = $tag.TrimStart("v")
$artifact = "rmesh_${version}_windows_amd64.zip"
$url = "$GitHub/$Repo/releases/download/$tag/$artifact"

$installRoot = if ($env:RMESH_INSTALL_DIR) {
    $env:RMESH_INSTALL_DIR
} else {
    Join-Path $env:LOCALAPPDATA "Programs\rmesh"
}
New-Item -ItemType Directory -Force -Path $installRoot | Out-Null

$tmp = Join-Path $env:TEMP "rmesh-install-$version"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$zip = Join-Path $tmp $artifact

Write-Host "Downloading $url"
Invoke-WebRequest -Uri $url -OutFile $zip
Expand-Archive -Path $zip -DestinationPath $tmp -Force

$dest = Join-Path $installRoot "rmesh.exe"
Copy-Item -Path (Join-Path $tmp "rmesh.exe") -Destination $dest -Force

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installRoot*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installRoot", "User")
    $env:Path = "$env:Path;$installRoot"
}

Write-Host "Installed rmesh $version to $dest"
Write-Host "Open a new terminal, then run: rmesh --version"
