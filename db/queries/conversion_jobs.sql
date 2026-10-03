-- name: CreateConversionJob :one
INSERT INTO conversion_jobs (
    project_id,
    project_file_id
)
SELECT
    project_file.project_id,
    project_file.id
FROM project_files AS project_file
WHERE project_file.project_id = sqlc.arg(project_id)
  AND project_file.id = sqlc.arg(project_file_id)
RETURNING
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at;

-- name: GetConversionJobByIDAndProject :one
SELECT
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at
FROM conversion_jobs
WHERE id = sqlc.arg(conversion_job_id)
  AND project_id = sqlc.arg(project_id);

-- name: ListConversionJobsByProjectFile :many
SELECT
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at
FROM conversion_jobs
WHERE project_id = sqlc.arg(project_id)
  AND project_file_id = sqlc.arg(project_file_id)
ORDER BY
    created_at DESC,
    id ASC;

-- name: ClaimNextPendingConversionJob :one
WITH next_job AS (
    SELECT id
    FROM conversion_jobs
    WHERE status = 'pending'
    ORDER BY
        created_at ASC,
        id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE conversion_jobs AS job
SET
    status = 'running',
    attempt_count = job.attempt_count + 1,
    started_at = now(),
    finished_at = NULL,
    last_error = NULL,
    updated_at = now()
FROM next_job
WHERE job.id = next_job.id
RETURNING
    job.id,
    job.project_id,
    job.project_file_id,
    job.status,
    job.attempt_count,
    job.last_error,
    job.created_at,
    job.started_at,
    job.finished_at,
    job.updated_at;

-- name: MarkConversionJobSucceeded :one
UPDATE conversion_jobs
SET
    status = 'succeeded',
    finished_at = now(),
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(conversion_job_id)
  AND status = 'running'
RETURNING
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at;

-- name: MarkConversionJobFailed :one
UPDATE conversion_jobs
SET
    status = 'failed',
    finished_at = now(),
    last_error = sqlc.arg(last_error),
    updated_at = now()
WHERE id = sqlc.arg(conversion_job_id)
  AND status = 'running'
RETURNING
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at;

-- name: RequeueConversionJob :one
UPDATE conversion_jobs
SET
    status = 'pending',
    started_at = NULL,
    finished_at = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(conversion_job_id)
  AND status = 'running'
RETURNING
    id,
    project_id,
    project_file_id,
    status,
    attempt_count,
    last_error,
    created_at,
    started_at,
    finished_at,
    updated_at;

-- name: RequeueRunningConversionJobs :exec
UPDATE conversion_jobs
SET
    status = 'pending',
    started_at = NULL,
    finished_at = NULL,
    last_error = NULL,
    updated_at = now()
WHERE status = 'running';
