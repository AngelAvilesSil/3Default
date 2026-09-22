-- name: CreateProjectRevision :one
INSERT INTO project_revisions (
    project_id,
    author_user_id,
    message,
    parent_revision_id,
    merge_parent_revision_id
)
VALUES (
    sqlc.arg(project_id),
    sqlc.arg(author_user_id),
    sqlc.arg(message),
    sqlc.narg(parent_revision_id),
    sqlc.narg(merge_parent_revision_id)
)
RETURNING
    id,
    project_id,
    author_user_id,
    message,
    parent_revision_id,
    merge_parent_revision_id,
    created_at;

-- name: GetProjectRevisionByIDAndProject :one
SELECT
    id,
    project_id,
    author_user_id,
    message,
    parent_revision_id,
    merge_parent_revision_id,
    created_at
FROM project_revisions
WHERE id = sqlc.arg(revision_id)
  AND project_id = sqlc.arg(project_id);
