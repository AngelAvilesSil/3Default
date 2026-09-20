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

-- name: UpdateProjectMetadataByIDAndOwner :one
UPDATE projects
SET
    name = CASE
        WHEN sqlc.arg(name_set)::boolean THEN sqlc.arg(name)::text
        ELSE name
    END,
    description = CASE
        WHEN sqlc.arg(description_set)::boolean
            THEN sqlc.narg(description)::text
        ELSE description
    END,
    updated_at = now()
WHERE id = sqlc.arg(project_id)
  AND owner_user_id = sqlc.arg(owner_user_id)
RETURNING
    id,
    owner_user_id,
    name,
    description,
    visibility,
    created_at,
    updated_at;
