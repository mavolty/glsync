CREATE TABLE IF NOT EXISTS events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT NOT NULL UNIQUE,
    event_type      TEXT NOT NULL CHECK (event_type IN ('push', 'mr_opened', 'mr_merged', 'mr_draft', 'unrecognized')),
    issue_keys      TEXT[] NOT NULL DEFAULT '{}',
    source_branch   TEXT NOT NULL DEFAULT '',
    target_branch   TEXT NOT NULL DEFAULT '',
    mr_title        TEXT NOT NULL DEFAULT '',
    mr_iid          INTEGER NOT NULL DEFAULT 0,
    project_id      INTEGER NOT NULL DEFAULT 0,
    author_email    TEXT NOT NULL DEFAULT '',
    raw_payload     JSONB NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_received_at ON events (received_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_issue_keys ON events USING GIN (issue_keys);
