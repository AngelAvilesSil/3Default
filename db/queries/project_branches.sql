-- name: CreateProjectBranch :one
INSERT INTO project_branches (
    project_id,
    name
)
VALUES (
    sqlc.arg(project_id),
    sqlc.arg(name)
)
RETURNING
    id,
    project_id,
    name,
    head_revision_id,
    created_at,
    updated_at;
