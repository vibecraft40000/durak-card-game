# Durak Online — Telegram Bot (Aiogram 3.x)

Бот для открытия WebApp игры «Дурак Онлайн».

## Возможности

- **`/start`** — считывает данные пользователя (id, username, имя), логирует и показывает кнопку «Играть»
- При нажатии «Играть» открывается WebApp; Telegram передаёт данные в мини-приложение через `initData`

## Переменные окружения

| Переменная       | Описание                                  |
|------------------|-------------------------------------------|
| `BOT_TOKEN`      | Токен бота от @BotFather                  |
| `TELEGRAM_BOT_TOKEN` | Альтернатива к BOT_TOKEN               |
| `WEBAPP_URL`     | URL WebApp (например, фронтенда)          |
| `WEBHOOK_URL`    | Полный URL webhook (HTTPS), если нужен    |

## Запуск из терминала

### Windows (PowerShell)

```powershell
cd bot
.\run.ps1
```

или

```powershell
cd bot
$env:BOT_TOKEN="8272949218:AAH..."; python main.py
```

### Windows (cmd)

```cmd
cd bot
run.bat
```

### Linux / macOS

```bash
cd bot
pip install -r requirements.txt
BOT_TOKEN=your-token WEBAPP_URL=https://your-domain.com python main.py
```

Токен можно задать в `.env` в корне проекта или в `bot/.env`.
