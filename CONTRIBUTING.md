# Contributing to glsync

**Last Updated:** 2026-04-17

## Prerequisites

- Go 1.25+
- Docker (for local PostgreSQL and Docker image builds)
- [`golang-migrate` CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) — required for `make migrate-*` targets
- [`golangci-lint`](https://golangci-lint.run/usage/install/) — required for `make lint`

## Local Dev Setup

```bash
# 1. Clone the repo
git clone https://github.com/mavolty/glsync.git
cd glsync

# 2. Start a local PostgreSQL 16 instance (Docker required)
make dev-db

# 3. Copy the example env file and fill in the required values
cp .env.example .env
# Edit .env — at minimum set GLSYNC_GITLAB__WEBHOOK_SECRET,
# GLSYNC_JIRA__USERNAME, and GLSYNC_JIRA__API_TOKEN

# 4. Apply database migrations
make migrate-up

# 5. Run the service
make run
```

The service listens on **port 8090** by default. Config file: `config/glsync.yaml`.

## Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Compile binary to `bin/glsync` |
| `make run` | Build then run with `config/glsync.yaml` |
| `make test` | Run all tests with the race detector (`go test ./... -race -count=1`) |
| `make lint` | Run `golangci-lint run ./...` |
| `make tidy` | Run `go mod tidy` |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |
| `make migrate-status` | Print the current migration version |
| `make dev-db` | Start a local `postgres:16-alpine` container named `glsync-pg` |
| `make docker-build` | Build a Docker image tagged `glsync:latest` |

`DB_URL` defaults to `postgres://glsync:glsync@localhost:5432/glsync?sslmode=disable` and can be overridden on the command line or via `.env`.

## Environment Variables

All variables use the `GLSYNC_` prefix with `__` (double underscore) as the nested-key separator, overriding the corresponding key in `config/glsync.yaml`.

### Required (binary exits at startup if missing)

| Variable | Config key | Example |
|----------|-----------|---------|
| `GLSYNC_GITLAB__WEBHOOK_SECRET` | `gitlab.webhook_secret` | `s3cr3t` |
| `GLSYNC_JIRA__USERNAME` | `jira.username` | `user@corp.com` |
| `GLSYNC_JIRA__API_TOKEN` | `jira.api_token` | `ATATT3x…` |
| `GLSYNC_DATABASE__URL` | `database.url` | `postgres://glsync:glsync@localhost:5432/glsync?sslmode=disable` |

### Optional

| Variable | Config key | Default | Notes |
|----------|-----------|---------|-------|
| `GLSYNC_GITLAB__BASE_URL` | `gitlab.base_url` | `""` | Not required for webhook-only operation |
| `GLSYNC_GITLAB__API_TOKEN` | `gitlab.api_token` | `""` | Not used in the current emoji flow |
| `GLSYNC_SERVER__ADMIN_TOKEN` | `server.admin_token` | `""` | Admin routes return 404 when unset |

## Testing

```bash
make test
```

Tests are table-driven and use `testify`. They live alongside source files in `_test` packages.

### Covered packages

| Package | What is tested |
|---------|---------------|
| `internal/config` | Config loading, env override, validation |
| `internal/gitlab` | Token validation, push/MR/emoji parsing, issue key extraction (regex, dedup, 3-level fallback) |
| `internal/workflow` | State classification, transition ID resolution, due-date strategies |
| `internal/worker` | Executor (all job types, mock Jira client); processor helpers (`NewJob`, `BackoffForAttempt`); reconciler lifecycle (`ResetStuck` calls, error tolerance, audit writes) |
| `internal/server` | Webhook handler integration tests via `httptest` (auth, idempotency, filtering, emoji, happy path) |

### Known gap

`internal/store` requires a live PostgreSQL instance and has no tests yet. Use `testcontainers-go` or `pgxmock` to add coverage.

## Code Standards

- Format with `gofmt` / `goimports` before committing.
- Run `make lint` and address any findings before opening a PR.
- Wrap errors at every boundary: `fmt.Errorf("context: %w", err)`.
- Define interfaces at the call site, not the implementation site.
- Do not hard-code secrets; load from environment only.

## Commit Message Format

```
<type>: <description>
```

Types: `feat`, `fix`, `refactor`, `chore`, `perf`, `ci`, `docs`, `test`

## Adding a New Event Type

When a new GitLab event type needs to be handled, update these files in order:

1. `internal/domain/domain.go` — add the new `EventType` constant
2. `internal/gitlab/parser.go` — parse the raw payload into `NormalizedEvent`
3. `internal/workflow/rules.go` — map the event to a `WorkflowState`
4. Tests for all three layers

## Database Migrations

Migrations live in `migrations/` and are managed by `golang-migrate`.

```bash
make migrate-up      # apply pending
make migrate-down    # roll back one step
make migrate-status  # current version
```

File naming convention: `NNN_description.up.sql` / `NNN_description.down.sql`.
