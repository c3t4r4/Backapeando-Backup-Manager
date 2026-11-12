UPDATE servers SET status = 'pending_key' WHERE status = 'pending_local';

ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_connection_mode_fields;
ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_connection_mode_check;
ALTER TABLE servers DROP COLUMN IF EXISTS connection_mode;

ALTER TABLE servers DROP CONSTRAINT IF EXISTS servers_status_check;
ALTER TABLE servers
    ADD CONSTRAINT servers_status_check
        CHECK (status IN ('pending_key', 'awaiting_authorization', 'ready', 'disabled'));
