CREATE TABLE IF NOT EXISTS audit_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID REFERENCES events(id),
    job_id     UUID REFERENCES jobs(id),
    action     TEXT NOT NULL,
    issue_key  TEXT NOT NULL DEFAULT '',
    detail     JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_event_id ON audit_logs (event_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_issue_key ON audit_logs (issue_key);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at DESC);
