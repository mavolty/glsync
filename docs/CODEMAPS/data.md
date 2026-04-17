<!-- Generated: 2026-04-16 | Migrations: 6 | Refactor: 14->8 packages -->

# Data

**Last Updated:** 2026-04-16

## Tables

### `events`
Normalised GitLab webhook events. One row per webhook delivery.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | gen_random_uuid() |
| idempotency_key | TEXT UNIQUE | Prevents duplicate processing |
| event_type | TEXT CHECK | push, mr_opened, mr_merged, mr_draft, emoji_award, unrecognized |
| issue_keys | TEXT[] | GIN-indexed; extracted Jira keys |
| source_branch | TEXT | Branch name (push) or MR source branch |
| target_branch | TEXT | MR target branch |
| mr_title | TEXT | MR title |
| mr_iid | INTEGER | GitLab MR internal ID |
| project_id | INTEGER | GitLab project ID |
| author_email | TEXT | Event author |
| raw_payload | JSONB | Full webhook body |
| received_at | TIMESTAMPTZ | When glsync received it |

Indexes: `idx_events_received_at` (DESC), `idx_events_issue_keys` (GIN).

### `jobs`
Durable work queue — one row per (event, issue_key) pair.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| event_id | UUID FK -> events | |
| job_type | TEXT CHECK | `jira_transition` only (migration 005 removed stubs) |
| status | TEXT CHECK | pending, running, completed, failed, dead |
| issue_key | TEXT | e.g. RIS-123 |
| target_state | TEXT | in_progress, code_review, rfqa, done |
| payload | JSONB | Reserved (currently `{}`) |
| attempts | INTEGER | Current retry count |
| max_attempts | INTEGER | Default 5 (configurable) |
| last_error | TEXT | Last failure message |
| next_run_at | TIMESTAMPTZ | When worker may claim it |
| running_since | TIMESTAMPTZ | Nullable; set on dequeue, cleared on terminal state |
| created_at | TIMESTAMPTZ | |
| completed_at | TIMESTAMPTZ | Nullable |

Indexes: `idx_jobs_dequeue` (partial, status IN pending/failed), `idx_jobs_event_id`,
`idx_jobs_issue_key`, `idx_jobs_active_status` (partial, non-terminal).

### `audit_logs`
Append-only action trail written by processor and reconciler.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| event_id | UUID | Nullable FK -> events |
| job_id | UUID | Nullable FK -> jobs |
| action | TEXT | transition_success, transition_failed, job_dead, reconcile_stuck_reset |
| issue_key | TEXT | |
| detail | JSONB | Structured context (target_state, error, attempt, count, etc.) |
| created_at | TIMESTAMPTZ | |

Indexes: `idx_audit_logs_event_id`, `idx_audit_logs_issue_key`, `idx_audit_logs_created_at` (DESC).

## Job Lifecycle

```
             enqueue
               |
           [pending]
               |
        Dequeue (FOR UPDATE SKIP LOCKED)
               |
           [running]  <--- running_since set here
               |
       +-------+----------+------------------+
       |                  |                  |
  Execute OK         retryable err     non-retryable err
       |             OR attempts        (400/404/422)
  [completed]        < max_attempts    OR attempts >= max
                          |                  |
                      [failed]            [dead]
                      next_run_at        (terminal)
                      = now + backoff
                          |
                     back to poll

  Reconciler (every 15 min):
    running AND running_since < cutoff -> [pending]
```

Backoff formula: `min(2^attempts * 30s, 30min)`

## Key Queries

```sql
-- Dequeue (atomic claim in single transaction)
SELECT ... FROM jobs
WHERE status IN ('pending', 'failed') AND next_run_at <= now()
ORDER BY next_run_at ASC LIMIT $1
FOR UPDATE SKIP LOCKED;

UPDATE jobs SET status = 'running', running_since = now()
WHERE id = ANY($1);

-- Reconciler: reset stuck jobs
UPDATE jobs SET status = 'pending', next_run_at = now(), running_since = NULL
WHERE status = 'running' AND running_since < $1;
```

## Migration History

| # | File | Change |
|---|------|--------|
| 001 | `001_create_events.up.sql` | Create events table |
| 002 | `002_create_jobs.up.sql` | Create jobs table with dequeue partial index |
| 003 | `003_create_audit_logs.up.sql` | Create audit_logs table |
| 004 | `004_add_running_since.up.sql` | Add running_since column to jobs |
| 005 | `005_remove_stub_job_types.up.sql` | Constrain job_type to jira_transition only |
| 006 | `006_add_emoji_award_event_type.up.sql` | Add emoji_award to event_type CHECK constraint |

## Related Codemaps

- [architecture.md](./architecture.md) — system diagram and data flow
- [backend.md](./backend.md) — handler-to-store mapping
