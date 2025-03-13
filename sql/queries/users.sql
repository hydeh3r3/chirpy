-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES ($1, $2, $2, $3)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;
