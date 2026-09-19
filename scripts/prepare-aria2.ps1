$ErrorActionPreference = "Stop"
$version = "1.37.0"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$temp = Join-Path $env:RUNNER_TEMP "bang-aria2"
$archive = Join-Path $temp "aria2.zip"
$url = "https://github.com/aria2/aria2/releases/download/release-$version/aria2-$version-win-64bit-build1.zip"
$target = Join-Path $root "internal/engine/binaries/windows-amd64-aria2c.exe"

Remove-Item $temp -Recurse -Force -ErrorAction SilentlyContinue
New-Item $temp -ItemType Directory | Out-Null
Invoke-WebRequest $url -OutFile $archive
Expand-Archive $archive -DestinationPath $temp
$binary = Get-ChildItem $temp -Recurse -Filter aria2c.exe | Select-Object -First 1
if (-not $binary) { throw "aria2c.exe was not found in the pinned archive" }
Copy-Item $binary.FullName $target -Force
$firstLine = & $target --version | Select-Object -First 1
if ($firstLine -notlike "*aria2 version $version*") { throw "Unexpected aria2 version: $firstLine" }
Write-Host "Embedded $target"
