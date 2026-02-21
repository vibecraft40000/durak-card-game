# Deploy to durakonline.duckdns.org (manual - enter password when prompted)
# Password: (your root password for 72.56.74.7)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$VpsHost = "72.56.74.7"

Set-Location $ProjectRoot

Write-Host "1. Building frontend..." -ForegroundColor Cyan
npm run build
New-Item -ItemType Directory -Force -Path "docker\frontend-dist" | Out-Null
Copy-Item -Path "dist\*" -Destination "docker\frontend-dist" -Recurse -Force

Write-Host "2. Creating remote dir (enter password)..." -ForegroundColor Cyan
ssh -o StrictHostKeyChecking=accept-new root@$VpsHost "mkdir -p /root/durakonline"

Write-Host "3. Uploading backend (enter password)..." -ForegroundColor Cyan
scp -r -o StrictHostKeyChecking=accept-new backend root@${VpsHost}:/root/durakonline/

Write-Host "4. Uploading docker (enter password)..." -ForegroundColor Cyan
scp -r -o StrictHostKeyChecking=accept-new docker root@${VpsHost}:/root/durakonline/

Write-Host "5. Uploading .env (enter password)..." -ForegroundColor Cyan
scp -o StrictHostKeyChecking=accept-new .env root@${VpsHost}:/root/durakonline/

Write-Host "6. Running setup on VPS (enter password)..." -ForegroundColor Cyan
ssh root@$VpsHost "cd /root/durakonline && sh scripts/setup-duckdns-ssl.sh"

Write-Host ""
Write-Host "Done. App: https://durakonline.duckdns.org" -ForegroundColor Green
