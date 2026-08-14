-- name: CreateAuditEvent :exec
INSERT INTO audit_events (user_id, action, method, path, status_code, request_id)
VALUES (sqlc.narg('user_id'), sqlc.arg('action'), sqlc.arg('method'), sqlc.arg('path'), sqlc.arg('status_code'), sqlc.arg('request_id'));

-- name: ListAuditEvents :many
SELECT id, user_id, action, method, path, status_code, request_id, created_at
FROM audit_events
ORDER BY id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
