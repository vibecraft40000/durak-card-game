# Durak Online — Telegram Mini App

Многопользовательская карточная игра **«Дурак»** (Подкидной / Переводной), реализованная как Telegram Mini App.

## Стек

| Компонент | Технология |
|-----------|-----------|
| Frontend | React + TypeScript + Vite |
| Backend | Go (chi, WebSocket, JWT) |
| Database | PostgreSQL |
| Cache | Redis |
| Контейнеризация | Docker, Docker Compose |
| Обратный прокси | Caddy (auto HTTPS) или Nginx |
| Бот | Python (Aiogram 3.x) |

## Возможности

- Реалтайм мультиплеер через WebSocket
- Авторизация через Telegram (initData + JWT)
- Подкидной и Переводной режимы игры
- Система комнат (лобби, создание, приглашение)
- Игровой стол с анимациями карт
- Разделение на роли (атакующий / защитник)
- Таймер хода и обработка отключений
- Telegram Mini App (работает внутри Telegram)
- Административная панель (Flask)
- Поддержка нескольких языков (русский, украинский)

## Быстрый старт (локальная разработка)

```bash
# 1. Запустить Postgres, Redis, миграции
docker compose -f docker/docker-compose.dev.yml up -d

# 2. Запустить backend
cd backend
go run ./cmd/api

# 3. Запустить frontend (в другом терминале)
npm install
npm run dev
```

Фронт доступен на `http://localhost:5173`, API на `http://localhost:8080`.

## Docker-окружение (полный стек)

```bash
docker compose -f docker/docker-compose.yml up --build
```

Запускает: Postgres, Redis, миграции, 2 экземпляра API, балансировщик Nginx, Prometheus, Grafana, k6.

## Структура проекта

```
durak-card-game/
├── backend/           # Go API (chi, WebSocket, JWT, миграции)
│   ├── cmd/api/       # точка входа
│   ├── internal/      # бизнес-логика
│   ├── pkg/           # shared packages
│   └── migrations/    # SQL миграции
├── src/               # React + TypeScript frontend
├── bot/               # Telegram bot (Aiogram 3.x)
├── admin_panel/       # Flask admin panel
├── docker/            # Docker Compose и конфиги
├── k8s/               # Kubernetes manifests
├── helm/              # Helm chart
├── loadtest/          # k6 нагрузочное тестирование
└── docs/              # Документация
```

## Тестирование

```bash
make build
make test
make test-race
make pipeline
```

Docker-тесты (поднимает Postgres + Redis):

```bash
make docker-test
make docker-race
```

## Нагрузочное тестирование (k6)

```bash
make load
```

Сценарий: 200 WebSocket соединений, join_room, make_move, reconnect.

## Мониторинг

- Health: `GET /health`
- Метрики: `GET /metrics` (Prometheus)
- Pprof: `http://127.0.0.1:6060/debug/pprof/` (локально)
- Grafana: `http://localhost:3000` (admin/admin)

## Деплой

См. `DEPLOY-DUCKDNS.md` и `docs/deployment-paths.md`.

## Лицензия

MIT
