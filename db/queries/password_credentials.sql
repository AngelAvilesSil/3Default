-- name: CreatePasswordCredential :one
INSERT INTO password_credentials (
    user_id,
    password_hash
)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(password_hash)
)
RETURNING
    user_id,
    password_hash,
    created_at,
    updated_at;

-- name: GetPasswordCredentialByUserID :one
SELECT
    user_id,
    password_hash,
    created_at,
    updated_at
FROM password_credentials
WHERE user_id = sqlc.arg(user_id);
