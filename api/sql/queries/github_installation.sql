-- name: UpsertGitHubInstallation :one
INSERT INTO github_installations (installation_id, account_login, account_type, repository_selection, owner_user_id)
VALUES (@installation_id, @account_login, @account_type, @repository_selection, @owner_user_id)
ON CONFLICT (installation_id) DO UPDATE SET
    account_login = EXCLUDED.account_login,
    account_type = EXCLUDED.account_type,
    repository_selection = EXCLUDED.repository_selection,
    owner_user_id = EXCLUDED.owner_user_id,
    updated_at = now()
WHERE github_installations.owner_user_id IS NULL
   OR github_installations.owner_user_id = EXCLUDED.owner_user_id
RETURNING *;

-- name: GetGitHubInstallation :one
SELECT * FROM github_installations WHERE installation_id = @installation_id;

-- name: GetGitHubInstallationForUser :one
SELECT * FROM github_installations
WHERE installation_id = @installation_id AND owner_user_id = @owner_user_id;

-- name: ListGitHubInstallations :many
SELECT * FROM github_installations ORDER BY account_login, installation_id;

-- name: ListGitHubInstallationsForUser :many
SELECT * FROM github_installations
WHERE owner_user_id = @owner_user_id
ORDER BY account_login, installation_id;
