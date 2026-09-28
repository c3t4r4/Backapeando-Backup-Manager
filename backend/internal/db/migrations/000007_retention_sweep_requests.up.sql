-- Adds retention_sweep_requests table for async global purge queueing.
-- The scheduler worker polls this table, claiming pending requests with
-- SKIP LOCKED for isolation, then executes global sweep across all eligible
-- servers (status='ready' AND enabled=true AND storage_target_id IS NOT NULL),
-- aggregating results and recording the final status/summary for UI polling.

CREATE TABLE retention_sweep_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    requested_at timestamptz NOT NULL DEFAULT NOW(),
    started_at timestamptz,
    finished_at timestamptz,
    summary jsonb,
    error text,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retention_sweep_requests_status
    ON retention_sweep_requests (status, requested_at);
