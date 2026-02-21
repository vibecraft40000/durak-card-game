# Deadlock Protection Audit

## Reviewed transaction paths

### HoldBet
- Query balance (`SELECT SUM(...)`) + insert hold (`INSERT ... bet_hold`) in one serializable transaction.
- Potential conflicts:
  - concurrent hold attempts on same user balance view.
- Protection:
  - `SERIALIZABLE` isolation
  - retry on SQLSTATE `40001` and `40P01`

### SettleWin
- Win + commission inserts in one serializable transaction.
- Idempotency:
  - `ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING`
- Protection:
  - retries on serialization/deadlock failure

### ApplyMove
- No DB transaction in move hot path (Redis + per-match lock).
- Deadlock risk at DB level is low; contention handled by Redis + process mutex.

## Retry policy
- max retries: 3
- linear backoff: 50ms, 100ms, 150ms
- retry for:
  - `40001` (serialization_failure)
  - `40P01` (deadlock_detected)

## Recommendation
- keep lock ordering stable when adding new DB writes in move path.
- if move path starts writing multiple tables, enforce explicit table access order.
