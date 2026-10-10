-- Revision history is append-only after creation.
-- The only permitted row update is the transition from
-- unfinalized to finalized, without modifying historical fields.

CREATE FUNCTION protect_project_revision_history()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NOT OLD.membership_finalized
           AND NEW.membership_finalized
           AND ROW(
               NEW.id,
               NEW.project_id,
               NEW.author_user_id,
               NEW.message,
               NEW.parent_revision_id,
               NEW.merge_parent_revision_id,
               NEW.created_at
           ) IS NOT DISTINCT FROM ROW(
               OLD.id,
               OLD.project_id,
               OLD.author_user_id,
               OLD.message,
               OLD.parent_revision_id,
               OLD.merge_parent_revision_id,
               OLD.created_at
           ) THEN
            RETURN NEW;
        END IF;

        RAISE EXCEPTION 'project revision history is immutable'
            USING ERRCODE = 'P0001';
    END IF;

    IF TG_OP = 'DELETE' THEN
        -- Allow cascading deletion only when the containing project
        -- has already been deleted in the current transaction.
        IF EXISTS (
            SELECT 1
            FROM projects
            WHERE id = OLD.project_id
        ) THEN
            RAISE EXCEPTION 'project revisions cannot be deleted independently'
                USING ERRCODE = 'P0001';
        END IF;

        RETURN OLD;
    END IF;

    IF TG_OP = 'TRUNCATE' THEN
        RAISE EXCEPTION 'project revision history cannot be truncated'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN NULL;
END;
$$;

CREATE TRIGGER project_revisions_protect_history
BEFORE UPDATE OR DELETE ON project_revisions
FOR EACH ROW
EXECUTE FUNCTION protect_project_revision_history();

CREATE TRIGGER project_revisions_reject_truncation
BEFORE TRUNCATE ON project_revisions
FOR EACH STATEMENT
EXECUTE FUNCTION protect_project_revision_history();

---- create above / drop below ----

DROP TRIGGER project_revisions_reject_truncation
    ON project_revisions;

DROP TRIGGER project_revisions_protect_history
    ON project_revisions;

DROP FUNCTION protect_project_revision_history();
