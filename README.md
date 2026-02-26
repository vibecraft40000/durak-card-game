# Durak Online Mini App

Первый инкремент проекта Telegram Mini App под игру "Дурак Онлайн".

## Стек

- React + TypeScript
- Vite
- React Router
- FSD-like структура в `src/`
- Go backend (chi, JWT, WebSocket, Redis, Postgres)

## Дизайн-артефакты

- 3 визуальных направления: [docs/design/visual-directions.md](docs/design/visual-directions.md)
- Style guide (цвета, шрифты, состояния, адаптив): [docs/design/style-guide.md](docs/design/style-guide.md)

## Авторизация и доступ

- **Мини-аппа рассчитана на запуск только из Telegram**: мобильный клиент, Telegram Desktop, Telegram Web (web.telegram.org). При открытии из Telegram приложение получает `initData`, по которому бэкенд выдаёт JWT (Bearer).
- **На прод-домене** (`https://durakonline.duckdns.org`) вход без Telegram **заблокирован**: при отсутствии валидного `initData` dev-авторизация не используется, токены не выдаются, все запросы к `/api/*` возвращают 401. Прямое открытие URL в обычном браузере приводит к экрану «Ошибка загрузки» — это ожидаемое поведение.
- **Telegram Web и Telegram Desktop** передают `initData` через query-параметр `tgWebAppData` в URL. Фронт читает как `window.Telegram.WebApp.initData`, так и `tgWebAppData`/`tg_data` из `location.search`, чтобы авторизация работала на всех клиентах.
- **Локально** (например `localhost:5173`) при пустом `initData` можно использовать dev-авторизацию (`buildDevInitData` с `hash=dev`), если в `.env` задано `ALLOW_DEV_TELEGRAM_AUTH=true` и на бэке включена поддержка dev-auth.

## Конфигурация (.env)

Основные переменные для авторизации и прод-домена:

| Переменная | Описание |
|------------|----------|
| `TELEGRAM_BOT_TOKEN` | Токен бота из BotFather (для проверки подписи initData на бэке). |
| `TELEGRAM_INIT_MAX_AGE` | Максимальный возраст initData (например `24h` или `720h` для 30 дней). Увеличение снижает риск 401 в Telegram Web/Desktop при долгой сессии. |
| `ALLOW_DEV_TELEGRAM_AUTH` | Разрешить `hash=dev` на бэкенде (только для разработки). |
| `AUTH_REPLAY_TTL` | TTL для защиты от повторного использования одного initData (replay). |
| `VITE_FORCE_DEV_AUTH` | На фронте всегда использовать dev initData (только для локальной разработки). |
| `VITE_API_BASE_URL` / `VITE_WS_URL` | Для локального dev; на проде при открытии с durakonline.duckdns.org используются same-origin API и `/ws`. |

Replay-проверка initData на бэкенде выполняется только при `ENV=production`; в остальных окружениях один и тот же initData может быть использован повторно (удобно для беты и разных устройств).

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

## Деплой на durakonline.duckdns.org

Скрипт собирает фронт, копирует артефакты и бэкенд на сервер, поднимает стек через Docker (Caddy + API + Postgres + Redis):

```bash
# из корня проекта; пароль можно передать через DEPLOY_PW или .deploy-pw
python scripts/deploy_duckdns.py
```

После деплоя фронт и API доступны по `https://durakonline.duckdns.org`. Мини-аппу нужно открывать из Telegram (бот, кнопка Web App).

## Что реализовано

- Базовый app shell и bottom navigation
- Маршруты: главная (Игры), лобби, создание игры, игровой стол, профиль (ввод/вывод, пополнение Crypto Bot, друзья, история)
- Инициализация Telegram WebApp (`ready`, `expand`, цвета, safe area)
- Авторизация через Telegram: `initData` (в т.ч. из `tgWebAppData` в URL для Web/Desktop), JWT в заголовке `Authorization`
- Блокировка входа без Telegram на прод-домене (нет dev-auth при пустом initData)
- Моки игровых комнат для UI-разработки (`VITE_USE_MOCK_API=true`)
- Игровой стол: роли (атакующий/защитник), таймер хода, финальный экран с местами и выплатами, обработка ошибок интентов (баннер, редирект при kick)
- Список комнат с фильтрами (колода, игроки, ставка), синхронизация с URL и localStorage, автообновление списка каждые 5 секунд

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
- Авторизация Telegram: проверка подписи `initData` (HMAC-SHA256), опциональная защита от replay (при `ENV=production`), поддержка `TELEGRAM_INIT_MAX_AGE` и dev-auth для разработки
- SQL миграции (`backend/migrations/0001_init.up.sql`, `backend/migrations/0002_constraints.up.sql`)
- Docker compose окружение (`docker/docker-compose.yml`, `docker/docker-compose.caddy.yml` для Caddy + API)
- Интеграция frontend с auth bootstrap и reconnect WS клиентом
- На проде: API и WS запросы идут на same-origin (без захардкоженного localhost в бандле), initData читается из `WebApp.initData` и из query `tgWebAppData`/`tg_data` для Telegram Web и Desktop
