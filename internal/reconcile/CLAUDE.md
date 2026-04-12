[Root](../../../CLAUDE.md) > [internal](../../) > **reconcile**

# internal/reconcile

Background loop that detects and repairs drift between expected and actual job states. Currently implements one repair action: reset jobs that are stuck in `running` status.

---

## Module Responsibility

A job enters `running` when `Dequeue` claims it but may never exit that status if the process crashes mid-execution. The reconciler periodically calls `JobRepository.ResetStuck` to move such jobs back to `pending` so they are re-tried.

---

## Configuration

```go
type ReconcileConfig struct {
    Interval        time.Duration // how often to run (default 15m)
    StuckJobTimeout time.Duration // age threshold for "stuck" (default 5m)
    DriftLookback   time.Duration // reserved for future drift detection (default 1h)
}
```

Enabled/disabled via `reconcile.enabled` in config. Controlled in `cmd/glsync/main.go`.

---

## Key Files

| File | Content |
|---|---|
| `reconciler.go` | `Reconciler`, `ReconcileConfig`, `New`, `Run`, `runOnce`, `resetStuckJobs`, `writeAudit` |

---

## Audit Actions Written

| Action | Trigger |
|---|---|
| `reconcile_stuck_reset` | When one or more stuck jobs are reset; `detail.count` = number affected |

---

## Tests & Quality

No tests in this package. A unit test for `runOnce` with a mock `JobRepository` would cover the primary path. The `DriftLookback` field is reserved but unused — future drift detection (e.g., jobs that succeeded in DB but did not update Jira) would use it.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
