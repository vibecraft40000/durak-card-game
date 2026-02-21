# Cloudflare Named Tunnel - durakonline.duckdns.org
# Перед запуском: cloudflared tunnel login (выбери durakonline.duckdns.org)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$tunnelName = "durakonline"
$domain = "durakonline.duckdns.org"

Set-Location $root

Write-Host "1. Creating tunnel '$tunnelName'..." -ForegroundColor Cyan
cloudflared tunnel create $tunnelName 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "   Tunnel may already exist, continuing..." -ForegroundColor Yellow
}

Write-Host "2. Routing DNS: $domain -> tunnel..." -ForegroundColor Cyan
cloudflared tunnel route dns $tunnelName $domain
if ($LASTEXITCODE -ne 0) {
    Write-Host "   Route may exist. Continuing..." -ForegroundColor Yellow
}

Write-Host "3. Creating config..." -ForegroundColor Cyan
$configDir = Join-Path $env:USERPROFILE ".cloudflared"
$configPath = Join-Path $configDir "config.yml"
$tunnelId = $null
try {
    $list = cloudflared tunnel list -o json 2>$null | ConvertFrom-Json
    $t = $list | Where-Object { $_.name -eq $tunnelName }
    if ($t) { $tunnelId = $t.id }
} catch {}
if (-not $tunnelId) {
    $tunnelId = (Get-ChildItem $configDir -Filter "*.json" -ErrorAction SilentlyContinue | Select-Object -First 1).BaseName
}
$credPath = Join-Path $configDir "$tunnelId.json"

$config = @"
tunnel: $tunnelName
credentials-file: $credPath

ingress:
  - hostname: $domain
    service: http://localhost:5173
  - hostname: www.$domain
    service: http://localhost:5173
  - service: http_status:404
"@
Set-Content $configPath -Value $config -Encoding UTF8
Write-Host "   Config: $configPath" -ForegroundColor Gray

Write-Host ""
Write-Host "4. Updating .env WEBAPP_URL..." -ForegroundColor Cyan
$envPath = Join-Path $root ".env"
$content = Get-Content $envPath -Raw -ErrorAction SilentlyContinue
if ($content -match "WEBAPP_URL=.*") {
    $content = $content -replace "WEBAPP_URL=.*", "WEBAPP_URL=https://$domain"
} else {
    $content = $content.TrimEnd() + "`nWEBAPP_URL=https://$domain`n"
}
Set-Content $envPath -Value $content.TrimEnd() -NoNewline:$false

Write-Host ""
Write-Host "=== Done ===" -ForegroundColor Green
Write-Host "URL: https://$domain" -ForegroundColor Yellow
Write-Host ""
Write-Host "Run tunnel: cloudflared tunnel run $tunnelName" -ForegroundColor Cyan
Write-Host "Or: .\scripts\run-named-tunnel.ps1" -ForegroundColor Cyan
