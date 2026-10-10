-- Project-file records represent immutable source-file identities.
-- New uploads create new rows rather than modifying existing rows.

CREATE FUNCTION reject_project_file_updates()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'project_files rows are immutable'
        USING ERRCODE = 'P0001';
END;
$$;

CREATE TRIGGER project_files_reject_updates
BEFORE UPDATE ON project_files
FOR EACH ROW
EXECUTE FUNCTION reject_project_file_updates();

---- create above / drop below ----

DROP TRIGGER project_files_reject_updates ON project_files;
DROP FUNCTION reject_project_file_updates();
