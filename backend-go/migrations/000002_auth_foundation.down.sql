DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS sessions;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS created_at;
ALTER TABLE users DROP COLUMN IF EXISTS driver_license;
ALTER TABLE users DROP COLUMN IF EXISTS is_active;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'users_id_unique'
    ) THEN
        ALTER TABLE users DROP CONSTRAINT users_id_unique;
    END IF;
END $$;
ALTER TABLE users DROP COLUMN IF EXISTS id;
