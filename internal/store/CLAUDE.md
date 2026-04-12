[Root](../../../CLAUDE.md) > [internal](../../) > **store**

# internal/store

PostgreSQL repository implementations using pgx/v5. Exposes Go interfaces so callers never import pgx directly.

---

## Module Responsibility

Durable persistence for three entities:
- **events** — normalised GitLab webhook events (insert-only, idempotency-keyed)
- **jobs** — the async work queue (enqueue, dequeue with advisory lock, lifecycle transitions)
- **audit_logs** — append-only action log

---

## Interfaces

### EventRepository

```go
Insert(ctx, event domain.NormalizedEvent) error
ExistsByIdempotencyKey(ctx, key string) (bool, error)
GetByID(ctx, id string) (*domain.NormalizedEvent, error)
ListRecent(ctx, limit int) ([]domain.NormalizedEvent, error)
```

### JobRepository

```go
Enqueue(ctx, job domain.Job) error
Dequeue(ctx, batchSize int) ([]domain.Job, error)        // SELECT FOR UPDATE SKIP LOCKED
MarkCompleted(ctx, id string) error
MarkFailed(ctx, id, errMsg string, nextRunAt time.Time) error
MarkDead(ctx, id, errMsg string) error
ListFailed(ctx) ([]domain.Job, error)
ListStale(ctx, olderThan time.Duration) ([]domain.Job, error)
ResetStuck(ctx, olderThan time.Duration) (int64, error)  // used by reconciler
```

### AuditRepository

```go
Insert(ctx, entry domain.AuditEntry) error
```

---

## Concurrency Notes

`Dequeue` uses `SELECT … FOR UPDATE SKIP LOCKED` inside a single round-trip — no application-level locking is needed. Multiple `Processor` instances can safely run against the same database.

---

## Key Files

| File | Content |
|---|---|
| `postgres.go` | `NewPool` — creates and pings a `pgxpool.Pool` |
| `event_repo.go` | `pgEventRepo` — implements `EventRepository` |
| `job_repo.go` | `pgJobRepo` — implements `JobRepository` |
| `audit_repo.go` | `pgAuditRepo` — implements `AuditRepository`; `nullableString` helper |

---

## Data Model

See `migrations/` for authoritative DDL.

```
events
  id UUID PK, idempotency_key UNIQUE, event_type, issue_keys TEXT[], ...raw_payload JSONB

jobs
  id UUID PK, event_id FK, job_type, status, issue_key, target_state,
  attempts, max_attempts, next_run_at, completed_at

audit_logs
  id UUID PK, event_id FK, job_id FK, action, issue_key, detail JSONB
```

Key indexes: `idx_jobs_dequeue` (partial, filters pending/failed) drives the hot dequeue path.

---

## Tests & Quality

No automated tests in this package. Integration tests would require a live PostgreSQL instance. This is the primary coverage gap in the project.

Recommended approach: use `testcontainers-go` or `pgxmock` to add repository tests.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
