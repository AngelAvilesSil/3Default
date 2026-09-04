-- name: CreateUser :one
INSERT INTO users (
    email,
    display_name
)
VALUES (
    sqlc.arg(email),
    sqlc.arg(display_name)
)
RETURNING
    id,
    email,
    display_name,
    created_at,
    updated_at;

-- name: GetUserByEmail :one
SELECT
    id,
    email,
    display_name,
    created_at,
    updated_at
FROM users
WHERE lower(email) = lower(sqlc.arg(email));