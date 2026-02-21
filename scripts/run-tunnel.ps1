# Cloudflared tunnel — выводит URL и обновляет .env
# ВАЖНО: не закрывай окно cloudflared! Иначе 530 — The origin has been unregistered

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$port = 5173
$cf = Join-Path $env:TEMP "cloudflared.exe"
if (-not (Test-Path $cf)) {
    Write-Host "Downloading cloudflared..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe" -OutFile $cf -UseBasicParsing
}

$logOut = Join-Path $env:TEMP "cf-out.log"
$logErr = Join-Path $env:TEMP "cf-err.log"
Remove-Item $logOut,$logErr -ErrorAction SilentlyContinue
Write-Host "Starting tunnel on port $port..." -ForegroundColor Cyan
Write-Host ""

$proc = Start-Process -FilePath $cf -ArgumentList "tunnel","--url","http://localhost:$port","--protocol","http2" `
    -PassThru -RedirectStandardOutput $logOut -RedirectStandardError $logErr -NoNewWindow

# Ждём URL в логе
$url = $null
for ($i = 0; $i -lt 25; $i++) {
    Start-Sleep -Seconds 1
    $log = (Get-Content $logOut -Raw -ErrorAction SilentlyContinue) + (Get-Content $logErr -Raw -ErrorAction SilentlyContinue)
    if ($log -match "https://([a-z0-9\-]+)\.trycloudflare\.com") {
        $url = "https://$($Matches[1]).trycloudflare.com"
        break
    }
}

if ($url) {
    Write-Host "=== TUNNEL URL ===" -ForegroundColor Green
    Write-Host $url -ForegroundColor Yellow
    Write-Host "==================" -ForegroundColor Green
    Write-Host ""
    # Обновляем .env
    $envPath = Join-Path $root ".env"
    $content = Get-Content $envPath -Raw -ErrorAction SilentlyContinue
    if ($content -match "WEBAPP_URL=.*") {
        $content = $content -replace "WEBAPP_URL=.*", "WEBAPP_URL=$url"
    } else {
        $content = ($content.TrimEnd() -replace "\s*$") + "`nWEBAPP_URL=$url`n"
    }
    Set-Content $envPath -Value $content.TrimEnd() -NoNewline:$false
    Write-Host "WEBAPP_URL обновлён в .env" -ForegroundColor Cyan
} else {
    Write-Host "URL not found. Check $logOut $logErr" -ForegroundColor Yellow
}

Write-Host ""
$pidMsg = "Туннель запущен PID " + $proc.Id + ". Ne zavershai cloudflared - budet 530."
Write-Host $pidMsg -ForegroundColor Red
Write-Host "Stop: Stop-Process -Id $($proc.Id)" -ForegroundColor Gray
