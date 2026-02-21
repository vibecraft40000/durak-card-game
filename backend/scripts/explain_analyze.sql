-- HoldBet: fast balance read under transactional scope.
EXPLAIN (ANALYZE, BUFFERS)
SELECT COALESCE(SUM(amount), 0)
FROM transactions
WHERE user_id = '00000000-0000-0000-0000-000000000001'
  AND status = 'confirmed';

-- SettleWin idempotency path: detect duplicate by (match_id, user_id, type).
EXPLAIN (ANALYZE, BUFFERS)
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'win', 1, 'confirmed', gen_random_uuid(), NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING;

-- ApplyMove persistence path: append state snapshot to history.
EXPLAIN (ANALYZE, BUFFERS)
INSERT INTO game_history (match_id, state, created_at)
VALUES (gen_random_uuid(), '{"status":"playing"}'::jsonb, NOW());
