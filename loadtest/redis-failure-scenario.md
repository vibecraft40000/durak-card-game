# Redis Failure Scenario

## Goal
Validate graceful degradation when Redis becomes unavailable during an active match.

## Steps
1. Start stack:
   - `docker compose -f docker/docker-compose.yml up -d postgres redis migrate api api2 lb`
2. Create two players, fund balances, create room, start match.
3. Open active websocket sessions and send `make_move`.
4. Stop Redis during active traffic:
   - `docker stop docker-redis-1`
5. Continue sending `make_move` for 30-60 seconds.
6. Check behavior:
   - API keeps process alive (no panic/crash).
   - Clients receive `error` events instead of hanging forever.
   - `/live` stays `ok`, `/ready` becomes degraded.
7. Restart Redis and validate reconnect + state sync:
   - `docker start docker-redis-1`
   - client sends `reconnect`, receives full snapshot.

## Expected Result
- No process crash.
- No duplicated settlement.
- Match operations degrade with controlled errors while Redis is down.
