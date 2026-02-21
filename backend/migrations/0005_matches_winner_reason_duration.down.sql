ALTER TABLE matches
    DROP COLUMN IF EXISTS winner,
    DROP COLUMN IF EXISTS finish_reason,
    DROP COLUMN IF EXISTS duration_seconds;

DROP TYPE IF EXISTS match_finish_reason;
