-- WalletPay integration: payments table for invoice tracking and idempotent crediting
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_status') THEN
        CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'expired', 'refunded');
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    external_id TEXT NOT NULL UNIQUE,
    wallet_order_id TEXT,
    amount_usd NUMERIC(18,2) NOT NULL,
    currency_code TEXT NOT NULL DEFAULT 'USD',
    paid_amount NUMERIC(18,8),
    paid_currency TEXT,
    status payment_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_webhook JSONB,
    CONSTRAINT amount_positive CHECK (amount_usd > 0)
);

CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_external_id ON payments(external_id);
