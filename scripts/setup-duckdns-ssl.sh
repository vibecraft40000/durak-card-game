#!/bin/sh
# Run on VPS: cd /root/durakonline && sh scripts/setup-duckdns-ssl.sh
# Prerequisites: your-domain.example -> server IP, ports 80 and 443 open

set -e
cd "$(dirname "$0")/.."
DOMAIN="your-domain.example"

echo "=== DuckDNS + Let's Encrypt ==="
echo "Domain: $DOMAIN"
echo ""

[ -f .env ] && export $(grep -v '^#' .env | xargs)

if [ ! -f docker/frontend-dist/index.html ]; then
  echo "ERROR: docker/frontend-dist empty. Deploy first: .\\scripts\\deploy-vps.ps1 -VpsHost YOUR_IP"
  exit 1
fi

echo "1. Stopping any services on port 80..."
docker compose -f docker/docker-compose.vps.yml down 2>/dev/null || true
docker compose -f docker/docker-compose.duckdns-http.yml down 2>/dev/null || true
docker compose -f docker/docker-compose.duckdns.yml down 2>/dev/null || true

echo "2. Installing certbot..."
apt-get update -qq && apt-get install -y -qq certbot 2>/dev/null || true

echo "3. Getting certificate (standalone, port 80)..."
certbot certonly --standalone \
  -d "$DOMAIN" -d "www.$DOMAIN" \
  --non-interactive --agree-tos --email "admin@$DOMAIN" \
  --preferred-challenges http

if [ ! -d "/etc/letsencrypt/live/$DOMAIN" ]; then
  echo "ERROR: No cert at /etc/letsencrypt/live/$DOMAIN"
  echo "Check: 1) DuckDNS IP = this server  2) ufw allow 80 443 && ufw reload"
  exit 1
fi

echo "4. Starting HTTPS stack..."
docker compose -f docker/docker-compose.duckdns.yml up -d --build

echo ""
echo "=== Done ==="
echo "App: https://$DOMAIN"
echo "BotFather Menu Button: https://$DOMAIN"
