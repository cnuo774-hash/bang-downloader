$ErrorActionPreference = "Stop"
$version = "1.37.0"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$tempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { [System.IO.Path]::GetTempPath() }
$temp = Join-Path $tempRoot ("bang-aria2-" + [guid]::NewGuid().ToString())
$archive = Join-Path $temp "aria2.zip"
$url = "https://github.com/aria2/aria2/releases/download/release-$version/aria2-$version-win-64bit-build1.zip"
$target = Join-Path $root "internal/engine/binaries/windows-amd64-aria2c.exe"

New-Item $temp -ItemType Directory | Out-Null
try {
Invoke-WebRequest $url -OutFile $archive
$expected = "67d015301eef0b612191212d564c5bb0a14b5b9c4796b76454276a4d28d9b288"
if ((Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw "aria2 archive checksum mismatch" }
Expand-Archive $archive -DestinationPath $temp
$binary = Get-ChildItem $temp -Recurse -Filter aria2c.exe | Select-Object -First 1
if (-not $binary) { throw "aria2c.exe was not found in the pinned archive" }
Copy-Item $binary.FullName $target -Force
$noticeDir = Join-Path $root "build/third-party"
New-Item $noticeDir -ItemType Directory -Force | Out-Null
Copy-Item (Join-Path $binary.DirectoryName "COPYING") (Join-Path $noticeDir "ARIA2-COPYING")
Copy-Item (Join-Path $binary.DirectoryName "LICENSE.OpenSSL") $noticeDir
Copy-Item (Join-Path $binary.DirectoryName "README.mingw") $noticeDir
$firstLine = & $target --version | Select-Object -First 1
if ($firstLine -notlike "*aria2 version $version*") { throw "Unexpected aria2 version: $firstLine" }
Write-Host "Embedded $target"
} finally {
    Remove-Item $temp -Recurse -Force -ErrorAction SilentlyContinue
}
