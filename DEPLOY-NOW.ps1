# Деплой на VPS
# Пароль передаётся через DEPLOY_PW или будет запрошен

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not $env:DEPLOY_PW) {
    $pw = Read-Host "Введи пароль root для VPS"
} else {
    $pw = $env:DEPLOY_PW
}

Write-Host "1. Сборка фронтенда..." -ForegroundColor Cyan
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist | Out-Null
Copy-Item dist\* docker\frontend-dist -Recurse -Force

Write-Host "2. Деплой на VPS..." -ForegroundColor Cyan
python scripts/deploy_duckdns.py

Write-Host "`nГотово." -ForegroundColor Green
