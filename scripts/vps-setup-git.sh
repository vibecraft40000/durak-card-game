#!/bin/sh
# VPS setup: Docker + Git clone + first run
# Usage: ssh root@72.56.74.7 'curl -sL https://raw.../vps-setup-git.sh | sh'
# Or: scp scripts/vps-setup-git.sh root@IP:/tmp/ && ssh root@IP 'sh /tmp/vps-setup-git.sh'

set -e
GIT_REPO="${GIT_REPO:-https://github.com/YOUR_USER/durakonline.git}"
PROJECT_DIR="/root/durakonline"

echo "=== VPS Setup: Docker + Git ==="
echo ""

echo "1. Installing Docker and Git..."
apt-get update -qq && apt-get install -y -qq docker.io docker-compose-v2 git

echo "2. Cloning project..."
if [ -d "$PROJECT_DIR" ]; then
    cd "$PROJECT_DIR"
    git pull
else
    git clone "$GIT_REPO" "$PROJECT_DIR"
    cd "$PROJECT_DIR"
fi

echo "3. Building frontend..."
if [ -f "docker/frontend-dist/index.html" ]; then
    echo "   frontend-dist exists"
elif command -v npm >/dev/null 2>&1; then
    npm run build && mkdir -p docker/frontend-dist && cp -r dist/* docker/frontend-dist/
else
    echo "   WARNING: npm not found. Build locally: npm run build; cp -r dist/* docker/frontend-dist; commit & push"
fi

echo "4. Creating .env (edit with your tokens)..."
if [ ! -f .env ]; then
    cat > .env << 'EOF'
TELEGRAM_BOT_TOKEN=your_token
BOT_TOKEN=your_token
JWT_SECRET=change-me-in-production
EOF
    echo "   Edit /root/durakonline/.env with your tokens!"
fi

echo "5. Starting Docker (Caddy + auto SSL)..."
docker compose -f docker/docker-compose.caddy.yml up -d --build

echo ""
echo "=== Done ==="
echo "App: https://durakonline.duckdns.org"
echo "Set DuckDNS IP to this server. Edit .env with tokens."
