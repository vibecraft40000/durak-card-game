# Deployment Paths

В репозитории есть несколько инфраструктурных артефактов, но каноническим считается только один deployment path.

## Canonical Path

**Канонический production-ish path для Mini App:**

- домен `https://your-domain.example`
- VPS
- `scripts/deploy_duckdns.py`
- `docker/docker-compose.caddy.yml`
- `docker/Caddyfile`

Почему именно он:

- корневой `README.md` уже ведет в этот сценарий;
- `scripts/deploy_duckdns.py` реально собирает фронтенд, обновляет `docker/frontend-dist`, загружает `backend/`, `docker/` и `.env` на VPS, затем прогоняет миграции и перезапускает `api`/`caddy`;
- `docker/docker-compose.caddy.yml` поднимает полный runtime для Mini App: `postgres`, `redis`, `migrate`, `api`, `caddy`;
- `docker/Caddyfile` обслуживает frontend и proxy для `/api/*`, `/auth/*`, `/ws`, `/webhooks/*` с auto-HTTPS, что соответствует текущему домену и same-origin модели приложения.

Primary entrypoints внутри этого же path:

- основной day-2 deploy с машины оператора: `python scripts/deploy_duckdns.py`
- ручной fallback на VPS для того же стека: `sh scripts/vps-deploy.sh`
- первичный bootstrap VPS: `sh scripts/vps-setup-git.sh` или ручной `git clone` + `docker compose -f docker/docker-compose.caddy.yml up -d --build`

Важные границы:

- `docker/frontend-dist/` является deployment-артефактом для Caddy-стека, а не исходником frontend;
- этот path покрывает frontend + Go API + Postgres + Redis для Mini App;
- `bot/` и `admin_panel/` остаются отдельными сервисами/утилитами и не деплоятся автоматически этим path.

## Secondary And Legacy Paths

| Path | Файлы | Статус | Почему не canonical |
| --- | --- | --- | --- |
| Raw VPS + Nginx | `DEPLOY-VPS.md`, `docker/docker-compose.vps.yml`, `scripts/deploy-vps.ps1`, `docker/run-on-vps.sh` | Legacy | Решает ту же задачу через другой стек, опирается на Nginx/raw IP/manual HTTPS и конфликтует с текущим Caddy-based source of truth. |
| DuckDNS + Certbot + Nginx | `docker/docker-compose.duckdns.yml`, `docker/docker-compose.duckdns-http.yml`, `scripts/setup-duckdns-ssl.sh` | Legacy | Исторический путь получения TLS через Certbot; функционально дублирует Caddy auto-HTTPS path. |
| Kubernetes manifests | `k8s/` | Secondary | Покрывает только API-слой, требует внешний Postgres/Redis и не описывает доставку frontend/static assets. |
| Helm chart | `helm/durak-online/` | Secondary | Тоже ориентирован только на API, без frontend distribution и без репозиторного CI/CD path, который реально это применяет. |
| Local Docker stack | `docker/docker-compose.yml` | Dev/Test only | Это локальное окружение с dev-настройками, балансировщиком, test/k6/prometheus/grafana, а не production source of truth. |

## Documentation Rule

- `README.md` и `DEPLOY-DUCKDNS.md` описывают primary path.
- `DEPLOY-VPS.md` остается только как исторический/manual fallback.
- `k8s/` и `helm/` считаются secondary до тех пор, пока не появится полноценная доставка frontend, secrets/config discipline и CI/CD, реально использующий этот путь.

## Next Step For Real CI/CD

Чтобы CI/CD действительно опирался на canonical path, следующим этапом нужно:

1. Собирать frontend в CI и выпускать артефакт вместо ручного обновления `docker/frontend-dist`.
2. Перевести deploy на безпарольный SSH key flow или другой non-interactive runner-friendly механизм, сохранив тот же `docker/docker-compose.caddy.yml`.
3. Добавить отдельный deploy workflow для protected branch, который сначала гоняет проверки, а затем обновляет VPS именно через canonical path.
