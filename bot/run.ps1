# Запуск бота из терминала (PowerShell)
# Токен читается из .env (BOT_TOKEN или TELEGRAM_BOT_TOKEN)

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# Проверка зависимостей
if (-not (Get-Command python -ErrorAction SilentlyContinue)) {
    Write-Error "Python не найден. Установите Python 3.10+"
    exit 1
}

# Установка зависимостей
python -m pip install -q -r requirements.txt 2>$null

Write-Host "Запуск бота (polling)..." -ForegroundColor Green
python main.py
