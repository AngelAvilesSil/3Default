CREATE TABLE project_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL
        REFERENCES projects(id)
        ON DELETE CASCADE,
    author_user_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,
    message text NOT NULL,
    parent_revision_id uuid,
    merge_parent_revision_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT project_revisions_project_id_id_unique
        UNIQUE (project_id, id),
    CONSTRAINT project_revisions_message_not_blank
        CHECK (btrim(message) <> ''),
    CONSTRAINT project_revisions_message_trimmed
        CHECK (message = btrim(message)),
    CONSTRAINT project_revisions_parent_not_self
        CHECK (
            parent_revision_id IS NULL
            OR parent_revision_id <> id
        ),
    CONSTRAINT project_revisions_merge_parent_not_self
        CHECK (
            merge_parent_revision_id IS NULL
            OR merge_parent_revision_id <> id
        ),
    CONSTRAINT project_revisions_merge_parent_requires_parent
        CHECK (
            merge_parent_revision_id IS NULL
            OR parent_revision_id IS NOT NULL
        ),
    CONSTRAINT project_revisions_parents_distinct
        CHECK (
            merge_parent_revision_id IS NULL
            OR merge_parent_revision_id <> parent_revision_id
        ),
    CONSTRAINT project_revisions_parent_same_project
        FOREIGN KEY (project_id, parent_revision_id)
        REFERENCES project_revisions(project_id, id),
    CONSTRAINT project_revisions_merge_parent_same_project
        FOREIGN KEY (project_id, merge_parent_revision_id)
        REFERENCES project_revisions(project_id, id)
);

CREATE TABLE project_branches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL
        REFERENCES projects(id)
        ON DELETE CASCADE,
    name text NOT NULL,
    head_revision_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT project_branches_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT project_branches_name_trimmed
        CHECK (name = btrim(name)),
    CONSTRAINT project_branches_updated_not_before_created
        CHECK (updated_at >= created_at),
    CONSTRAINT project_branches_project_name_unique
        UNIQUE (project_id, name),
    CONSTRAINT project_branches_head_same_project
        FOREIGN KEY (project_id, head_revision_id)
        REFERENCES project_revisions(project_id, id)
);

INSERT INTO project_branches (
    project_id,
    name
)
SELECT
    id,
    'main'
FROM projects;

---- create above / drop below ----

DROP TABLE project_branches;
DROP TABLE project_revisions;
