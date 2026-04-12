[Root](../../../CLAUDE.md) > [internal](../../) > **worker**

# internal/worker

Background job processor and dispatcher. Polls the jobs table, executes Jira transitions, handles exponential-backoff retries, and writes audit records for every outcome.

---

## Module Responsibility

- `Processor`: polls `JobRepository.Dequeue` on a configurable interval, dispatches each job to `Executor` in a goroutine pool bounded by a semaphore.
- `Executor`: routes by `domain.JobType`; for `jira_transition` calls the Jira client; stubs `larkbase_sync` and `feishu_notify`.
- Retry logic with exponential backoff: `min(2^attempts * 30s, 30min)`.
- Marks jobs `completed`, `failed` (with `next_run_at`), or `dead` (exhausted retries).

---

## Key Types

### ProcessorConfig

```go
type ProcessorConfig struct {
    Concurrency  int           // goroutine pool size (default 3)
    PollInterval time.Duration // dequeue tick rate (default 5s)
    MaxAttempts  int           // per-job retry limit (default 5)
}
```

### Backoff

```
attempt 0 → 30s
attempt 1 → 1m
attempt 2 → 2m
attempt 3 → 4m
attempt 4 → dead (max_attempts=5)
cap at 30 minutes
```

`BackoffForAttempt(attempt int)` is exported for tests and the reconciler.

---

## In-Progress Transition (`executor.go`)

When `job.TargetState == StateInProgress`, the executor:
1. Optionally fetches story points from Jira (`GetStoryPoints`) if strategy is `story_points`.
2. Computes `dueDate` and `startDate` via `workflow.CalculateDueDate`.
3. Calls `jira.TransitionIssueWithFields` with the date custom fields.

If `InProgressConfig.TransitionID` is empty, the transition is silently skipped (safe default for incomplete config).

---

## Key Files

| File | Content |
|---|---|
| `processor.go` | `Processor`, `ProcessorConfig`, `NewProcessor`, `Run`, backoff helpers, `NewJob` |
| `executor.go` | `Executor`, `NewExecutor`, `Execute`, `executeJiraTransition`, `executeInProgressTransition` |

---

## Tests & Quality

No tests in this package. Priority items to add:
- Unit test for `nextBackoff` / `BackoffForAttempt`.
- Unit test for `Executor.Execute` with a mock `jira.Transitioner`.
- Integration test for `Processor.Run` lifecycle with a mock `JobRepository`.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
