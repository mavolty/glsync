<!-- Generated: 2026-04-16 | Source: go.mod | Refactor: 14->8 packages -->

# Dependencies

**Last Updated:** 2026-04-16

## External Services

| Service | Direction | Purpose |
|---------|-----------|---------|
| GitLab (webhook) | Inbound | Push, MR open/merge, emoji award events via POST |
| Jira Cloud REST API v2 | Outbound | TransitionIssue, TransitionIssueWithFields, GetStoryPoints, GetIssueStatus |
| PostgreSQL 16 | Persistent store | events, jobs, audit_logs tables; used as durable job queue |

Note: GitLab API v4 client (`integration/gitlab`) was removed in this refactor.
Emoji events are now parsed directly from the webhook payload — no outbound GitLab API call.

## Go Module Dependencies (go.mod)

Module: `gitlab.surya-am.com/sam/risk/glsync`
Go version: `1.25`

### Direct

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/go-chi/chi/v5` | v5.2.5 | HTTP router |
| `github.com/go-chi/httprate` | v0.15.0 | IP-based rate limiting on webhook endpoint |
| `github.com/google/uuid` | v1.6.0 | UUID generation for event/job IDs |
| `github.com/jackc/pgx/v5` | v5.9.1 | PostgreSQL driver + connection pool |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions (assert, require) |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML config file parsing |

### Notable Indirect

| Package | Version | Purpose |
|---------|---------|---------|
| `golang.org/x/sync` | v0.17.0 | Sync primitives (used by pgx pool) |
| `github.com/jackc/puddle/v2` | v2.2.2 | Connection pool underlying pgx |
| `github.com/zeebo/xxh3` | v1.0.2 | Fast hashing (pgx dependency) |

Note: `github.com/knadh/koanf` is NOT a dependency. Config loading uses
`gopkg.in/yaml.v3` + manual `os.Getenv` env overrides in `internal/config/config.go`.

## Config Layering

```
config/glsync.yaml          (base defaults, checked into repo)
  |
  +-- GLSYNC_* env vars     (override individual fields at runtime)
      separator: __ (double underscore) for nested keys
      e.g. GLSYNC_JIRA__API_TOKEN -> jira.api_token
```

### Required env vars (binary exits if missing)

| Variable | Config key |
|----------|-----------|
| `GLSYNC_GITLAB__WEBHOOK_SECRET` | `gitlab.webhook_secret` |
| `GLSYNC_JIRA__USERNAME` | `jira.username` |
| `GLSYNC_JIRA__API_TOKEN` | `jira.api_token` |
| `GLSYNC_DATABASE__URL` | `database.url` |

### Optional env vars

| Variable | Config key | Default |
|----------|-----------|---------|
| `GLSYNC_SERVER__ADMIN_TOKEN` | `server.admin_token` | "" (admin routes return 404) |
| `GLSYNC_GITLAB__BASE_URL` | `gitlab.base_url` | "" |
| `GLSYNC_GITLAB__API_TOKEN` | `gitlab.api_token` | "" |
| `GLSYNC_JIRA__BASE_URL` | `jira.base_url` | "" |

## Removed Integrations (this refactor)

| Deleted Package | Was |
|----------------|-----|
| `internal/integration/gitlab` | GitLab API v4 client (emoji award counter) |
| `internal/integration/larkbase` | Stub deployment-record interface |
| `internal/integration/feishu` | Stub group-notification interface |
| `internal/extract` | Issue key regex — merged into `internal/gitlab/issuekey.go` |
| `internal/reconcile` | Reconciler loop — merged into `internal/worker/reconciler.go` |

## Related Codemaps

- [architecture.md](./architecture.md) — system diagram and component wiring
- [backend.md](./backend.md) — HTTP routes and handler detail
