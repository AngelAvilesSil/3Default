CREATE TABLE content_objects (
    sha256 text PRIMARY KEY,
    size_bytes bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT content_objects_sha256_valid
        CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT content_objects_size_nonnegative
        CHECK (size_bytes >= 0)
);

CREATE TABLE project_files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL,
    uploaded_by_user_id uuid NOT NULL,
    content_sha256 text NOT NULL,
    original_filename text NOT NULL,
    media_type text,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT project_files_project_id_id_unique
        UNIQUE (project_id, id),
    CONSTRAINT project_files_project_exists
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON DELETE CASCADE,
    CONSTRAINT project_files_uploader_exists
        FOREIGN KEY (uploaded_by_user_id)
        REFERENCES users(id)
        ON DELETE RESTRICT,
    CONSTRAINT project_files_content_exists
        FOREIGN KEY (content_sha256)
        REFERENCES content_objects(sha256)
        ON DELETE RESTRICT,
    CONSTRAINT project_files_original_filename_not_blank
        CHECK (btrim(original_filename) <> ''),
    CONSTRAINT project_files_original_filename_trimmed
        CHECK (original_filename = btrim(original_filename)),
    CONSTRAINT project_files_media_type_not_blank
        CHECK (
            media_type IS NULL
            OR btrim(media_type) <> ''
        ),
    CONSTRAINT project_files_media_type_trimmed
        CHECK (
            media_type IS NULL
            OR media_type = btrim(media_type)
        )
);

CREATE TABLE project_revision_files (
    project_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    project_file_id uuid NOT NULL,

    CONSTRAINT project_revision_files_primary_key
        PRIMARY KEY (
            project_id,
            revision_id,
            project_file_id
        ),
    CONSTRAINT project_revision_files_revision_same_project
        FOREIGN KEY (project_id, revision_id)
        REFERENCES project_revisions(project_id, id)
        ON DELETE CASCADE,
    CONSTRAINT project_revision_files_file_same_project
        FOREIGN KEY (project_id, project_file_id)
        REFERENCES project_files(project_id, id)
);

---- create above / drop below ----

DROP TABLE project_revision_files;
DROP TABLE project_files;
DROP TABLE content_objects;
