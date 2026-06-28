# Sync the backend to the Droplet and (optionally) launch the stack.
#
# Usage (run from the waymeet-backend/ directory):
#   ./deploy/deploy.ps1 -DropletIp 203.0.113.45
#   ./deploy/deploy.ps1 -DropletIp 203.0.113.45 -Up      # also build & start
#
# Requires Windows' built-in ssh/scp/tar (OpenSSH + bsdtar, present on Win10/11).
param(
    [Parameter(Mandatory = $true)] [string]$DropletIp,
    [string]$User = "deploy",
    [switch]$Up
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot   # waymeet-backend/
$remote   = "$User@$DropletIp"
$tarball  = Join-Path $env:TEMP "waymeet-backend.tar.gz"

Write-Host "==> Packaging source (excluding binaries/logs/build)..." -ForegroundColor Cyan
# bsdtar honors --exclude; keep deploy/ but drop secrets and the live env file.
tar --create --gzip --file $tarball `
    --exclude='*.exe' --exclude='*.exe~' --exclude='*.log' `
    --exclude='./build' --exclude='./bin' --exclude='./.git' `
    --exclude='./deploy/secrets/*' --exclude='./deploy/.env.prod' `
    -C $repoRoot .

Write-Host "==> Copying to $remote ..." -ForegroundColor Cyan
ssh $remote "mkdir -p ~/waymeet-backend"
scp $tarball "${remote}:~/waymeet-backend.tar.gz"

Write-Host "==> Unpacking on the Droplet..." -ForegroundColor Cyan
ssh $remote "tar -xzf ~/waymeet-backend.tar.gz -C ~/waymeet-backend && rm ~/waymeet-backend.tar.gz"

Remove-Item $tarball -Force

if ($Up) {
    Write-Host "==> Building & starting the stack (needs deploy/.env.prod on the server)..." -ForegroundColor Cyan
    ssh $remote "cd ~/waymeet-backend && docker compose -f deploy/docker-compose.prod.yml --project-directory . --env-file deploy/.env.prod up -d --build"
}

Write-Host "==> Done. Next: ssh $remote  then configure deploy/.env.prod and deploy/secrets/fcm.json" -ForegroundColor Green
