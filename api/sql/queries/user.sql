-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES (@username, @password_hash)
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = @id LIMIT 1;

-- name: UpdateUsername :one
UPDATE users
SET username = @username
WHERE id = @id
RETURNING *;

-- name: UpdatePassword :exec
UPDATE users
SET password_hash = @password_hash
WHERE id = @id;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
