DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'transaction_type'
          AND e.enumlabel = 'bet_hold_release'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE 'bet_hold_release';
    END IF;
END $$;
