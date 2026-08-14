-- name: CreateApp :one
INSERT INTO apps (name)
VALUES (@name)
RETURNING *;

-- name: CreateAppForUser :one
INSERT INTO apps (name, owner_user_id)
VALUES (@name, @owner_user_id)
RETURNING *;

-- name: GetAppByName :one
SELECT * FROM apps
WHERE name = @name
LIMIT 1;

-- name: GetAppByNameForUser :one
SELECT * FROM apps
WHERE name = @name AND owner_user_id = @owner_user_id
LIMIT 1;

-- name: GetAppByID :one
SELECT * FROM apps
WHERE id = @id
LIMIT 1;

-- name: GetAppByIDForUser :one
SELECT * FROM apps
WHERE id = @id AND owner_user_id = @owner_user_id
LIMIT 1;

-- name: ListApps :many
SELECT * FROM apps
ORDER BY created_at DESC;

-- name: ListAppsForUser :many
SELECT * FROM apps
WHERE owner_user_id = @owner_user_id
ORDER BY created_at DESC;

-- name: UpdateAppStatus :exec
UPDATE apps
SET status = @status, updated_at = now()
WHERE id = @id;

-- name: UpdateAppStatusForUser :exec
UPDATE apps
SET status = @status, updated_at = now()
WHERE id = @id AND owner_user_id = @owner_user_id;

-- name: UpdateAppPublicURL :exec
UPDATE apps
SET public_url = @public_url, updated_at = now()
WHERE id = @id;

-- name: UpdateAppPublicURLForUser :exec
UPDATE apps
SET public_url = @public_url, updated_at = now()
WHERE id = @id AND owner_user_id = @owner_user_id;

-- name: DeleteApp :exec
DELETE FROM apps WHERE id = @id;

-- name: DeleteAppForUser :exec
DELETE FROM apps WHERE id = @id AND owner_user_id = @owner_user_id;
