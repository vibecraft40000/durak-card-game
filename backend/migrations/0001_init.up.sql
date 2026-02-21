CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    telegram_id BIGINT UNIQUE NOT NULL,
    username TEXT NOT NULL DEFAULT '',
    referral_code TEXT UNIQUE NOT NULL,
    invited_by UUID NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TYPE transaction_type AS ENUM ('deposit', 'withdraw', 'bet_hold', 'win', 'commission');
CREATE TYPE transaction_status AS ENUM ('pending', 'confirmed', 'failed');

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    type transaction_type NOT NULL,
    amount NUMERIC(18,2) NOT NULL,
    status transaction_status NOT NULL,
    match_id UUID NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_transactions_non_zero_amount'
    ) THEN
        ALTER TABLE transactions
            ADD CONSTRAINT chk_transactions_non_zero_amount CHECK (amount <> 0);
    END IF;
END $$;

CREATE TYPE match_status AS ENUM ('waiting', 'active', 'finished');

CREATE TABLE IF NOT EXISTS matches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status match_status NOT NULL,
    stake NUMERIC(18,2) NOT NULL,
    mode TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS match_players (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_id UUID NOT NULL REFERENCES matches(id),
    user_id UUID NOT NULL REFERENCES users(id),
    position INT NOT NULL,
    result TEXT NULL
);

CREATE TABLE IF NOT EXISTS game_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    match_id UUID NOT NULL REFERENCES matches(id),
    state JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_matches_status ON matches(status);
CREATE INDEX IF NOT EXISTS idx_match_players_match_id ON match_players(match_id);
CREATE INDEX IF NOT EXISTS idx_game_history_match_id ON game_history(match_id);
CREATE INDEX IF NOT EXISTS idx_transactions_match_id ON transactions(match_id);

CREATE OR REPLACE VIEW user_balances AS
SELECT
    user_id,
    COALESCE(SUM(amount), 0)::NUMERIC(18,2) AS balance
FROM transactions
WHERE status = 'confirmed'
GROUP BY user_id;
