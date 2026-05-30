CREATE TABLE IF NOT EXISTS platform_fees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL UNIQUE,
    gross_pot NUMERIC(18,2) NOT NULL CHECK (gross_pot > 0),
    commission_bps INTEGER NOT NULL CHECK (commission_bps >= 0),
    commission_amount NUMERIC(18,2) NOT NULL CHECK (commission_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_platform_fees_created_at ON platform_fees(created_at DESC);
