-- Immutable storage folder prefix for a server's backup blobs (RN-BACKUP-034).
-- Nullable initially so existing rows can be backfilled by the application
-- (same Slugify as Go). New inserts always set blob_prefix.
ALTER TABLE servers ADD COLUMN blob_prefix text;
