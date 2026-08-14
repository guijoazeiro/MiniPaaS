DROP INDEX IF EXISTS apps_owner_user_id_idx;
ALTER TABLE apps DROP CONSTRAINT IF EXISTS apps_owner_user_id_fkey;
ALTER TABLE apps DROP COLUMN IF EXISTS owner_user_id;
