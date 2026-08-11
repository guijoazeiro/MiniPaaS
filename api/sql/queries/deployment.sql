-- name: CreateDeployment :one
INSERT INTO deployments (app_id, image_tag)
VALUES (@app_id, @image_tag)
RETURNING *;

-- name: CreateGitDeployment :one
INSERT INTO deployments (app_id, image_tag, source_type, repository, branch)
VALUES (@app_id, @image_tag, 'git', @repository, @branch)
RETURNING *;

-- name: UpdateDeploymentGitMetadata :exec
UPDATE deployments
SET commit_sha = @commit_sha,
    commit_author = @commit_author,
    commit_message = @commit_message,
    branch = @branch
WHERE id = @id;

-- name: GetDeploymentByID :one
SELECT * FROM deployments WHERE id = @id;

-- name: GetActiveDeployment :one
SELECT * FROM deployments
WHERE app_id = @app_id AND status = 'running'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListDeploymentsByApp :many
SELECT * FROM deployments
WHERE app_id = @app_id
ORDER BY created_at DESC
LIMIT @lim;

-- name: UpdateDeploymentRunning :exec
UPDATE deployments
SET status       = 'running',
    container_id = @container_id,
    port         = @port,
    image_tag    = @image_tag,
    duration_ms  = @duration_ms,
    finished_at  = now()
WHERE id = @id;

-- name: UpdateDeploymentStatus :exec
UPDATE deployments
SET status = @status, finished_at = now()
WHERE id = @id;

-- name: ListRunningDeployments :many
SELECT * FROM deployments WHERE status = 'running';

-- name: ListDeploymentsForRetention :many
SELECT * FROM deployments
WHERE deployments.id IN (
    SELECT d.id FROM deployments d
    WHERE d.app_id = @app_id
    ORDER BY d.created_at DESC
    OFFSET @keep
)
AND status NOT IN ('running', 'pending', 'building')
ORDER BY created_at DESC;
