# Run Cloudflare Named Tunnel
# Requires: cloudflared tunnel login + setup-named-tunnel.ps1

$tunnelName = "durakonline"
Write-Host "Starting tunnel: $tunnelName" -ForegroundColor Cyan
Write-Host "Vite must be on port 5173. Do not close this window." -ForegroundColor Yellow
Write-Host ""
cloudflared tunnel run $tunnelName
