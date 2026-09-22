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

-- name: GetProjectBranchByIDAndProject :one
SELECT
    id,
    project_id,
    name,
    head_revision_id,
    created_at,
    updated_at
FROM project_branches
WHERE id = sqlc.arg(branch_id)
  AND project_id = sqlc.arg(project_id);

-- name: AdvanceProjectBranchHead :one
UPDATE project_branches
SET
    head_revision_id = sqlc.arg(new_head_revision_id),
    updated_at = now()
WHERE id = sqlc.arg(branch_id)
  AND project_id = sqlc.arg(project_id)
  AND head_revision_id IS NOT DISTINCT FROM
      sqlc.narg(expected_head_revision_id)
RETURNING
    id,
    project_id,
    name,
    head_revision_id,
    created_at,
    updated_at;
