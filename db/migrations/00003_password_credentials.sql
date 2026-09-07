CREATE TABLE password_credentials (
    user_id uuid PRIMARY KEY
        REFERENCES users(id)
        ON DELETE CASCADE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT password_credentials_password_hash_not_blank
        CHECK (btrim(password_hash) <> ''),
    CONSTRAINT password_credentials_password_hash_trimmed
        CHECK (password_hash = btrim(password_hash)),
    CONSTRAINT password_credentials_updated_not_before_created
        CHECK (updated_at >= created_at)
);

---- create above / drop below ----

DROP TABLE password_credentials;
