$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root "web"
$embedded = Join-Path $root "internal\web\dist"
$dist = Join-Path $root "dist"

Push-Location $web
try {
  pnpm install
  pnpm build
}
finally {
  Pop-Location
}

if (Test-Path $embedded) {
  Remove-Item -Recurse -Force $embedded
}
New-Item -ItemType Directory -Force $embedded | Out-Null
Copy-Item -Recurse -Force (Join-Path $web "dist\*") $embedded

New-Item -ItemType Directory -Force $dist | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o (Join-Path $dist "proxy-hub.exe") ./cmd/proxy-hub
