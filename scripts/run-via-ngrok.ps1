# Run app locally and expose via ngrok. Requires: Docker, ngrok.
# Vite listens on 0.0.0.0:5173 so ngrok can connect (not just IPv6).
# ngrok must point to port 5173.

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $ProjectRoot

Write-Host "Starting Docker (postgres, redis, api, lb)..." -ForegroundColor Cyan
docker compose -f docker/docker-compose.yml -f docker/docker-compose.ngrok.yml up -d postgres redis migrate api lb
if ($LASTEXITCODE -ne 0) { exit 1 }

Write-Host "Waiting for API on 8090..." -ForegroundColor Cyan
$attempts = 0
do {
    Start-Sleep -Seconds 2
    try {
        $r = Invoke-WebRequest -Uri "http://localhost:8090/health" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
        if ($r.StatusCode -eq 200) { break }
    } catch {}
    $attempts++
    if ($attempts -ge 30) {
        Write-Host "API did not start in time." -ForegroundColor Red
        exit 1
    }
} while ($true)
Write-Host "API ready." -ForegroundColor Green

Write-Host "Starting ngrok http 5173 (Vite frontend)..." -ForegroundColor Cyan
$ngrok = Start-Process -FilePath "ngrok" -ArgumentList "http", "5173" -PassThru -WindowStyle Normal
Start-Sleep -Seconds 4

$tunnels = $null
try {
    $tunnels = Invoke-RestMethod -Uri "http://127.0.0.1:4040/api/tunnels" -TimeoutSec 5
} catch {
    Write-Host "Could not get ngrok URL. Start ngrok manually: ngrok http 5173" -ForegroundColor Yellow
}
if ($tunnels -and $tunnels.tunnels) {
    $url = ($tunnels.tunnels | Where-Object { $_.proto -eq "https" } | Select-Object -First 1).public_url
    if ($url) {
        Write-Host ""
        Write-Host "=== NGROK URL ===" -ForegroundColor Green
        Write-Host $url -ForegroundColor Yellow
        Write-Host "=================" -ForegroundColor Green
        Write-Host ""
        # Обновляем WEBAPP_URL в .env для бота
        $envPath = Join-Path $ProjectRoot ".env"
        $envContent = Get-Content $envPath -Raw -ErrorAction SilentlyContinue
        if ($envContent -match "WEBAPP_URL=.*") {
            $envContent = $envContent -replace "WEBAPP_URL=.*", "WEBAPP_URL=$url"
        } else {
            $envContent = $envContent.TrimEnd() + "`nWEBAPP_URL=$url`n"
        }
        Set-Content $envPath -Value $envContent.TrimEnd() -NoNewline:$false
        Write-Host "WEBAPP_URL обновлён в .env. Запусти бота в отдельном терминале: cd bot; .\run.ps1" -ForegroundColor Cyan
        Write-Host ""
    }
}

Write-Host "Starting frontend (Vite) on port 5173..." -ForegroundColor Cyan
npm run dev
