# Deploy: Git + Docker + Caddy (durakonline.duckdns.org)

## Архитектура

```
ПК (git push) → GitHub → VPS (deploy_duckdns.py или git pull + docker compose)
                         ↓
                    Caddy (auto HTTPS)
```

---

## Автообновление

**Нет.** При `git push` на GitHub файлы **не обновляются** на VPS автоматически. Нужно вручную:
- запустить `python scripts/deploy_duckdns.py`, или
- по SSH: `git pull` + `docker compose ... up -d --build`

Чтобы сделать автодеплой, можно настроить **GitHub Actions**: workflow на каждый push будет по SSH подключаться к VPS и выполнять deploy.

---

## Что было сделано (чеклист)

| Действие | Статус |
|----------|--------|
| Git init, commit, push в GitHub | ✅ |
| Репозиторий: github.com/martaliman671-cpu/durakonline | ✅ |
| Caddyfile (SSL для durakonline.duckdns.org) | ✅ |
| docker-compose.caddy.yml (postgres, redis, migrate, api, caddy) | ✅ |
| deploy_duckdns.py (загрузка на VPS через SSH) | ✅ |
| Установка Docker на VPS | ✅ |
| **Docker login на VPS** (обход rate limit) | ✅ `docker login` в SSH |
| **Swap 2GB на VPS** (Go build падал по OOM) | ✅ fallocate + mkswap + swapon |
| **Остановка nginx** (освобождение 80/443) | ✅ systemctl stop nginx |
| Первый запуск compose | ✅ |
| HTTPS: durakonline.duckdns.org | ✅ 200 OK |

---

## 1. DuckDNS

- [duckdns.org](https://www.duckdns.org) → IP: **72.56.74.7** (VPS Timeweb)
- Убедись, что durakonline.duckdns.org указывает на этот IP

## 2. GitHub

**На ПК:**

```powershell
cd "c:\Users\GANG\Desktop\cursor project\durakonline"

# Сборка фронта (в docker/frontend-dist)
npm run build
New-Item -ItemType Directory -Force -Path docker\frontend-dist
Copy-Item dist\* docker\frontend-dist -Recurse

git add .
git commit -m "update"
git push
```

## 3. Первый запуск на VPS (если с нуля)

```powershell
ssh root@72.56.74.7
```

На сервере:

```bash
# Docker + Git
apt update && apt install -y docker.io docker-compose-v2 git
git clone https://github.com/martaliman671-cpu/durakonline.git /root/durakonline
cd /root/durakonline

# Docker Hub (обязательно, иначе rate limit)
docker login

# Swap 2GB (если VPS ~1GB RAM — иначе go build упадёт по OOM)
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
grep -q swapfile /etc/fstab || echo "/swapfile none swap sw 0 0" >> /etc/fstab

# Если nginx занимает 80/443 — остановить
systemctl stop nginx
systemctl disable nginx

# .env с токенами (обязательно!)
nano .env
# VITE_API_BASE_URL=https://durakonline.duckdns.org
# VITE_WS_URL=wss://durakonline.duckdns.org/ws
# TELEGRAM_BOT_TOKEN=...
# JWT_SECRET=...   # секрет для JWT, нужен для авторизации
# VITE_FORCE_DEV_AUTH=false   # обязательно для продакшена (реальный initData из Mini App)
# ALLOW_DEV_TELEGRAM_AUTH=true  # если initData пустой (Menu Button, некоторые клиенты)
# CRYPTO_PAY_API_TOKEN=...    # для пополнения через Crypto Bot

# Порты
ufw allow 80 && ufw allow 443 && ufw reload

# Запуск (--force-recreate api — чтобы подхватить изменения .env)
docker compose -f docker/docker-compose.caddy.yml up -d --build --force-recreate api
```

## 4. Обновление (деплой)

**Скрипт (рекомендуется):**

```powershell
cd "c:\Users\GANG\Desktop\cursor project\durakonline"
$env:DEPLOY_PW = "пароль_root"
python scripts/deploy_duckdns.py
```

Скрипт: собирает фронт, загружает файлы по SCP, выполняет `docker compose up -d --build` на VPS.

**Вручную:**

```powershell
# 1. Сборка фронта
npm run build
Copy-Item dist\* docker\frontend-dist -Recurse

# 2. Git
git add .
git commit -m "update"
git push

# 3. На VPS
ssh root@72.56.74.7 "cd /root/durakonline && git pull && docker compose -f docker/docker-compose.caddy.yml up -d --build"
```

## 5. BotFather

Menu Button → `https://durakonline.duckdns.org`

## 6. CryptoBot (Crypto Pay) — пополнение баланса

1. Открой [@CryptoBot](https://t.me/CryptoBot?start=pay) → Crypto Pay → Create App
2. Получи API Token и добавь в `.env`:
   ```
   CRYPTO_PAY_API_TOKEN=12345:ABC...
   ```
3. Webhooks → Enable → URL: `https://durakonline.duckdns.org/webhooks/cryptopay`
4. Перезапусти API (`docker compose up -d api --force-recreate`)

---

## Полезные команды

```bash
# Логи API
docker logs -f docker-api-1

# Статус контейнеров
docker ps

# Рестарт Caddy
docker restart docker-caddy-1
```
