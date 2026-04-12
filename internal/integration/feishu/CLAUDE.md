[Root](../../../../CLAUDE.md) > [internal](../../../) > [integration](../) > **feishu**

# internal/integration/feishu

Stub package for future Feishu/Lark group notification integration.

---

## Module Responsibility

Define the `Notifier` interface and provide a `NoopNotifier` no-op implementation. The `feishu_notify` job type is registered in the domain but the executor currently returns nil without calling anything.

---

## Interface

```go
type Notifier interface {
    Send(ctx context.Context, n Notification) error
}
```

`Notification` carries `GroupID`, `Title`, `Body`, `IssueKey`, and `MRLink`.

---

## Status

Intentional stub. Do not implement until the Feishu notification feature is scheduled.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
