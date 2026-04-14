-- Add running_since column to track when a job was claimed by a worker.
-- Used by the reconciler to detect truly stuck jobs (instead of next_run_at
-- which is set at creation time and never updated on dequeue).
ALTER TABLE jobs ADD COLUMN running_since TIMESTAMPTZ;

-- Backfill any existing running rows so the reconciler can reset them
-- on the next cycle.
UPDATE jobs SET running_since = now() WHERE status = 'running';
