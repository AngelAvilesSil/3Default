CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,

    CONSTRAINT sessions_token_hash_not_empty
        CHECK (octet_length(token_hash) > 0),
    CONSTRAINT sessions_expires_after_creation
        CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX sessions_token_hash_unique
    ON sessions (token_hash);

CREATE INDEX sessions_user_id_index
    ON sessions (user_id);

CREATE INDEX sessions_expires_at_index
    ON sessions (expires_at);

---- create above / drop below ----

DROP TABLE sessions;
