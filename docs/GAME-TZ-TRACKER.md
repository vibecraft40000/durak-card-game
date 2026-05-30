# Game TZ Tracker (No Deposit/Withdraw Scope)

Last updated: 2026-02-28

## Scope

In scope:
- matchmaking/rooms
- game engine (durak rules, phases, timers, reconnect)
- websocket protocol and client/server sync
- game UX-related backend/frontend bugs

Out of scope for this tracker:
- deposit flow
- withdraw flow
- payment provider integrations

## Status Matrix (Game-Only)

| Area | Status | Notes / Evidence |
|---|---|---|
| Rooms: create/join/ready/start | done | Implemented via `/api/rooms/*` and WS room events (`backend/cmd/api/main.go`, `backend/internal/rooms/service.go`, `backend/internal/ws/handler.go`). |
| Stake confirmation before start | done | `awaiting_stake_confirm`, confirm endpoint + WS event, timeout cancellation implemented (`backend/internal/rooms/service.go`, `backend/internal/scheduler/timer.go`). |
| Core FSM: attack/defend/take/pass | done | Implemented in engine (`backend/internal/games/engine/rules.go`). |
| Translate (perevodnoy) | done | `ActionTranslate` + UI button and WS mapping exist (`backend/internal/games/engine/rules.go`, `src/pages/game-table/GameTablePage.tsx`). |
| Throw-in (podkidka by non-attacker players) | done | Backend supports `throw`/`throw_card` with rank + round-limit validation and safe timing in `attack` phase; frontend drag/button flow aligned (`backend/internal/games/engine/rules.go`, `backend/internal/ws/handler.go`, `src/entities/game/lib/canPlayCard.ts`). |
| Turn timeout auto-actions | done | Auto take/pass/attack first card implemented (`backend/internal/games/service.go`, scheduler loop). |
| Reconnect in active game | done | Reconnect/sync supports deterministic `state_sync` with versioned replay fallback (`replay` or `snapshot`) plus final authoritative `game_state`; replay durability is backed by Redis ring-buffer so cold WS-handler restart still replays recent moves; client sends `lastKnownVersion` and applies version-ordered activity buffering to avoid live+replay out-of-order logs (`backend/internal/ws/handler.go`, `src/shared/api/ws/socket.ts`, `src/processes/joinGame.process.ts`). |
| AFK handling with bot substitution/skip policy flag | done | Added configurable policy via `DISCONNECT_POLICY`: `abandon` (legacy) or `bot_takeover` (keeps match active, enables auto/bot handling, emits `player_afk_bot_takeover`) (`backend/internal/games/service.go`, `backend/internal/scheduler/timer.go`, `backend/pkg/config/config.go`). |
| Shuler: report flow | done | Dedicated `shuler_play` + `shuler_report` flow implemented; report permanently disables further shuler actions (`backend/internal/games/engine/rules.go`, `backend/internal/ws/handler.go`, `src/pages/game-table/GameTablePage.tsx`). |
| Shuler: 3-second report window and full lifecycle | done | Engine opens bounded `ShulerWindowUntil = now + 3s`; report allowed only inside window; WS DTO exposes `isWindowOpen` and `windowEndsAt`; frontend applies local countdown/expiry guard for reconnect-lag UX (`backend/internal/games/engine/rules.go`, `backend/internal/ws/handler.go`, `src/pages/game-table/GameTablePage.tsx`). |
| WS event envelope (type+payload+correlation-id+locale) | done | `ServerEvent` now includes `correlationId` and optional `locale`; metadata is auto-populated for send/broadcast paths and covered by integration test (`backend/internal/ws/protocol.go`, `backend/internal/ws/meta.go`, `backend/internal/ws/handler.go`, `backend/internal/integration/ws_contract_test.go`). |
| Versioning optimistic lock | done | `expectedVersion` checked server-side, mismatch event emitted and client reconciles (`backend/internal/games/service.go`, `backend/internal/ws/handler.go`, `src/processes/joinGame.process.ts`). |
| Game tests coverage | done | Engine tests cover translate, shuler-play/report window semantics, throw-in flows, 4-player multi-throw order, throw round-limit, and take-after-throw rotation; service/integration tests cover timeout auto-pass/auto-take decisions, reconnect+shuler-window boundary, stale `expectedVersion` retry race, websocket contracts for `INVALID_ACTION`/`version_mismatch`, and reconnect replay/diff ordering + duplicate suppression (`backend/internal/games/engine/rules_extended_test.go`, `backend/internal/games/service_timeout_test.go`, `backend/internal/integration/production_path_test.go`, `backend/internal/integration/ws_contract_test.go`). |

## Recently Fixed (2026-02-28)

