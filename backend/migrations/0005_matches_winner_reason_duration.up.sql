-- Add winner, finish_reason, duration_seconds to matches for game_history analytics
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'match_finish_reason') THEN
        CREATE TYPE match_finish_reason AS ENUM ('normal', 'abandon', 'disconnect_timeout');
    END IF;
END $$;

ALTER TABLE matches
    ADD COLUMN IF NOT EXISTS winner TEXT NULL,
    ADD COLUMN IF NOT EXISTS finish_reason match_finish_reason NULL,
    ADD COLUMN IF NOT EXISTS duration_seconds INT NULL;
