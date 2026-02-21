# Deploy to VPS with DuckDNS (durakonline.duckdns.org -> 72.56.74.7 Timeweb)
# Uploads files. Then on VPS run: sh scripts/setup-duckdns-ssl.sh
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
& "$scriptDir\deploy-vps.ps1" -VpsHost 72.56.74.7 -UploadOnly
Write-Host ""
Write-Host "Now on VPS run:" -ForegroundColor Cyan
Write-Host "  ssh root@72.56.74.7" -ForegroundColor Yellow
Write-Host "  cd /root/durakonline && sh scripts/setup-duckdns-ssl.sh" -ForegroundColor Yellow
