CREATE INDEX IF NOT EXISTS idx_transactions_user_status_created
  ON transactions (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_transactions_match_user_type
  ON transactions (match_id, user_id, type);

CREATE INDEX IF NOT EXISTS idx_game_history_match_created
  ON game_history (match_id, created_at DESC);
