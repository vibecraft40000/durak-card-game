# Админ-панель (Flask)

Веб-админка для Дурак Онлайн: авторизация, дашборд со статистикой, список пользователей из API бэкенда.

## Стек

- **Flask** — веб-приложение
- **Flask-Login** — аутентификация
- **Flask-SQLAlchemy** — модель админа (SQLite)
- **REST API** — интеграция с Go бэкендом (`/admin/stats`, `/admin/users`)

## Установка

```bash
cd admin_panel
pip install -r requirements.txt
cp .env.example .env
# Отредактируйте .env: FLASK_SECRET_KEY, ADMIN_PASSWORD, API_BASE_URL, ADMIN_SECRET
```

## Запуск

```bash
python run.py
```

Откройте в браузере: http://localhost:5000  
Логин и пароль — из `ADMIN_USERNAME` и `ADMIN_PASSWORD` (первый админ создаётся при первом запуске).

## Переменные окружения

| Переменная | Описание |
|------------|----------|
| `FLASK_SECRET_KEY` | Секрет для сессий (обязательно сменить в проде) |
| `ADMIN_USERNAME` | Логин первого админа (по умолчанию `admin`) |
| `ADMIN_PASSWORD` | Пароль первого админа (при первом запуске создаётся учётка) |
| `API_BASE_URL` | URL Go бэкенда (например `https://durakonline.duckdns.org`) |
| `ADMIN_SECRET` | Секрет для запросов к `/admin/stats` и `/admin/users` (тот же, что в backend) |

В бэкенде (Go) должны быть заданы `ADMIN_SECRET` и включены маршруты `/admin/stats` и `/admin/users`.

## Функции

- **Вход** — логин/пароль, декоратор `@admin_required` на всех страницах админки.
- **Дашборд** — число пользователей в БД (из API), статус подключения к API.
- **Пользователи** — таблица с пагинацией (данные из GET `/admin/users`).
- **Настройки** — отображение конфигурации (API_BASE_URL, наличие ADMIN_SECRET).
