# Security Checklist

## Telegram initData validation (Production)

- [ ] **ALLOW_DEV_TELEGRAM_AUTH=false** in production
- [ ] **TELEGRAM_BOT_TOKEN** — real bot token from BotFather (not `dev-bot-token`)
- [ ] **InitDataMaxAge** (default 24h) — consider reducing for stricter auth
- [ ] Replay protection: initData hash is marked used via `MarkInitDataHashUsed` (Redis)

Validation is implemented in `backend/internal/auth/telegram.go` (HMAC-SHA256, auth_date, user payload).
