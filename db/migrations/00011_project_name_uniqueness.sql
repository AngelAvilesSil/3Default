-- Project names must be unique per owner, ignoring letter case.
-- The existing projects_name_trimmed constraint enforces stored-name trimming.

CREATE UNIQUE INDEX projects_owner_name_case_insensitive_unique
    ON projects (owner_user_id, lower(name));

---- create above / drop below ----

DROP INDEX projects_owner_name_case_insensitive_unique;
