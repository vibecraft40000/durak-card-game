DROP TABLE IF EXISTS admin_audit_logs;

ALTER TABLE users
    DROP COLUMN IF EXISTS is_banned;

-- Enum value transaction_type.admin_adjust is intentionally kept to avoid
-- unsafe ALTER TYPE rewrites in down migrations.
