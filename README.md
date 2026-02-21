# Durak Online Mini App

Первый инкремент проекта Telegram Mini App под игру "Дурак Онлайн".

## Стек

- React + TypeScript
- Vite
- React Router
- FSD-like структура в `src/`

## Frontend запуск

```bash
npm install
npm run dev
```

## Локальный тест без backend API

Чтобы посмотреть весь фронт без Go API, включите mock режим:

```bash
copy .env.example .env
npm run dev
```

При `VITE_USE_MOCK_API=true` доступны:

- список комнат (`GET /api/rooms`)
- создание комнаты (`POST /api/rooms`)
- вход в комнату (`POST /api/rooms/:id/join`)
- упрощенная локальная симуляция матча и действий на столе

## Что реализовано

- Базовый app shell и bottom navigation
- Маршруты: главная, лобби, создание игры, игровой стол, профиль
- Инициализация Telegram WebApp (`ready`, `expand`, цвета)
- Моки игровых комнат для UI-разработки

## Backend запуск

```bash
cd backend
go run ./cmd/api
```

## Docker окружение (Postgres + Redis + API)

```bash
docker compose -f docker/docker-compose.yml up --build
```

Миграции выполняются автоматически контейнером `migrate` перед стартом API.

## Test pipeline и race detector

```bash
make build
make test
make test-race
make pipeline
```

Docker race test (поднимает Postgres + Redis и гоняет `go test -race ./...`):

```bash
make docker-test
make docker-race
```

Health endpoint:

```bash
GET /health
```

Prometheus:

```bash
GET /metrics
```

pprof (локально на отдельном порту):

```bash
http://127.0.0.1:6060/debug/pprof/
```

Ключевые runtime-метрики:

- `ws_active_connections`
- `active_matches`
- `match_move_duration_seconds`
- `db_query_duration_seconds`
- `redis_latency_seconds`
- `settlement_total`

## Load test (k6)

Сценарий: `loadtest/ws-load.js` (200 WS соединений, join_room, 10 make_move, reconnect).

Пример запуска:

```bash
k6 run -e BASE_URL=http://localhost:8080 -e WS_URL=ws://localhost:8080/ws -e WS_TOKEN=<jwt> -e ROOM_ID=<room_id> loadtest/ws-load.js
```

Через Docker (без локального `k6`):

```bash
make load
```

## Что добавлено в MVP-core

- Go backend (`chi`, JWT auth, WebSocket, базовый game engine, wallet транзакции)
- SQL миграции (`backend/migrations/0001_init.up.sql`, `backend/migrations/0002_constraints.up.sql`)
- Docker compose окружение (`docker/docker-compose.yml`)
- Интеграция frontend с auth bootstrap и reconnect WS клиентом
