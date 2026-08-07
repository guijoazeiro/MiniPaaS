-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES (@username, @password_hash)
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = @username LIMIT 1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;
