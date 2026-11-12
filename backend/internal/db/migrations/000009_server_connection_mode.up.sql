-- Access path for a server: ssh (remote, current behavior) or local
-- (commands run in the API/worker process against the host Docker socket).
-- Existing rows stay ssh via the column default.

ALTER TABLE servers
    ADD COLUMN connection_mode text NOT NULL DEFAULT 'ssh';

ALTER TABLE servers
    ADD CONSTRAINT servers_connection_mode_check
        CHECK (connection_mode IN ('ssh', 'local'));

-- The original status CHECK was declared inline in 000001, so its name is
-- whatever Postgres assigned. Locate it by definition instead of guessing.
DO $$
DECLARE
    cname text;
BEGIN
    SELECT con.conname INTO cname
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'servers'
      AND con.contype = 'c'
      AND pg_get_constraintdef(con.oid) LIKE '%pending_key%'
      AND pg_get_constraintdef(con.oid) LIKE '%awaiting_authorization%'
      AND pg_get_constraintdef(con.oid) NOT LIKE '%connection_mode%';
    IF cname IS NULL THEN
        RAISE EXCEPTION 'servers status check constraint not found';
    END IF;
    EXECUTE format('ALTER TABLE servers DROP CONSTRAINT %I', cname);
END $$;

ALTER TABLE servers
    ADD CONSTRAINT servers_status_check
        CHECK (status IN ('pending_key', 'awaiting_authorization', 'ready', 'disabled', 'pending_local'));

ALTER TABLE servers
    ADD CONSTRAINT servers_connection_mode_fields CHECK (
        (connection_mode = 'ssh' AND host <> '' AND ssh_user <> '' AND port BETWEEN 1 AND 65535)
        OR (connection_mode = 'local' AND deployment_mode = 'docker')
    );
