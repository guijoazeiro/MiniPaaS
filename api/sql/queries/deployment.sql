-- name: CreateDeployment :one
INSERT INTO deployments (app_id, image_tag)
VALUES (@app_id, @image_tag)
RETURNING *;

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
WHERE app_id = @app_id
ORDER BY created_at DESC
OFFSET @keep;
