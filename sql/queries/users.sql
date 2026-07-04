-- name: CreateUser :one
INSERT INTO users(id, created_at, updated_at, email, hashed_password)
VALUES(
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2
)
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users where id = $1;


-- name: UpdateUserPassword :one
UPDATE users
SET hashed_password = $1
WHERE id = $2
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE users
SET email = $1
WHERE id = $2
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET
    email = $1,
    hashed_password = $2
WHERE id = $3
RETURNING *;