1. `confirmStake` no longer maps to `pass_turn`; it sends `confirm_stake`.
2. Dead `intent` WS event branch removed from client type contract.
3. `throwIn` now maps to dedicated wire action `throw_card`; backend supports `ActionThrow`.
4. Added `shuler_play` action end-to-end (engine + WS normalize + frontend intent/button/drag flow).
5. Added bounded 3-second shuler report window (`ShulerWindowUntil`, `windowEndsAt`, `isWindowOpen`) and tests.
6. Fixed throw-in phase bug: throw now executes in `attack` phase (not `defend`), preventing table pair corruption; added regression tests.
7. Expanded 4-player throw-in edge tests (multi-throw order, round-limit denial, take rotation) and timeout decision tests (`auto_take` in defend, `auto_pass` in attacker window).
8. Improved shuler report UX: local countdown from `windowEndsAt`, client-side late-click guard, and mapped user-facing errors for `INVALID_ACTION/INVALID_TURN/INVALID_CARD`.
9. Added integration tests for reconnect + shuler report window boundary (report allowed before expiry, denied after expiry).
10. Added integration race test for stale `expectedVersion` after timeout in throw window, including retry behavior with fresh version.
11. Added websocket integration contract tests: late `shuler_report` returns `INVALID_ACTION`; stale move after timeout emits `game_state` then `version_mismatch` with action/card/actionId echo.
12. Implemented deterministic reconnect sync contract: `state_sync` (`replay|snapshot|noop`) + versioned `move_applied` replay + final `game_state`; added integration test for replay ordering and no duplicate replay on fresh `sync_request`.
13. Added Redis-backed replay durability (`ws:replay:<matchId>`) with WS contract test for replay after handler restart (local in-memory buffer empty).
14. Added client-side version-ordered activity buffering: `move_applied` events are buffered and flushed only when confirmed by current `game_state.version`, preventing out-of-order activity entries during replay+live interleaving.
15. Added stale move-activity GC policy on client: move log entries older than `currentVersion - 80` are pruned (system entries preserved), reducing long-session drift/noise.
16. Added WS envelope metadata (`correlationId`, optional `locale`) with auto-population in send/broadcast paths and integration coverage.
17. Added configurable disconnect policy (`DISCONNECT_POLICY=abandon|bot_takeover`): bot-takeover mode no longer force-finishes after grace and emits `player_afk_bot_takeover`.
18. Added client replay observability via `VITE_WS_REPLAY_DEBUG` counters (and optional verbose logs) + `match_finished` activity reset hook (`VITE_MATCH_ACTIVITY_RESET_ON_FINISH`).
19. Improved reconnect fallback semantics: if full replay range is unavailable, server sends `state_sync` in `snapshot` mode with a short contiguous tail replay (`replayFromVersion` + `replayCount`) before final authoritative `game_state`.
20. Added P2 tests for simultaneous finish groups and translated-round timeout/reconnect race with stale-version retry behavior (`backend/internal/games/engine/rules_extended_test.go`, `backend/internal/integration/production_path_test.go`).
21. Added deterministic `state_diff` reconnect event for snapshot syncs (including no-replay snapshots) and frontend merge handling before authoritative `game_state`.
22. Added replay observability tuning knobs (`VITE_WS_REPLAY_WARN_*`) and a dev-only WS packet-disorder harness (`VITE_WS_PACKET_DISORDER*`) to reproducibly stress replay buffering and duplicate suppression on client.
23. Added optional P1 reconnect optimization behind `WS_SYNC_DIFF_SKIP_FINAL_STATE`: server can skip final full `game_state` only for capability-advertising clients on same `matchId`; added sync payload-size metrics (`ws_sync_payload_bytes`) and skip counter (`ws_sync_final_state_skipped_total`) for staging validation.
24. Hardened `state_diff.patch` to carry full `game_state` DTO shape (not selective fields), reducing stale-field risk when final `game_state` is skipped in optimized reconnect mode.

## Prioritized Backlog (Game-Only, No Payments)

P0 (start here):
1. Run staging session and collect replay-debug snapshots/warnings with production traffic profile; tune `VITE_WS_REPLAY_WARN_*` defaults if noisy (local packet-disorder harness is ready for pre-checks).

P1:
1. Validate in staging (with/without `WS_SYNC_DIFF_SKIP_FINAL_STATE`) and tune rollout: inspect `ws_sync_payload_bytes` + `ws_sync_final_state_skipped_total`, client replay-debug snapshots, and confirm no reconnect regressions.

P2:
1. Expand engine/integration tests for 3/4-player edge-cases: throw-ins, simultaneous finish groups, timeout+reconnect races on translated rounds. (done)
