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

-- name: ListReachableProjectRevisionsFromRevision :many
WITH RECURSIVE reachable_revision_ids (id) AS (
    SELECT
        revision.id
    FROM project_revisions AS revision
    WHERE revision.id = sqlc.arg(start_revision_id)
      AND revision.project_id = sqlc.arg(project_id)

    UNION

    SELECT
        parent.id
    FROM reachable_revision_ids AS reachable
    JOIN project_revisions AS child
      ON child.id = reachable.id
     AND child.project_id = sqlc.arg(project_id)
    JOIN project_revisions AS parent
      ON parent.project_id = child.project_id
     AND (
            parent.id = child.parent_revision_id
            OR parent.id = child.merge_parent_revision_id
         )
)
SELECT
    revision.*
FROM project_revisions AS revision
JOIN reachable_revision_ids AS reachable
  ON reachable.id = revision.id
WHERE revision.project_id = sqlc.arg(project_id)
ORDER BY revision.created_at DESC, revision.id ASC;
