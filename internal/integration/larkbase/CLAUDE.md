[Root](../../../../CLAUDE.md) > [internal](../../../) > [integration](../) > **larkbase**

# internal/integration/larkbase

Stub package for future Lark Base (deployment-tracking spreadsheet) integration.

---

## Module Responsibility

Define the `Syncer` interface and provide a `NoopSyncer` no-op implementation. The `larkbase_sync` job type is registered in the domain but the executor currently returns nil without doing anything.

---

## Interface

```go
type Syncer interface {
    SyncDeployment(ctx context.Context, record DeploymentRecord) error
}
```

`DeploymentRecord` captures deployment metadata: Jira key, MR link, source/target branch, migration flags, risk notes, etc.

---

## Status

This is an intentional stub. Do not implement until the deployment-tracking feature is scheduled. The `NoopSyncer` keeps compilation and the job dispatch path functional in the meantime.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
