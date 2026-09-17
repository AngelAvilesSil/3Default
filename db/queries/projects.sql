-- name: CreateProject :one
INSERT INTO projects (
    owner_user_id,
    name,
    description
)
VALUES (
    sqlc.arg(owner_user_id),
    sqlc.arg(name),
    sqlc.narg(description)
)
RETURNING
    id,
    owner_user_id,
    name,
    description,
    visibility,
    created_at,
    updated_at;

-- name: ListProjectsByOwner :many
SELECT
    id,
    owner_user_id,
    name,
    description,
    visibility,
    created_at,
    updated_at
FROM projects
WHERE owner_user_id = sqlc.arg(owner_user_id)
ORDER BY created_at DESC, id DESC;

-- name: GetProjectByIDAndOwner :one
SELECT
    id,
    owner_user_id,
    name,
    description,
    visibility,
    created_at,
    updated_at
FROM projects
WHERE id = sqlc.arg(project_id)
  AND owner_user_id = sqlc.arg(owner_user_id);
