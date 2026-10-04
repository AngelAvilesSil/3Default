CREATE TABLE conversion_job_outputs (
    conversion_job_id uuid PRIMARY KEY,
    content_sha256 text NOT NULL,
    media_type text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT conversion_job_outputs_job_exists
        FOREIGN KEY (conversion_job_id)
        REFERENCES conversion_jobs(id)
        ON DELETE CASCADE,

    CONSTRAINT conversion_job_outputs_content_exists
        FOREIGN KEY (content_sha256)
        REFERENCES content_objects(sha256)
        ON DELETE RESTRICT,

    CONSTRAINT conversion_job_outputs_media_type_not_blank
        CHECK (btrim(media_type) <> ''),

    CONSTRAINT conversion_job_outputs_media_type_trimmed
        CHECK (media_type = btrim(media_type))
);

---- create above / drop below ----

DROP TABLE conversion_job_outputs;
