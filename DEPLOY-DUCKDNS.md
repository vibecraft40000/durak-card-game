# Deploy: Git + Docker + Caddy (durakonline.duckdns.org)

## Архитектура

```
ПК (git push) → GitHub → VPS (git pull + docker compose)
                         ↓
                    Caddy (auto HTTPS)
```

Никаких ручных загрузок файлов.

---

## 1. DuckDNS

- [duckdns.org](https://www.duckdns.org) → IP: **72.56.74.7** (твой VPS Timeweb)
- Update IP

## 2. GitHub

**На ПК:**

```powershell
cd "c:\Users\GANG\Desktop\cursor project\durakonline"

# Сборка фронта (в docker/frontend-dist)
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist
Copy-Item dist\* docker\frontend-dist -Recurse

git init
git add .
git commit -m "init"
git remote add origin https://github.com/ТВОЙ_USER/durakonline.git
git push -u origin main
```

## 3. Первый запуск на VPS

```powershell
ssh root@72.56.74.7
```

На сервере:

```bash
# Установка Docker, Git, клонирование
apt update && apt install -y docker.io docker-compose-v2 git
git clone https://github.com/ТВОЙ_USER/durakonline.git /root/durakonline
cd /root/durakonline

# .env с токенами
nano .env
# TELEGRAM_BOT_TOKEN=...
# BOT_TOKEN=...
# JWT_SECRET=change-me

# Порты
ufw allow 80 && ufw allow 443 && ufw reload

# Запуск (Caddy сам получит SSL)
docker compose -f docker/docker-compose.caddy.yml up -d --build
```

## 4. Обновление (деплой)

**На ПК после изменений:**

```powershell
npm run build
Copy-Item dist\* docker\frontend-dist -Recurse
git add .
git commit -m "update"
git push
```

**На VPS:**

```bash
cd /root/durakonline
git pull
docker compose -f docker/docker-compose.caddy.yml up -d --build
```

Или одной командой с ПК:
```powershell
ssh root@72.56.74.7 "cd /root/durakonline && git pull && docker compose -f docker/docker-compose.caddy.yml up -d --build"
```

## 5. BotFather

Menu Button → `https://durakonline.duckdns.org`

---

## Caddy vs Nginx + Certbot

- **Caddy** — SSL автоматически, без certbot
- **Nginx + Certbot** — ручная настройка (см. `scripts/setup-duckdns-ssl.sh`)
