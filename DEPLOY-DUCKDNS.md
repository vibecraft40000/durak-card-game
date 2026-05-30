# Deploy: Git + Docker + Caddy

> Canonical deployment path: `scripts/deploy_duckdns.py` + `docker/docker-compose.caddy.yml`

## Architecture

```
PC (git push) → GitHub → VPS (deploy script or git pull + docker compose)
                         ↓
                    Caddy (auto HTTPS)
```

## 1. Domain and DNS

Set up a domain (e.g., via DuckDNS) pointing to your server IP.

## 2. First run on VPS

```bash
# Install Docker + Git
apt update && apt install -y docker.io docker-compose-v2 git
git clone https://github.com/YOUR_USER/durak-card-game.git /root/durakonline
cd /root/durakonline

# .env with tokens (required!)
nano .env
# VITE_API_BASE_URL=https://your-domain.example
# VITE_WS_URL=wss://your-domain.example/ws
# TELEGRAM_BOT_TOKEN=...
# JWT_SECRET=...

# Ports
ufw allow 80 && ufw allow 443 && ufw reload

# Start
docker compose -f docker/docker-compose.caddy.yml up -d --build
```

## 3. Update (deploy)

**Script (recommended):**

```powershell
$env:DEPLOY_PW = "your_root_password"
python scripts/deploy_duckdns.py
```

**Manually:**

```powershell
npm run build
Copy-Item dist\* docker\frontend-dist -Recurse
git add . && git commit -m "update" && git push
ssh root@YOUR_SERVER_IP "cd /root/durakonline && git pull && docker compose -f docker/docker-compose.caddy.yml up -d --build"
```

## 4. BotFather

Menu Button → `https://your-domain.example`

## Useful commands

```bash
docker logs -f docker-api-1
docker ps
docker restart docker-caddy-1
```
