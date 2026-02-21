# Запуск на VPS (72.56.74.7)

## Быстрый старт

1. **Собери фронт и залей файлы на сервер** (в PowerShell из корня проекта):

```powershell
# Сборка фронта и копирование в docker/frontend-dist
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist
Copy-Item -Path dist\* -Destination docker\frontend-dist -Recurse -Force

# Заливка на VPS (по запросу введи пароль)
scp -r -o StrictHostKeyChecking=accept-new backend docker root@72.56.74.7:/root/durakonline/
scp -o StrictHostKeyChecking=accept-new .env root@72.56.74.7:/root/durakonline/
```

2. **Подключись по SSH и запусти контейнеры**:

```powershell
ssh -o StrictHostKeyChecking=accept-new root@72.56.74.7
```

На сервере:

```bash
cd /root/durakonline
sh docker/run-on-vps.sh
```

Либо одной командой с твоего ПК (после загрузки файлов):

```powershell
ssh root@72.56.74.7 "cd /root/durakonline && docker compose -f docker/docker-compose.vps.yml up -d --build"
```

3. **Проверка**: открой в браузере `http://72.56.74.7/` — должна открыться игра.

4. **Telegram**: для Mini App нужен HTTPS. Варианты:
   - Настроить домен (например duckdns) и Nginx + Certbot на VPS, затем в BotFather указать `https://твой-домен/`.
   - Либо временно для теста можно использовать `http://72.56.74.7` (не все клиенты Telegram это поддерживают).

## Переменные на VPS

В `/root/durakonline/.env` на сервере (или в переменных окружения) задай:

- `TELEGRAM_BOT_TOKEN` — токен бота из BotFather.
- `ALLOWED_ORIGIN` — URL Mini App (например `https://твой-домен.com`), чтобы CORS и Telegram принимали запросы.
- `JWT_SECRET` — секрет для JWT (придумай свой для продакшена).
- `ALLOW_DEV_TELEGRAM_AUTH` — `true` только для теста без Telegram; в проде лучше `false`.

## Автодеплой (если настроен SSH по ключу)

Из корня проекта:

```powershell
.\scripts\deploy-vps.ps1
```

Без ключа используй шаги из раздела «Быстрый старт» выше.
