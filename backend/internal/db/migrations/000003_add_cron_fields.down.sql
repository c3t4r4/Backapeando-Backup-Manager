-- Rollback Phase 4 scheduler fields

ALTER TABLE servers DROP COLUMN last_scheduled_at;
