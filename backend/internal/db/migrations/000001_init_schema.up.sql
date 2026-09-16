CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE admin_users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email           citext NOT NULL UNIQUE,
    password_hash   text NOT NULL,
    role            text NOT NULL DEFAULT 'admin',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    last_login_at   timestamptz
);

CREATE TABLE admin_sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    user_agent  text,
    ip          text
);
CREATE INDEX idx_admin_sessions_user_id ON admin_sessions(user_id);
CREATE INDEX idx_admin_sessions_expires_at ON admin_sessions(expires_at);

CREATE TABLE azure_targets (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    text NOT NULL,
    account_name            text NOT NULL,
    container_name          text NOT NULL,
    sas_token_encrypted     bytea NOT NULL,
    sas_token_expires_at    timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE servers (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                        text NOT NULL,
    host                        text NOT NULL,
    port                        integer NOT NULL DEFAULT 22,
    ssh_user                    text NOT NULL,
    container_name              text NOT NULL,
    db_name                     text NOT NULL,
    db_user                     text NOT NULL,
    pg_dump_extra_args          text NOT NULL DEFAULT '',
    ssh_private_key_encrypted   bytea,
    ssh_public_key              text,
    ssh_key_fingerprint         text,
    ssh_host_key_fingerprint    text,
    azure_target_id             uuid REFERENCES azure_targets(id),
    cron_expression             text NOT NULL DEFAULT '0 3 * * *',
    enabled                      boolean NOT NULL DEFAULT false,
    status                      text NOT NULL DEFAULT 'pending_key'
        CHECK (status IN ('pending_key', 'awaiting_authorization', 'ready', 'disabled')),
    last_test_connection_at    timestamptz,
    last_test_connection_ok    boolean,
    next_run_at                 timestamptz,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_servers_next_run_at ON servers(next_run_at) WHERE enabled AND status = 'ready';

CREATE TABLE retention_policies (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id       uuid UNIQUE REFERENCES servers(id) ON DELETE CASCADE,
    recent_count    integer NOT NULL DEFAULT 3,
    monthly_count   integer NOT NULL DEFAULT 12,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
-- server_id IS NULL represents the single global default policy row
CREATE UNIQUE INDEX idx_retention_policies_global ON retention_policies((server_id IS NULL)) WHERE server_id IS NULL;

CREATE TABLE backup_runs (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id           uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status              text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'success', 'failed')),
    started_at          timestamptz,
    finished_at          timestamptz,
    blob_name           text,
    blob_size_bytes      bigint,
    dump_duration_ms     integer,
    upload_duration_ms   integer,
    error_message        text,
    log_output           text,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_backup_runs_server_id ON backup_runs(server_id, created_at DESC);

CREATE TABLE retention_deletions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id       uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    backup_run_id   uuid REFERENCES backup_runs(id) ON DELETE SET NULL,
    blob_name       text NOT NULL,
    deleted_at      timestamptz NOT NULL DEFAULT now(),
    reason          text
);
CREATE INDEX idx_retention_deletions_server_id ON retention_deletions(server_id, deleted_at DESC);

INSERT INTO retention_policies (server_id, recent_count, monthly_count) VALUES (NULL, 3, 12);
