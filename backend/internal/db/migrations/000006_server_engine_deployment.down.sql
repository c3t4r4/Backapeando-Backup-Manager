ALTER TABLE servers
    DROP CONSTRAINT servers_password_required_by_engine,
    DROP CONSTRAINT servers_container_name_requires_docker;

-- Existing host-mode rows (container_name IS NULL) would violate NOT NULL on
-- rollback; backfill a placeholder before restoring the constraint. This is a
-- lossy rollback (documented, not silently "correct") — acceptable because
-- rolling back this migration is an emergency path, not a supported
-- forward-compatible operation.
UPDATE servers SET container_name = '' WHERE container_name IS NULL;
ALTER TABLE servers ALTER COLUMN container_name SET NOT NULL;

ALTER TABLE servers
    DROP COLUMN db_engine,
    DROP COLUMN deployment_mode,
    DROP COLUMN db_password_encrypted,
    DROP COLUMN mysql_dump_extra_args,
    DROP COLUMN sqlcmd_extra_args;
