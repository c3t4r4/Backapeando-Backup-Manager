-- Adds database engine (postgres/mysql/sqlserver) and deployment mode
-- (docker/host) discrimination to servers, plus per-engine extra-args
-- columns and an optional encrypted DB password (required for mysql/
-- sqlserver, optional for postgres to preserve today's trust/peer-auth
-- behavior).

ALTER TABLE servers
    ADD COLUMN db_engine text NOT NULL DEFAULT 'postgres'
        CHECK (db_engine IN ('postgres', 'mysql', 'sqlserver')),
    ADD COLUMN deployment_mode text NOT NULL DEFAULT 'docker'
        CHECK (deployment_mode IN ('docker', 'host')),
    ADD COLUMN db_password_encrypted bytea,
    ADD COLUMN mysql_dump_extra_args text NOT NULL DEFAULT '',
    ADD COLUMN sqlcmd_extra_args text NOT NULL DEFAULT '';

-- container_name becomes nullable: only required when deployment_mode='docker'.
ALTER TABLE servers ALTER COLUMN container_name DROP NOT NULL;

ALTER TABLE servers
    ADD CONSTRAINT servers_container_name_requires_docker CHECK (
        (deployment_mode = 'docker' AND container_name IS NOT NULL AND container_name <> '')
        OR (deployment_mode = 'host' AND container_name IS NULL)
    ),
    ADD CONSTRAINT servers_password_required_by_engine CHECK (
        db_engine = 'postgres' OR db_password_encrypted IS NOT NULL
    );
