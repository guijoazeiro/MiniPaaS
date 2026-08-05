-- name: CreateApp :one
INSERT INTO apps (name)
VALUES (@name)
RETURNING *;

-- name: GetAppByName :one
SELECT * FROM apps
WHERE name = @name
LIMIT 1;

-- name: GetAppByID :one
SELECT * FROM apps
WHERE id = @id
LIMIT 1;

-- name: ListApps :many
SELECT * FROM apps
ORDER BY created_at DESC;

-- name: UpdateAppStatus :exec
UPDATE apps
SET status = @status, updated_at = now()
WHERE id = @id;

-- name: DeleteApp :exec
DELETE FROM apps WHERE id = @id;
