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
