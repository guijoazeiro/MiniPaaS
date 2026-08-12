-- name: UpsertGitSource :one
INSERT INTO app_git_sources (app_id, repository, branch, build_context, dockerfile_path, access_mode, github_installation_id, github_repository_id, private)
VALUES (@app_id, @repository, @branch, @build_context, @dockerfile_path, @access_mode, @github_installation_id, @github_repository_id, @private)
ON CONFLICT (app_id) DO UPDATE SET
    repository = EXCLUDED.repository,
    branch = EXCLUDED.branch,
    build_context = EXCLUDED.build_context,
    dockerfile_path = EXCLUDED.dockerfile_path,
    access_mode = EXCLUDED.access_mode,
    github_installation_id = EXCLUDED.github_installation_id,
    github_repository_id = EXCLUDED.github_repository_id,
    private = EXCLUDED.private,
    auto_deploy = CASE WHEN EXCLUDED.access_mode = 'public' THEN false ELSE app_git_sources.auto_deploy END,
    updated_at = now()
RETURNING *;

-- name: GetGitSource :one
SELECT * FROM app_git_sources WHERE app_id = @app_id;

-- name: DeleteGitSource :exec
DELETE FROM app_git_sources WHERE app_id = @app_id;

-- name: SetGitSourceAutoDeploy :one
UPDATE app_git_sources
SET auto_deploy = @auto_deploy, updated_at = now()
WHERE app_id = @app_id
RETURNING *;

-- name: ListAutoDeployGitSourcesByRepository :many
SELECT app_git_sources.*
FROM app_git_sources
WHERE github_repository_id = @github_repository_id
  AND access_mode = 'github_app'
  AND auto_deploy = true
ORDER BY app_id;
