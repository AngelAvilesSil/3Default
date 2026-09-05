-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    token_hash,
    expires_at
)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(expires_at)
)
RETURNING
    id,
    user_id,
    token_hash,
    created_at,
    expires_at;

-- name: GetActiveSessionByTokenHash :one
SELECT
    id,
    user_id,
    created_at,
    expires_at
FROM sessions
WHERE token_hash = sqlc.arg(token_hash)
  AND expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE token_hash = sqlc.arg(token_hash);

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions
WHERE expires_at <= now();
