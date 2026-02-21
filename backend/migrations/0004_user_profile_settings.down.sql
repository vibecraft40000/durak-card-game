ALTER TABLE users
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS photo_url,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name;
