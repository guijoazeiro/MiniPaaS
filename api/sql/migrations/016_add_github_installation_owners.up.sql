ALTER TABLE github_installations
    ADD COLUMN owner_user_id UUID;

UPDATE github_installations
SET owner_user_id = (
    SELECT id FROM users ORDER BY created_at ASC, id ASC LIMIT 1
)
WHERE owner_user_id IS NULL;

ALTER TABLE github_installations
    ADD CONSTRAINT github_installations_owner_user_id_fkey
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX github_installations_owner_user_id_idx
    ON github_installations(owner_user_id, account_login, installation_id);
