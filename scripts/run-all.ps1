# Durak Online - run everything locally
# 1) Docker backend, 2) Frontend, 3) Cloudflared tunnel (VPS SSH forwarding denied by provider)

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "1. Docker backend..." -ForegroundColor Cyan
Push-Location docker
docker compose up -d postgres redis migrate api api2 lb 2>$null
Pop-Location
Start-Sleep -Seconds 5

Write-Host "2. Frontend (Vite)..." -ForegroundColor Cyan
Start-Process -FilePath "npm" -ArgumentList "run","dev" -WorkingDirectory $root -WindowStyle Minimized
Start-Sleep -Seconds 5

$port = 5173
foreach ($p in 5173..5177) {
    try {
        $t = New-Object System.Net.Sockets.TcpClient
        $t.Connect("127.0.0.1", $p)
        $t.Close()
        $port = $p
        break
    } catch {}
}

Write-Host "3. Cloudflared tunnel (port $port)..." -ForegroundColor Cyan
$cf = Join-Path $env:TEMP "cloudflared.exe"
if (-not (Test-Path $cf)) {
    Write-Host "   Downloading cloudflared..."
    Invoke-WebRequest -Uri "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe" -OutFile $cf -UseBasicParsing
}
Start-Process -FilePath $cf -ArgumentList "tunnel","--url","http://localhost:$port","--protocol","http2" -WorkingDirectory $root

Write-Host ""
Write-Host "Done. Backend: 8090 | Frontend: localhost:$port" -ForegroundColor Green
Write-Host "Check the cloudflared window for public URL (https://xxx.trycloudflare.com)" -ForegroundColor Yellow
