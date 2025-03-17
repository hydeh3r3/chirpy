-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES ($1, $2, $2, $3, $4)
RETURNING *;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = $1, updated_at = $1
WHERE token = $2;

-- name: GetUserFromRefreshToken :one
SELECT u.* FROM users u
JOIN refresh_tokens rt ON u.id = rt.user_id
WHERE rt.token = $1 AND rt.revoked_at IS NULL AND rt.expires_at > NOW(); 