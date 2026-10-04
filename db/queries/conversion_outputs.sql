-- name: CreateConversionJobOutput :one
INSERT INTO conversion_job_outputs (
    conversion_job_id,
    content_sha256,
    media_type
)
SELECT
    job.id,
    sqlc.arg(content_sha256),
    sqlc.arg(media_type)
FROM conversion_jobs AS job
WHERE job.id = sqlc.arg(conversion_job_id)
  AND job.status = 'running'
RETURNING
    conversion_job_id,
    content_sha256,
    media_type,
    created_at;

-- name: GetConversionJobOutputByJobID :one
SELECT
    conversion_job_id,
    content_sha256,
    media_type,
    created_at
FROM conversion_job_outputs
WHERE conversion_job_id = sqlc.arg(conversion_job_id);

-- name: GetLatestConversionJobOutputByProjectFile :one
SELECT
    output.conversion_job_id,
    output.content_sha256,
    output.media_type,
    output.created_at
FROM conversion_job_outputs AS output
JOIN conversion_jobs AS job
  ON job.id = output.conversion_job_id
WHERE job.project_id = sqlc.arg(project_id)
  AND job.project_file_id = sqlc.arg(project_file_id)
  AND job.status = 'succeeded'
ORDER BY
    job.finished_at DESC,
    job.id DESC
LIMIT 1;
