-- Revision membership is constructed within one transaction.
-- Existing revisions are finalized during migration.

ALTER TABLE project_revisions
ADD COLUMN membership_finalized boolean NOT NULL DEFAULT FALSE;

UPDATE project_revisions
SET membership_finalized = TRUE;

CREATE FUNCTION prevent_revision_reopening()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.membership_finalized AND NOT NEW.membership_finalized THEN
        RAISE EXCEPTION 'finalized revisions cannot be reopened'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER project_revisions_prevent_reopening
BEFORE UPDATE OF membership_finalized ON project_revisions
FOR EACH ROW
EXECUTE FUNCTION prevent_revision_reopening();

-- This deferred check reads the current row state, not the initial
-- NEW value, because the revision is finalized later in the transaction.
CREATE FUNCTION require_finalized_revision_at_commit()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM project_revisions
        WHERE project_id = NEW.project_id
          AND id = NEW.id
          AND membership_finalized = FALSE
    ) THEN
        RAISE EXCEPTION 'project revision must be finalized before commit'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER project_revisions_require_finalization
AFTER INSERT OR UPDATE OF membership_finalized ON project_revisions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION require_finalized_revision_at_commit();

CREATE FUNCTION protect_revision_file_membership()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    revision_finalized boolean;
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- Serialize membership insertion with revision finalization.
        SELECT membership_finalized
        INTO revision_finalized
        FROM project_revisions
        WHERE project_id = NEW.project_id
          AND id = NEW.revision_id
        FOR UPDATE;

        -- Preserve foreign-key errors for nonexistent revisions.
        IF FOUND AND revision_finalized THEN
            RAISE EXCEPTION 'finalized revision membership is immutable'
                USING ERRCODE = 'P0001';
        END IF;

        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE' THEN
        RAISE EXCEPTION 'revision file membership is immutable'
            USING ERRCODE = 'P0001';
    END IF;

    -- During a project-level cascading deletion, its project row
    -- has already been deleted within the transaction.
    IF EXISTS (
        SELECT 1
        FROM projects
        WHERE id = OLD.project_id
    ) THEN
        RAISE EXCEPTION 'revision file membership is immutable'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN OLD;
END;
$$;

CREATE TRIGGER project_revision_files_protect_membership
BEFORE INSERT OR UPDATE OR DELETE ON project_revision_files
FOR EACH ROW
EXECUTE FUNCTION protect_revision_file_membership();

-- TRUNCATE bypasses row-level DELETE triggers, so reject it explicitly.
CREATE FUNCTION reject_revision_membership_truncation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'revision file membership cannot be truncated'
        USING ERRCODE = 'P0001';
    RETURN NULL;
END;
$$;

CREATE TRIGGER project_revision_files_reject_truncation
BEFORE TRUNCATE ON project_revision_files
FOR EACH STATEMENT
EXECUTE FUNCTION reject_revision_membership_truncation();

---- create above / drop below ----

DROP TRIGGER project_revision_files_reject_truncation
    ON project_revision_files;
DROP FUNCTION reject_revision_membership_truncation();

DROP TRIGGER project_revision_files_protect_membership
    ON project_revision_files;
DROP FUNCTION protect_revision_file_membership();

DROP TRIGGER project_revisions_require_finalization
    ON project_revisions;
DROP FUNCTION require_finalized_revision_at_commit();

DROP TRIGGER project_revisions_prevent_reopening
    ON project_revisions;
DROP FUNCTION prevent_revision_reopening();

ALTER TABLE project_revisions
DROP COLUMN membership_finalized;
