CREATE TABLE IF NOT EXISTS game_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL REFERENCES matches(id),
    user_id UUID NOT NULL REFERENCES users(id),
    stake NUMERIC(18,2) NOT NULL,
    payout NUMERIC(18,2) NOT NULL,
    profit NUMERIC(18,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_game_results_user_id ON game_results(user_id);
CREATE INDEX idx_game_results_match_id ON game_results(match_id);
CREATE INDEX idx_game_results_created_at ON game_results(created_at DESC);
