-- Add cron scheduling and backup tracking fields
-- Supports Phase 4: Scheduler worker with SKIP LOCKED polling

ALTER TABLE servers ADD COLUMN last_scheduled_at TIMESTAMP WITH TIME ZONE;

-- Update backup_runs to ensure all required columns for scheduler exist
-- Note: status, blob_name, blob_size_bytes, error_message already exist from schema 000001
-- No additional columns needed; this migration is primarily for servers.last_scheduled_at
