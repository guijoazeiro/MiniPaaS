ALTER TABLE apps ADD COLUMN owner_user_id UUID;

UPDATE apps
SET owner_user_id = (
    SELECT id FROM users ORDER BY created_at ASC, id ASC LIMIT 1
)
WHERE owner_user_id IS NULL;

ALTER TABLE apps
    ADD CONSTRAINT apps_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX apps_owner_user_id_idx ON apps(owner_user_id, created_at DESC);
