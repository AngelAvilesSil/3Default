-- name: GetLoginCredentialByEmail :one
SELECT
    users.id,
    users.email,
    users.display_name,
    users.created_at,
    users.updated_at,
    password_credentials.password_hash
FROM users
INNER JOIN password_credentials
    ON password_credentials.user_id = users.id
WHERE lower(users.email) = lower(sqlc.arg(email));
