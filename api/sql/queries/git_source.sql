-- name: UpsertGitSource :one
INSERT INTO app_git_sources (app_id, repository, branch, build_context, dockerfile_path)
VALUES (@app_id, @repository, @branch, @build_context, @dockerfile_path)
ON CONFLICT (app_id) DO UPDATE SET
    repository = EXCLUDED.repository,
    branch = EXCLUDED.branch,
    build_context = EXCLUDED.build_context,
    dockerfile_path = EXCLUDED.dockerfile_path,
    updated_at = now()
RETURNING *;

-- name: GetGitSource :one
SELECT * FROM app_git_sources WHERE app_id = @app_id;

-- name: DeleteGitSource :exec
DELETE FROM app_git_sources WHERE app_id = @app_id;
