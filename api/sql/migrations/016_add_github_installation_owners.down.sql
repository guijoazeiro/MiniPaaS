DROP INDEX IF EXISTS github_installations_owner_user_id_idx;
ALTER TABLE github_installations DROP CONSTRAINT IF EXISTS github_installations_owner_user_id_fkey;
ALTER TABLE github_installations DROP COLUMN IF EXISTS owner_user_id;
