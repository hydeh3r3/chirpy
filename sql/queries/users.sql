-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET email = $1, hashed_password = $2, updated_at = $3
WHERE id = $4
RETURNING *;

-- name: UpgradeUserToChirpyRed :exec
UPDATE users SET is_chirpy_red = true WHERE id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;
