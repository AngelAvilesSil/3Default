CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT users_email_not_blank
        CHECK (btrim(email) <> ''),
    CONSTRAINT users_email_trimmed
        CHECK (email = btrim(email)),
    CONSTRAINT users_display_name_not_blank
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT users_display_name_trimmed
        CHECK (display_name = btrim(display_name))
);

CREATE UNIQUE INDEX users_email_case_insensitive_unique
    ON users (lower(email));

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE RESTRICT,
    name text NOT NULL,
    description text,
    visibility text NOT NULL DEFAULT 'private',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT projects_name_not_blank
        CHECK (btrim(name) <> ''),
    CONSTRAINT projects_name_trimmed
        CHECK (name = btrim(name)),
    CONSTRAINT projects_visibility_valid
        CHECK (visibility IN ('private', 'public'))
);

---- create above / drop below ----

DROP TABLE projects;
DROP TABLE users;
