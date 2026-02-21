# Persistent Game State

## Where game_state lives

- **Redis**: `match:state:{matchID}` — main source of truth for in-flight games
- **Postgres**: `game_history` — snapshots on each move; `matches` — metadata (winner, reason, duration)
- **Memory**: None — no in-memory game state; all reads/writes go through Redis

## API restart

- **State preserved**: API restart does not lose game state. Redis runs out-of-process.
- **Recovery**: In-flight matches continue after restart; clients can reconnect via WS.

## Redis failure

- **State lost**: If Redis goes down, in-flight game state is lost.
- **Expected**: Current architecture assumes Redis availability; no automatic recovery from `game_history`.
- **Optional**: Last `game_history` snapshot could be used for abandoned-match recovery (not implemented).
