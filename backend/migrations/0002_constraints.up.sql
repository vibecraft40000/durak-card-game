ALTER TABLE transactions
    ADD CONSTRAINT ux_transactions_match_user_type
    UNIQUE (match_id, user_id, type);

ALTER TABLE match_players
    ADD CONSTRAINT ux_match_players_match_user
    UNIQUE (match_id, user_id);

ALTER TABLE match_players
    ALTER COLUMN result SET DEFAULT 'lose';
