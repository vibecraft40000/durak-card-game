# Economy Integrity Spec

## Settlement Flow

1. Game finishes → `StatusFinished`, `WinnerPlayer` set
2. `SettleIfFinished` called (from make_move or disconnect timeout handler)
3. Redis `SETNX payout:{matchID}` — single settlement per match
4. If SETNX ok: `wallet.SettleWin` (PostgreSQL, ON CONFLICT DO NOTHING)
5. `game_results.Insert` for audit
6. `payout:result:{matchID}` cached in Redis

## Idempotency

| Layer | Mechanism |
|-------|-----------|
| Settlement | Redis SETNX `payout:{matchID}` |
| Wallet | `ON CONFLICT (match_id, user_id, type) DO NOTHING` |
| Transactions | Unique constraint on match+user+type |

## Crash Scenarios

### Crash after SETNX, before wallet.SettleWin

- SETNX key held; lock released on Redis TTL (30 min)
- Another pod/retry: SETNX fails, tries `GET payout:result:{matchID}` → may be empty
- Fallback: return computed payouts without `settlementId`; client shows amounts but no DB credit
- **Mitigation:** Retry settlement on next match_finished broadcast; first successful wallet write wins

### Crash after wallet.SettleWin, before payout:result cache

- Wallet already credited (idempotent)
- `payout:result` may be empty; fallback payouts returned
- Client receives match_finished with payouts; balance already updated by wallet
- No double-credit: ON CONFLICT prevents duplicate inserts

### Crash during game, before StatusFinished

- Redis state may be lost (if Redis down)
- No settlement attempted; game state not final
- Recovery: optional restore from game_history (not implemented)

## No Rollback

Between `game_finished` and settlement there is no rollback. State is final. Settlement retries are idempotent.
