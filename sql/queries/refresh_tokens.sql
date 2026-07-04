-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens(token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES(
    $1,
    now(),
    now(),
    $2,
    $3,
    null    
)
RETURNING *;

-- name: GetToken :one
SELECT * FROM refresh_tokens WHERE token = $1;

-- name: SetRevoked :one
UPDATE refresh_tokens
SET revoked_at = now()
WHERE token = $1
RETURNING *;