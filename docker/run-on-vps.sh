#!/bin/sh
# Run on VPS after upload. Usage: cd /root/durakonline && sh docker/run-on-vps.sh
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"
if [ -f .env ]; then export $(grep -v '^#' .env | xargs); fi
docker compose -f docker/docker-compose.vps.yml up -d --build
echo "Done. App: http://$(hostname -I | awk '{print $1}')/"
