# glsync

GitLab-to-Jira synchronisation service. Listens for GitLab webhook events (push and merge-request), extracts Jira issue keys from branch names and MR titles, then drives the target Jira ticket through its workflow states. Supports async job processing with exponential-backoff retries, a stuck-job reconciler, and an optional in-progress date strategy.

---

## Architecture Overview

```
GitLab  ──POST──►  /api/v1/webhooks/gitlab
                        │
                   [server]
                   validate token
                   parse + normalise event
                   extract issue keys
                   classify workflow state
                   persist event
                   enqueue jobs
                        │
              ┌─────────┘
              │
         [worker / Processor]
         dequeue (SELECT FOR UPDATE SKIP LOCKED)
              │
         [worker / Executor]
         ├─ jira_transition  ──► Jira REST API v2
         ├─ larkbase_sync    (stub, no-op)
         └─ feishu_notify    (stub, no-op)
              │
         [reconcile / Reconciler]
         resets stuck "running" jobs every 15 min
```

Key design choices:
- Single binary, no external message broker — PostgreSQL is the durable job queue.
- All downstream logic is isolated behind interfaces (`store.EventRepository`, `store.JobRepository`, `jira.Transitioner`), making each layer independently testable.
- Configuration is layered: YAML file, then `GLSYNC_`-prefixed env var overrides (via koanf).

---

## Module Structure

```mermaid
graph TD
    A["(root) glsync"] --> B["cmd/glsync"]
    A --> C["internal"]
    C --> D["config"]
    C --> E["domain"]
    C --> F["gitlab"]
    C --> H["workflow"]
    C --> I["store"]
    C --> J["server"]
    C --> K["worker"]
    C --> M["integration"]
    M --> N["jira"]
    A --> R["migrations"]
```

---

## Module Index

| Path | Responsibility |
|------|---------------|
| `cmd/glsync` | Binary entry point — wires all components and starts the server |
| `internal/config` | Layered config loader (YAML + env var overrides); `Validate()` fails fast on missing required fields |
| `internal/domain` | Pure value types: `NormalizedEvent`, `Job`, `AuditEntry`, `WorkflowState` |
| `internal/gitlab` | Webhook token validation, payload parsing (push / MR / emoji), issue key extraction from branch/title/commits |
| `internal/workflow` | Classify event → target state; resolve state → Jira transition ID; calculate due dates |
| `internal/store` | pgx/v5 repository implementations for events, jobs, and audit logs |
| `internal/server` | chi HTTP router, webhook handler, health/readiness, admin list endpoints |
| `internal/worker` | Concurrent job processor (poll + semaphore), executor that calls Jira, reconciler that resets stuck jobs |
| `internal/integration/jira` | Jira REST API v2 client: `TransitionIssue`, `TransitionIssueWithFields`, `GetStoryPoints`, etc. |
| `migrations` | golang-migrate SQL files for `events`, `jobs`, `audit_logs` tables |

---

## HTTP Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/v1/webhooks/gitlab` | Receive GitLab webhook events |
| `GET` | `/healthz` | Liveness probe (always 200) |
| `GET` | `/readyz` | Readiness probe (pings DB) |
| `GET` | `/api/v1/admin/events?limit=N` | List recent events (default 50, max 500) |
| `GET` | `/api/v1/admin/jobs/failed` | List failed and dead jobs |

> Admin endpoints require bearer token auth (`GLSYNC_SERVER__ADMIN_TOKEN`). Returns 404 if token is empty.

---

## Webhook → Jira State Mapping

| GitLab Event | Condition | Jira Transition |
|---|---|---|
| Push (`object_kind: push`) | New branch only (`before == 0000…`) | `in_progress` |
| MR opened / reopened / update (non-draft) | — | `code_review` |
| MR merged | target branch = `develop` | `rfqa` |
| MR merged | target branch = `master` | `done` |
| Emoji award (`object_kind: emoji`) | `thumbsup` on MR with issue key | `done` |
| MR draft / unrecognised | — | ignored |

---

## Running & Development

```bash
# Prerequisites: Go 1.25, PostgreSQL 16

# 1. Start local DB
make dev-db

# 2. Copy and edit secrets
cp .env.example .env
# fill GLSYNC_GITLAB_WEBHOOK_SECRET, GLSYNC_JIRA_USERNAME, GLSYNC_JIRA_API_TOKEN

# 3. Apply migrations
make migrate-up

# 4. Run
make run

# 5. Build binary
make build

# 6. Build Docker image
make docker-build
```

Default port: **8090**. Config file: `config/glsync.yaml`.

### Make Targets

