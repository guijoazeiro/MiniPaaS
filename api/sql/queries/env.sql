-- name: UpsertEnvVar :exec
INSERT INTO env_vars (app_id, key, value, nonce)
VALUES (@app_id, @key, @value, @nonce)
ON CONFLICT (app_id, key) DO UPDATE
SET value      = EXCLUDED.value,
    nonce      = EXCLUDED.nonce,
    updated_at = now();

-- name: ListEnvVarsByApp :many
SELECT id, app_id, key, updated_at FROM env_vars
WHERE app_id = @app_id
ORDER BY key;

-- name: GetEnvVar :one
SELECT * FROM env_vars
WHERE app_id = @app_id AND key = @key;

-- name: GetEnvVarsByApp :many
SELECT * FROM env_vars WHERE app_id = @app_id;

-- name: DeleteEnvVar :exec
DELETE FROM env_vars WHERE app_id = @app_id AND key = @key;
