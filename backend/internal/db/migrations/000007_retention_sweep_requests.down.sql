-- Rollback: drop retention_sweep_requests table and index.

DROP INDEX IF EXISTS idx_retention_sweep_requests_status;
DROP TABLE IF EXISTS retention_sweep_requests;
