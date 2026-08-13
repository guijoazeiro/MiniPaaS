-- name: CreateDeployment :one
INSERT INTO deployments (app_id, image_tag)
VALUES (@app_id, @image_tag)
RETURNING *;

-- name: CreateGitDeployment :one
INSERT INTO deployments (app_id, image_tag, source_type, repository, branch)
VALUES (@app_id, @image_tag, 'git', @repository, @branch)
RETURNING *;

-- name: CreateTriggeredGitDeployment :one
INSERT INTO deployments (app_id, image_tag, source_type, repository, branch, trigger_type, github_delivery_id)
VALUES (@app_id, @image_tag, 'git', @repository, @branch, @trigger_type, @github_delivery_id)
RETURNING *;

-- name: CreateRetryGitDeployment :one
INSERT INTO deployments (app_id, image_tag, source_type, repository, branch, attempt, retry_of)
VALUES (@app_id, @image_tag, 'git', @repository, @branch, @attempt, @retry_of)
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

-- name: ListDeployments :many
SELECT d.*, a.name AS app_name
FROM deployments d
JOIN apps a ON a.id = d.app_id
WHERE (sqlc.narg('app_name')::text IS NULL OR a.name = sqlc.narg('app_name'))
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status'))
ORDER BY d.created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountDeployments :one
SELECT COUNT(*)
FROM deployments d
JOIN apps a ON a.id = d.app_id
WHERE (sqlc.narg('app_name')::text IS NULL OR a.name = sqlc.narg('app_name'))
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status'));

-- name: UpdateDeploymentRunning :exec
UPDATE deployments
SET status       = 'running',
    container_id = @container_id,
    port         = @port,
    image_tag    = @image_tag,
    duration_ms  = @duration_ms,
    finished_at  = now()
WHERE id = @id;

-- name: UpdateDeploymentCandidate :exec
UPDATE deployments
SET candidate_container_id = @candidate_container_id,
    candidate_port = @candidate_port
WHERE id = @id;

-- name: PromoteDeploymentCandidate :exec
UPDATE deployments
SET status = 'running',
    container_id = @container_id,
    port = @port,
    image_tag = @image_tag,
    duration_ms = @duration_ms,
    candidate_container_id = NULL,
    candidate_port = NULL,
    finished_at = now()
WHERE id = @id;

-- name: ClearDeploymentCandidate :exec
UPDATE deployments
SET candidate_container_id = NULL,
    candidate_port = NULL
WHERE id = @id;

-- name: ListDeploymentCandidates :many
SELECT * FROM deployments
WHERE candidate_container_id IS NOT NULL
  AND status <> 'running'
ORDER BY created_at ASC;

-- name: UpdateDeploymentStatus :exec
UPDATE deployments
SET status = @status, finished_at = now()
WHERE id = @id;

-- name: RequestDeploymentCancel :one
UPDATE deployments
SET status = 'cancel_requested', cancel_requested = true
WHERE id = @id AND status IN ('pending', 'building', 'cancel_requested')
RETURNING *;

-- name: MarkDeploymentCancelled :exec
UPDATE deployments
SET status = 'cancelled', cancel_requested = true, finished_at = now()
WHERE id = @id AND status IN ('pending', 'building', 'cancel_requested');

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

-- name: CreateDeploymentLog :one
INSERT INTO deployment_logs (deployment_id, stage, stream, message)
VALUES (@deployment_id, @stage, @stream, @message)
RETURNING *;

-- name: ListDeploymentLogs :many
SELECT * FROM deployment_logs
WHERE deployment_id = @deployment_id
  AND id > @after_id
ORDER BY id ASC
LIMIT @lim;

-- name: ListHealthCheckLogsByApp :many
SELECT l.*
FROM deployment_logs l
JOIN deployments d ON d.id = l.deployment_id
WHERE d.app_id = @app_id
  AND l.stage = 'health_check'
ORDER BY l.created_at DESC
LIMIT @lim;
