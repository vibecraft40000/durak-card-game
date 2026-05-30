# Deploy to VPS with DuckDNS (your-domain.example -> YOUR_SERVER_IP)
# Uploads files. Then on VPS run: sh scripts/setup-duckdns-ssl.sh
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
& "$scriptDir\deploy-vps.ps1" -VpsHost YOUR_SERVER_IP -UploadOnly
Write-Host ""
Write-Host "Now on VPS run:" -ForegroundColor Cyan
Write-Host "  ssh root@YOUR_SERVER_IP" -ForegroundColor Yellow
Write-Host "  cd /root/durakonline && sh scripts/setup-duckdns-ssl.sh" -ForegroundColor Yellow
