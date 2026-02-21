-- HoldBet (balance read)
EXPLAIN ANALYZE
SELECT COALESCE(SUM(amount), 0)
FROM transactions
WHERE user_id = '00000000-0000-0000-0000-000000000001'
  AND status = 'confirmed';

-- HoldBet (insert hold)
EXPLAIN ANALYZE
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'bet_hold', -10, 'confirmed', '00000000-0000-0000-0000-000000000010', NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING;

-- SettleWin (insert win)
EXPLAIN ANALYZE
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'win', 19.4, 'confirmed', '00000000-0000-0000-0000-000000000010', NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING;

-- SettleWin (insert commission)
EXPLAIN ANALYZE
INSERT INTO transactions (id, user_id, type, amount, status, match_id, created_at)
VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'commission', -0.6, 'confirmed', '00000000-0000-0000-0000-000000000010', NOW())
ON CONFLICT ON CONSTRAINT ux_transactions_match_user_type DO NOTHING;

-- ApplyMove persistence (snapshot insert)
EXPLAIN ANALYZE
INSERT INTO game_history (id, match_id, state, created_at)
VALUES (gen_random_uuid(), '00000000-0000-0000-0000-000000000010', '{"status":"playing"}'::jsonb, NOW());
