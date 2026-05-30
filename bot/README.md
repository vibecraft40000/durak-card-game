# Durak Online — Telegram Bot (Aiogram 3.x)

Бот для открытия WebApp игры «Дурак Онлайн».

## Возможности

- **`/start`** — считывает данные пользователя (id, username, имя), логирует и показывает приветствие
- При открытии бота можно запустить WebApp через Menu Button (иконка слева от поля ввода)
- **Админка** — команда **`/admin`** только для пользователей из `ADMIN_IDS`:
  - **Статистика** — число подписчиков бота и (если настроены `API_BASE_URL` и `ADMIN_SECRET`) число пользователей в БД с бэкенда
  - **Рассылка** — отправить сообщение всем, кто нажимал `/start`

## Переменные окружения

| Переменная       | Описание                                                                 |
|------------------|--------------------------------------------------------------------------|
| `BOT_TOKEN`      | Токен бота от @BotFather                                                |
| `TELEGRAM_BOT_TOKEN` | Альтернатива к BOT_TOKEN                                            |
| `ADMIN_IDS`      | ID админов через запятую (узнать свой: @userinfobot). Доступ к `/admin` |
| `WEBAPP_URL`     | URL WebApp (например, фронтенда)                                        |
| `WEBHOOK_URL`    | Полный URL webhook (HTTPS), если нужен                                   |
| `API_BASE_URL`   | (Опционально) URL бэкенда для статистики в админке                      |
| `ADMIN_SECRET`   | (Опционально) Секрет для запроса `/admin/stats` на бэкенде               |

## Запуск из терминала

### Windows (PowerShell)

```powershell
cd bot
.\run.ps1
```

или

```powershell
cd bot
$env:BOT_TOKEN="ваш_токен"; $env:ADMIN_IDS="ваш_telegram_id"; python main.py
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
BOT_TOKEN=your-token WEBAPP_URL=https://your-domain.example python main.py
```

Токен можно задать в `.env` в корне проекта или в `bot/.env`.
