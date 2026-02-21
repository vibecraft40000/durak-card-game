ALTER TABLE match_players DROP CONSTRAINT IF EXISTS ux_match_players_match_user;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS ux_transactions_match_user_type;
