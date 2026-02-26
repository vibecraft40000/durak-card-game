# Админ-панель (Flask)

Панель для управления Telegram Mini App «Дурак Онлайн».

## Что умеет

- Авторизация администратора (Flask-Login).
- Дашборд (проверка подключения к backend API, счетчик пользователей).
- Список пользователей с действиями:
  - `ban / unban`
  - ручная корректировка баланса (`balance-adjust`).
- Расширенные логи:
  - операции из `transactions`
  - admin audit события
  - результаты матчей
  - последние выводы.

## Требования

- Python 3.11+
- Доступ к Go backend с включенным `ADMIN_SECRET`

## Установка

```bash
cd admin_panel
pip install -r requirements.txt
```

## Конфигурация (`.env`)

- `FLASK_SECRET_KEY` — секрет сессий Flask.
- `ADMIN_USERNAME` — логин администратора.
- `ADMIN_PASSWORD` — пароль администратора (первичный админ создается при первом запуске).
- `API_BASE_URL` — адрес backend API.
- `ADMIN_SECRET` — тот же секрет, что и в backend (`X-Admin-Secret`).

## Запуск

```bash
python run.py
```

По умолчанию: `http://localhost:5000`.

## Используемые backend endpoints

- `GET /admin/stats`
- `GET /admin/users`
- `POST /admin/users/{id}/ban`
- `POST /admin/users/{id}/unban`
- `POST /admin/users/{id}/balance-adjust`
- `GET /admin/logs`
- `GET /admin/withdrawals`
