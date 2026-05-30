#!/bin/sh
# VPS deploy: git pull + rebuild
# Usage: ssh root@YOUR_SERVER_IP 'cd /root/durakonline && sh scripts/vps-deploy.sh'

set -e
cd "$(dirname "$0")/.."

echo "=== Deploy ==="
echo "1. git pull..."
git pull

echo "2. docker compose up --build..."
docker compose -f docker/docker-compose.caddy.yml up -d --build

echo ""
echo "Done. https://your-domain.example"