| Command | Description |
|---------|-------------|
| `make build` | Compile binary to `bin/glsync` |
| `make run` | Build + run with config file |
| `make test` | Run all tests with race detector |
| `make lint` | Run `golangci-lint` |
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-status` | Show current migration version |
| `make dev-db` | Start local PostgreSQL 16 via Docker |
| `make docker-build` | Build Docker image |
| `make tidy` | Run `go mod tidy` |

### Environment Variables (override YAML)

| Variable | YAML key | Required | Example |
|---|---|---|---|
| `GLSYNC_GITLAB__WEBHOOK_SECRET` | `gitlab.webhook_secret` | **YES** | `s3cr3t` |
| `GLSYNC_GITLAB__BASE_URL` | `gitlab.base_url` | no | `https://gitlab.example.com` |
| `GLSYNC_GITLAB__API_TOKEN` | `gitlab.api_token` | no | `glpat-…` |
| `GLSYNC_JIRA__USERNAME` | `jira.username` | **YES** | `user@corp.com` |
| `GLSYNC_JIRA__API_TOKEN` | `jira.api_token` | **YES** | `ATATT3x…` |
| `GLSYNC_DATABASE__URL` | `database.url` | **YES** | `postgres://…` |
| `GLSYNC_SERVER__ADMIN_TOKEN` | `server.admin_token` | no | `random-secret` |

> **Note:** The binary calls `cfg.Validate()` at startup and exits immediately if any required variable is missing.

---

## Testing Strategy

Tests live alongside source in the same package (external `_test` packages). All use table-driven patterns with `testify`.

```bash
make test           # go test ./... -race -count=1
```

Covered packages (unit / integration tests):
- `internal/gitlab` — webhook token validation, issue key extraction (regex, deduplication, 3-level fallback), emoji event parsing (award, revoke, non-MR ignored)
- `internal/workflow` — state classification, transition resolver, due-date strategies
- `internal/worker` — executor unit tests (mock Jira client): all job types, unconfigured state skip, error propagation; processor (`NewJob`, `BackoffForAttempt`, cap); reconciler lifecycle (calls `ResetStuck`, tolerates errors, writes audit entries)
- `internal/server` — webhook handler integration tests (httptest): auth, idempotency, filtering, emoji handling, happy path
- `internal/config` — config loading, validation, defaults

Known gaps (not yet tested):
- `internal/store` — requires a live or Dockerised PostgreSQL instance; use `testcontainers-go` or `pgxmock`

---

## Coding Standards

- Language: Go 1.25. Module: `gitlab.surya-am.com/sam/risk/glsync`.
- Formatter: `gofmt` / `goimports` (mandatory).
- Linter: `golangci-lint` (`make lint`).
- Error wrapping: `fmt.Errorf("context: %w", err)` at every boundary.
- Interfaces defined at call site (e.g. `jira.Transitioner`, `store.JobRepository`).
- No mutation of shared state; concurrency via channels and `sync.WaitGroup`.
- Secrets: never hard-code; load from env only.

---

## AI Usage Guidelines

- The domain model (`internal/domain`) is the authoritative source of type names — always check it before generating new types.
- Jira transition IDs are environment-specific integers stored in config; do not hard-code them.
- When adding a new event type, update: `domain/event.go` → `gitlab/parser.go` → `workflow/rules.go` → tests.
- The `Dequeue` query uses `SELECT FOR UPDATE SKIP LOCKED` — do not add locks elsewhere in the job lifecycle.

---

## Production Checklist

Before exposing to the internet:
- [ ] Set all four required env vars (`WEBHOOK_SECRET`, `JIRA_USERNAME`, `JIRA_API_TOKEN`, `DATABASE_URL`)
- [ ] Set `GLSYNC_SERVER__ADMIN_TOKEN` or confirm `/api/v1/admin/*` is not reachable externally
- [ ] Fill `workflow.transitions.done` once the Jira RFQA→Done transition ID is known
  - Discover it: `GET <jira_base_url>/rest/api/2/issue/<rfqa-ticket>/transitions`
- [ ] Apply all migrations: `make migrate-up`
- [ ] Run `make test` — all tests must pass before deploying

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-17 | Simplified from 14 → 8 packages: merged `internal/extract` into `internal/gitlab/issuekey.go`; merged `internal/reconcile` into `internal/worker/reconciler.go`; removed `integration/gitlab`, `integration/larkbase`, `integration/feishu`; added reconciler and processor tests; updated CLAUDE.md and docs/CONTRIBUTING.md |
| 2026-04-16 | Emoji award webhook: parse emoji events, thumbsup triggers done without merge-to-master; removed emoji threshold/counter; added `integration/gitlab` API client |
| 2026-04-16 | Removed all sub-module CLAUDE.md files; updated root CLAUDE.md to reflect current state |
| 2026-04-16 | Added Slidev presentation in `slides/` |
| 2026-04-12 | Production hardening: config validation, admin bearer auth, executor empty-ID guard, executor & webhook handler tests |
| 2026-04-08 | Initial CLAUDE.md generated by architecture scan |
