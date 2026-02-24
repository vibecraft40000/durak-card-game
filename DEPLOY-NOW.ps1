# Деплой durakonline.duckdns.org
# Один раз создай файл .deploy-pw с паролем root VPS (одна строка, без пробелов)
# Команда: "твой_пароль" | Out-File -Encoding utf8 .deploy-pw

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path ".deploy-pw")) {
    Write-Host "Создай .deploy-pw с паролем root VPS:" -ForegroundColor Yellow
    Write-Host '  "твой_пароль" | Out-File -Encoding utf8 .deploy-pw' -ForegroundColor Cyan
    Write-Host ""
    $pw = Read-Host "Или введи пароль сейчас (будет сохранён в .deploy-pw)"
    if ($pw) {
        $pw | Out-File -Encoding utf8 .deploy-pw
        Write-Host "Пароль сохранён. Больше вводить не нужно." -ForegroundColor Green
    } else {
        exit 1
    }
}

Write-Host "1. Сборка фронтенда..." -ForegroundColor Cyan
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist | Out-Null
Copy-Item dist\* docker\frontend-dist -Recurse -Force

Write-Host "2. Деплой на VPS..." -ForegroundColor Cyan
python scripts/deploy_duckdns.py

Write-Host "`nГотово. https://durakonline.duckdns.org" -ForegroundColor Green
