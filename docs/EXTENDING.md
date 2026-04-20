# Extending glsync

glsync ships with a lightweight plugin system that lets you attach custom behaviour
without forking the core service. Two hook points are available:

| Hook | When it fires | Common use cases |
|------|--------------|-----------------|
| `EventHandler` | After a webhook event is persisted, before jobs are enqueued | Custom notifications (Slack, Feishu, Lark), analytics, audit |
| `JobHandler` | After a job is executed (success **or** failure) | Custom alerting, metrics push, dead-letter forwarding |

---

## Quick Start

### 1. Import the plugin package

```go
import "github.com/mavolty/glsync/plugin"
```

### 2. Implement an interface

```go
// notifier.go
package myext

import (
    "context"
    "fmt"

    "github.com/mavolty/glsync/internal/domain"
)

type SlackNotifier struct {
    webhookURL string
}

func (n *SlackNotifier) HandleEvent(ctx context.Context, event domain.NormalizedEvent) error {
    fmt.Printf("[slack] event %s for keys %v\n", event.EventType, event.IssueKeys)
    // call your Slack webhook here
    return nil
}
```

### 3. Register before the server starts

```go
package main

import (
    "github.com/mavolty/glsync/plugin"
    "github.com/your-org/glsync-ext/myext"
)

func main() {
    plugin.RegisterEventHandler(&myext.SlackNotifier{
        webhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
    })

    // ... rest of glsync startup
}
```

---

## Interface Reference

```go
// EventHandler is called after a webhook event is parsed and persisted,
// before jobs are enqueued. A non-nil error logs a warning but does NOT
// abort job processing. Implementations must be safe for concurrent use.
type EventHandler interface {
    HandleEvent(ctx context.Context, event domain.NormalizedEvent) error
}

// JobHandler is called after a job is executed (success or failure).
// execErr is nil on success. Implementations must be safe for concurrent use.
type JobHandler interface {
    HandleJobResult(ctx context.Context, job domain.Job, execErr error)
}
```

---

## Private Fork Pattern

The recommended approach for team-specific extensions:

```
github.com/your-org/glsync-ext/   ← private repo
├── main.go                        ← replaces glsync's main.go; calls Register*
├── go.mod                         ← imports github.com/mavolty/glsync
└── internal/
    ├── feishu/   ← your Feishu notifier
    └── lark/     ← your LarkBase sync
```

`go.mod` of the private fork:

```
module github.com/your-org/glsync-ext

require github.com/mavolty/glsync v0.1.0
```

`main.go` only needs two extra lines compared to upstream:

```go
plugin.RegisterEventHandler(&feishu.Notifier{...})
plugin.RegisterJobHandler(&lark.Syncer{...})
```

Everything else — config, DB, HTTP server, processor, reconciler — is imported unchanged.

---

## Notes

- Handlers are called **synchronously** in registration order.
- Handler errors are **logged as warnings** and do not abort the main processing flow.
- Handlers registered via `init()` are safe — Go guarantees all `init()` calls complete before `main()` runs.
- Handler slices are written only at startup and never mutated at runtime, so concurrent reads from worker goroutines are safe without a mutex.
