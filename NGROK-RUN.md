# Запуск Mini App (локально + туннель)

## Вариант 1: Cloudflared (рекомендуется)

Cloudflared работает стабильнее ngrok (TCP, нет блокировок).

```powershell
cd "c:\Users\GANG\Desktop\cursor project\durakonline"
.\scripts\run-all.ps1
```

Скрипт запускает: Docker backend → Vite frontend → Cloudflared.  
URL появится в окне cloudflared (например `https://xxx.trycloudflare.com`).

## Вариант 2: ngrok

```powershell
.\scripts\run-via-ngrok.ps1
```

Скрипт автоматически пишет ngrok-URL в `.env` как `WEBAPP_URL`. В другом терминале запусти бота:

```powershell
cd bot
.\run.ps1
```

**Примечание:** ngrok может блокировать IP или давать ошибки (QUIC, IPv6).

## Настройка BotFather

1. Скопируй публичный URL (cloudflared или ngrok)
2. [@BotFather](https://t.me/BotFather) → твой бот → Bot Settings → Menu Button → Configure menu button → вставь URL
3. Открой бота в Telegram → нажми кнопку меню → Mini App

**Требования:** Docker, npm, ngrok. В `.env` — `BOT_TOKEN` или `TELEGRAM_BOT_TOKEN`.
