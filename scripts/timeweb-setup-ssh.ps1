# Timeweb Cloud: SSH key setup for root@72.56.74.7

$ErrorActionPreference = "Stop"
$keyPath = "$env:USERPROFILE\.ssh\id_ed25519"
$keyPathPub = "$keyPath.pub"

Write-Host "=== Timeweb Cloud: SSH key ===" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path $keyPathPub)) {
    Write-Host "1. Creating SSH key..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.ssh" | Out-Null
    $null = Start-Process -FilePath "ssh-keygen" -ArgumentList "-t","ed25519","-f",$keyPath,"-C","durakonline","-N",'""' -Wait -NoNewWindow
    Write-Host "   Done." -ForegroundColor Green
} else {
    Write-Host "1. Key exists: $keyPathPub" -ForegroundColor Green
}

$pubKey = Get-Content $keyPathPub -Raw
Write-Host ""
Write-Host "2. Your PUBLIC key (copy all):" -ForegroundColor Yellow
Write-Host ""
Write-Host $pubKey -ForegroundColor White
Write-Host ""
$pubKey | Set-Clipboard
Write-Host "   [Copied to clipboard]" -ForegroundColor Gray
Write-Host ""

Write-Host "3. Add key in Timeweb Cloud:" -ForegroundColor Cyan
Write-Host "   A) timeweb.cloud/my/servers -> your server -> Access -> Edit -> Add key" -ForegroundColor White
Write-Host "   B) timeweb.cloud/my/sshkeys -> Add -> Paste (Ctrl+V) -> Save" -ForegroundColor White
Write-Host "   Then: server -> Access -> Edit -> Select this key" -ForegroundColor White
Write-Host "   Wait 1-2 min after adding." -ForegroundColor Yellow
Write-Host ""

Start-Process "https://timeweb.cloud/my/servers"
Write-Host "   Opened timeweb.cloud in browser" -ForegroundColor Gray
Write-Host ""

Write-Host "4. After adding key, deploy:" -ForegroundColor Cyan
Write-Host "   .\scripts\deploy-duckdns.ps1" -ForegroundColor White
Write-Host ""
