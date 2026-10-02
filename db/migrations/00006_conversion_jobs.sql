CREATE TABLE conversion_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL,
    project_file_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    attempt_count integer NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT conversion_jobs_project_file_same_project
        FOREIGN KEY (project_id, project_file_id)
        REFERENCES project_files(project_id, id)
        ON DELETE CASCADE,

    CONSTRAINT conversion_jobs_status_valid
        CHECK (
            status IN (
                'pending',
                'running',
                'succeeded',
                'failed'
            )
        ),

    CONSTRAINT conversion_jobs_attempt_count_nonnegative
        CHECK (attempt_count >= 0),

    CONSTRAINT conversion_jobs_attempt_count_state_valid
        CHECK (
            status = 'pending'
            OR attempt_count > 0
        ),

    CONSTRAINT conversion_jobs_last_error_valid
        CHECK (
            last_error IS NULL
            OR (
                btrim(last_error) <> ''
                AND last_error = btrim(last_error)
            )
        ),

    CONSTRAINT conversion_jobs_updated_at_valid
        CHECK (updated_at >= created_at),

    CONSTRAINT conversion_jobs_started_at_valid
        CHECK (
            started_at IS NULL
            OR started_at >= created_at
        ),

    CONSTRAINT conversion_jobs_finished_at_valid
        CHECK (
            finished_at IS NULL
            OR finished_at >= COALESCE(started_at, created_at)
        ),

    CONSTRAINT conversion_jobs_state_valid
        CHECK (
            (
                status = 'pending'
                AND started_at IS NULL
                AND finished_at IS NULL
                AND last_error IS NULL
            )
            OR (
                status = 'running'
                AND started_at IS NOT NULL
                AND finished_at IS NULL
                AND last_error IS NULL
            )
            OR (
                status = 'succeeded'
                AND started_at IS NOT NULL
                AND finished_at IS NOT NULL
                AND last_error IS NULL
            )
            OR (
                status = 'failed'
                AND started_at IS NOT NULL
                AND finished_at IS NOT NULL
                AND last_error IS NOT NULL
            )
        )
);

CREATE UNIQUE INDEX conversion_jobs_one_active_per_project_file
    ON conversion_jobs (
        project_id,
        project_file_id
    )
    WHERE status IN ('pending', 'running');

CREATE INDEX conversion_jobs_pending_order
    ON conversion_jobs (
        created_at,
        id
    )
    WHERE status = 'pending';

CREATE INDEX conversion_jobs_project_file_history
    ON conversion_jobs (
        project_id,
        project_file_id,
        created_at DESC,
        id ASC
    );

---- create above / drop below ----

DROP TABLE conversion_jobs;
