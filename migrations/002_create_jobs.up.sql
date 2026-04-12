CREATE TABLE IF NOT EXISTS jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id     UUID NOT NULL REFERENCES events(id),
    job_type     TEXT NOT NULL CHECK (job_type IN ('jira_transition', 'larkbase_sync', 'feishu_notify')),
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'dead')),
    issue_key    TEXT NOT NULL,
    target_state TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    last_error   TEXT NOT NULL DEFAULT '',
    next_run_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- Partial index used by the worker dequeue query
CREATE INDEX IF NOT EXISTS idx_jobs_dequeue ON jobs (next_run_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX IF NOT EXISTS idx_jobs_event_id ON jobs (event_id);
CREATE INDEX IF NOT EXISTS idx_jobs_issue_key ON jobs (issue_key);

-- For reconciler: quickly find non-terminal jobs
CREATE INDEX IF NOT EXISTS idx_jobs_active_status ON jobs (status)
    WHERE status NOT IN ('completed', 'dead');
