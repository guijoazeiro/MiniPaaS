-- name: CreateCustomDomain :one
INSERT INTO custom_domains (app_id, hostname)
VALUES (@app_id, @hostname)
RETURNING *;

-- name: GetCustomDomain :one
SELECT * FROM custom_domains WHERE id = @id;

-- name: ListCustomDomainsByApp :many
SELECT * FROM custom_domains
WHERE app_id = @app_id
ORDER BY created_at ASC;

-- name: UpdateCustomDomainVerification :exec
UPDATE custom_domains
SET status = @status,
    last_error = @last_error,
    verified_at = @verified_at,
    updated_at = now()
WHERE id = @id;

-- name: DeleteCustomDomain :exec
DELETE FROM custom_domains WHERE id = @id;
