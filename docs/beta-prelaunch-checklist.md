# Beta Prelaunch Checklist

## Scope

- Deposits: CryptoPay only.
- WalletPay: disabled for the first beta (`WALLET_PAY_ENABLED=false`).
- Withdrawals: disabled server-side (`WITHDRAWALS_ENABLED=false`).
- Admin money-changing actions: disabled server-side.

## Required Environment

- `ENV=staging` or `ENV=production`
- `POSTGRES_URL`
- `REDIS_URL`
- `ALLOWED_ORIGIN`
- `JWT_SECRET`
- `TELEGRAM_BOT_TOKEN`
- `CRYPTO_PAY_API_TOKEN`
- `CRYPTO_PAY_TESTNET`
- `ADMIN_SECRET`
- `ALLOW_DEV_TELEGRAM_AUTH=false`
- `DISABLE_MONEY=false`
- `WITHDRAWALS_ENABLED=false`
- `WALLET_PAY_ENABLED=false`

## Services That Must Be Up

- PostgreSQL
- Redis
- Backend API
- Frontend Mini App build/preview or production static host
- Telegram bot only if the deployed Mini App entrypoint depends on it

## Required Feature Flags

- Enabled:
  - Telegram WebApp auth validation
  - CryptoPay deposit route
  - match/room/game flow
- Disabled:
  - WalletPay create payment route
  - WalletPay webhook route
  - withdraw route execution
  - admin balance-adjust / admin stake-confirm money actions
  - dev auth in any beta-like environment

## Mandatory Smoke Tests Before Launch

1. `/health` and `/ready` return OK.
2. `/api/config` returns `depositProvider=cryptopay`, `walletPayEnabled=false`, `withdrawalsEnabled=false`.
3. Two-session login works with real Telegram `initData`.
4. Room create/join/ready/confirm/start works from two independent sessions.
5. One invalid move is rejected server-side.
6. One full match finishes with exact pot conservation.
7. Deposit invoice creation works through CryptoPay.
8. CryptoPay webhook rejects invalid signatures.
9. Replaying the same valid CryptoPay webhook does not duplicate credit.
10. `POST /api/withdraw/create` returns forbidden.
11. Admin `balance-adjust` and admin `stake/confirm` return forbidden.

## Known First-Beta Limitations

- WalletPay is out of scope.
- Withdrawals are intentionally disabled.
- Real paid CryptoPay invoice settlement still needs one controlled manual smoke before launch day.
- Previously exposed secrets must be rotated outside code before launch.

## Controlled CryptoPay Manual Smoke

Use the operator runbook in [cryptopay-manual-smoke.md](/c:/Users/GANG/Desktop/cursor%20project/durakonline/docs/cryptopay-manual-smoke.md).
