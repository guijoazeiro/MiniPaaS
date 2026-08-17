-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash, token_prefix, scopes, expires_at)
VALUES (@user_id, @name, @token_hash, @token_prefix, @scopes, @expires_at)
RETURNING *;

-- name: ListAPITokensForUser :many
SELECT * FROM api_tokens
WHERE user_id = @user_id
ORDER BY created_at DESC;

-- name: GetAPITokenByHash :one
SELECT * FROM api_tokens
WHERE token_hash = @token_hash
LIMIT 1;

-- name: RevokeAPIToken :execrows
UPDATE api_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE id = @id AND user_id = @user_id;

-- name: TouchAPIToken :exec
UPDATE api_tokens
SET last_used_at = now()
WHERE id = @id
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());
