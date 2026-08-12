-- name: UpsertGitHubInstallation :one
INSERT INTO github_installations (installation_id, account_login, account_type, repository_selection)
VALUES (@installation_id, @account_login, @account_type, @repository_selection)
ON CONFLICT (installation_id) DO UPDATE SET
    account_login = EXCLUDED.account_login,
    account_type = EXCLUDED.account_type,
    repository_selection = EXCLUDED.repository_selection,
    updated_at = now()
RETURNING *;

-- name: GetGitHubInstallation :one
SELECT * FROM github_installations WHERE installation_id = @installation_id;

-- name: ListGitHubInstallations :many
SELECT * FROM github_installations ORDER BY account_login, installation_id;
