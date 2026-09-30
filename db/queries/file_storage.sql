-- name: EnsureContentObject :one
INSERT INTO content_objects (
    sha256,
    size_bytes
)
VALUES (
    sqlc.arg(sha256),
    sqlc.arg(size_bytes)
)
ON CONFLICT (sha256) DO UPDATE
SET
    size_bytes = content_objects.size_bytes
WHERE content_objects.size_bytes = EXCLUDED.size_bytes
RETURNING
    sha256,
    size_bytes,
    created_at;

-- name: GetContentObjectBySHA256 :one
SELECT
    sha256,
    size_bytes,
    created_at
FROM content_objects
WHERE sha256 = sqlc.arg(sha256);

-- name: CreateProjectFile :one
INSERT INTO project_files (
    project_id,
    uploaded_by_user_id,
    content_sha256,
    original_filename,
    media_type
)
VALUES (
    sqlc.arg(project_id),
    sqlc.arg(uploaded_by_user_id),
    sqlc.arg(content_sha256),
    sqlc.arg(original_filename),
    sqlc.narg(media_type)
)
RETURNING
    id,
    project_id,
    uploaded_by_user_id,
    content_sha256,
    original_filename,
    media_type,
    created_at;

-- name: GetProjectFileByIDAndProject :one
SELECT
    id,
    project_id,
    uploaded_by_user_id,
    content_sha256,
    original_filename,
    media_type,
    created_at
FROM project_files
WHERE id = sqlc.arg(project_file_id)
  AND project_id = sqlc.arg(project_id);

-- name: ListProjectFilesByProject :many
SELECT
    id,
    project_id,
    uploaded_by_user_id,
    content_sha256,
    original_filename,
    media_type,
    created_at
FROM project_files
WHERE project_id = sqlc.arg(project_id)
ORDER BY created_at DESC, id DESC;

-- name: CreateProjectRevisionFile :one
INSERT INTO project_revision_files (
    project_id,
    revision_id,
    project_file_id
)
SELECT
    sqlc.arg(project_id),
    sqlc.arg(revision_id),
    project_file.id
FROM project_files AS project_file
WHERE project_file.project_id = sqlc.arg(project_id)
  AND project_file.id = sqlc.arg(project_file_id)
RETURNING
    project_id,
    revision_id,
    project_file_id;

-- name: ListProjectFilesByRevision :many
SELECT
    project_file.id,
    project_file.project_id,
    project_file.uploaded_by_user_id,
    project_file.content_sha256,
    project_file.original_filename,
    project_file.media_type,
    project_file.created_at
FROM project_revision_files AS revision_file
JOIN project_files AS project_file
  ON project_file.project_id = revision_file.project_id
 AND project_file.id = revision_file.project_file_id
WHERE revision_file.project_id = sqlc.arg(project_id)
  AND revision_file.revision_id = sqlc.arg(revision_id)
ORDER BY
    project_file.created_at ASC,
    project_file.id ASC;
