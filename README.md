# glsync

[![CI](https://github.com/mavolty/glsync/actions/workflows/ci.yml/badge.svg)](https://github.com/mavolty/glsync/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Automate your GitLab → Jira workflow.** Push a branch → ticket moves to *In Progress*. Open a PR → *Code Review*. Merge → *RFQA*. React 👍 → *Done*. No manual updates, no webhooks to babysit.

---

> **Technical highlights**
> - PostgreSQL as a durable job queue using `SELECT FOR UPDATE SKIP LOCKED` — no Redis or Kafka needed
> - Exponential backoff retries with int64 overflow guard, capped at 30 minutes
> - Stuck-job reconciler resets orphaned `running` jobs every 15 minutes
> - Plugin extension interface: attach custom handlers (Slack, Feishu, LarkBase) without forking
> - 80%+ test coverage, race-detector clean, table-driven tests throughout
> - Single binary, ~10 MB Docker image (distroless), zero runtime dependencies

---

## How it works

```
GitLab  ──POST──►  /api/v1/webhooks/gitlab
                        │
                   [server]
                   validate token → parse event → extract issue keys
                   → persist event → enqueue jobs
                        │
              ┌─────────┘
              │
         [worker / Processor]        ← SELECT FOR UPDATE SKIP LOCKED
         dequeue jobs concurrently
              │
         [worker / Executor]
         └─ jira_transition  ──► Jira REST API v2
              │
         [reconciler]                ← runs every 15 min
         resets stuck "running" jobs
```

```mermaid
graph LR
    GL[GitLab Webhook] -->|POST| S[glsync Server]
    S -->|persist| DB[(PostgreSQL)]
    S -->|enqueue| DB
    W[Worker Pool] -->|SKIP LOCKED| DB
    W -->|transition| J[Jira REST API]
    R[Reconciler] -->|reset stuck| DB
```

---

## GitLab → Jira Event Mapping

| GitLab event | Condition | Jira transition |
|---|---|---|
| Push | New branch (`before == 0000…`) | → In Progress |
| MR opened / reopened | non-draft | → Code Review |
| MR merged | target = `develop` | → RFQA |
| MR merged | target = `master` | → Done |
| Emoji `👍` on MR | any merged MR with issue key | → Done |
| MR draft / unrecognised | — | ignored |

Issue keys are extracted from branch name → MR title → commit messages (first match wins, deduped).

---

## Quickstart

### Option A — Docker Compose (recommended for local dev)

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Copy and fill secrets
cp .env.example .env
# Required: GLSYNC_GITLAB__WEBHOOK_SECRET, GLSYNC_JIRA__USERNAME,
#            GLSYNC_JIRA__API_TOKEN, GLSYNC_DATABASE__URL

# 3. Set your Jira project key
# Edit config/glsync.yaml → workflow.project_key: "YOUR_PROJECT"

# 4. Apply migrations
make migrate-up

# 5. Run
make run
```

### Option B — Binary

```bash
# Prerequisites: Go 1.25+, PostgreSQL 16+, golang-migrate CLI

git clone https://github.com/mavolty/glsync.git
cd glsync
make build          # → bin/glsync
make dev-db         # start local postgres via Docker
make migrate-up
make run
```

### Option C — Production (systemd)

```bash
sudo bash deploy/setup.sh   # installs binary, creates DB, enables systemd service
```

The service listens on **port 8090** by default.

---

## Configuration

All settings live in `config/glsync.yaml`. Every key can be overridden by a
`GLSYNC_`-prefixed environment variable using `__` as a nesting separator.

### Required

| Variable | Description |
|---|---|
| `GLSYNC_GITLAB__WEBHOOK_SECRET` | Token GitLab sends in `X-Gitlab-Token` |
| `GLSYNC_JIRA__USERNAME` | Jira account email |
| `GLSYNC_JIRA__API_TOKEN` | Jira API token |
| `GLSYNC_DATABASE__URL` | PostgreSQL DSN (`postgres://user:pass@host/db`) |

### Optional

| Variable | Default | Description |
|---|---|---|
| `GLSYNC_SERVER__ADMIN_TOKEN` | `""` | Bearer token for `/api/v1/admin/*`; routes return 404 when unset |
| `GLSYNC_GITLAB__BASE_URL` | `""` | Your GitLab root URL |
| `GLSYNC_GITLAB__API_TOKEN` | `""` | GitLab personal access token |

### Workflow config (`config/glsync.yaml`)

```yaml
workflow:
  project_key: "YOUR_PROJECT"   # e.g. "PROJ", "ENG"
  develop_branch: "develop"
  master_branch: "master"
  transitions:
    # Discover IDs: curl -u EMAIL:TOKEN https://YOUR_JIRA/rest/api/2/issue/TICKET-1/transitions
    code_review: "14"
    rfqa: "15"
    done: "17"
  done_emoji: "thumbsup"
  in_progress:
    transition_id: "2"
    due_date_strategy: "sprint_end"   # or "story_points"
    sprint_end_weekday: "tuesday"
```

---

## Admin API

> Requires `Authorization: Bearer <GLSYNC_SERVER__ADMIN_TOKEN>`.
> All endpoints return 404 if the token is unset.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/admin/events?limit=N` | List recent events (default 50, max 500) |
| `GET` | `/api/v1/admin/jobs/failed` | List failed and dead jobs |
| `GET` | `/healthz` | Liveness probe (always 200) |
| `GET` | `/readyz` | Readiness probe (pings DB) |

```bash
curl -H "Authorization: Bearer $GLSYNC_SERVER__ADMIN_TOKEN" \
     http://localhost:8090/api/v1/admin/events | jq .
```

---

## Extending glsync

glsync exposes two plugin hooks for attaching custom behaviour without forking:

```go
// Register before the server starts
plugin.RegisterEventHandler(&myNotifier{})  // called after event persist
plugin.RegisterJobHandler(&myAlerter{})     // called after job execution
```

See **[docs/EXTENDING.md](docs/EXTENDING.md)** for the full interface reference and
the private-fork pattern used to build team-specific integrations on top of glsync.

---

## Development

```bash
make test       # go test ./... -race -count=1
make lint       # golangci-lint run ./...
make build      # compile → bin/glsync
make migrate-up # apply pending migrations
```

See **[CONTRIBUTING.md](CONTRIBUTING.md)** for the full contributing guide.

---

## Roadmap

- [x] Push / MR / Emoji event handling
- [x] PostgreSQL job queue with `SELECT FOR UPDATE SKIP LOCKED`
- [x] Exponential backoff retries (capped at 30 min)
- [x] Stuck-job reconciler
- [x] Plugin extension interface
- [x] GitHub Actions CI
- [ ] Store integration tests via `testcontainers-go`
- [ ] GoReleaser pre-built binaries on GitHub Releases
- [ ] Prometheus metrics endpoint (`/metrics`)
- [ ] Webhook event replay from admin API
- [ ] Multi-project support (multiple `project_key` configs)

---

## Architecture

See **[docs/CODEMAPS/architecture.md](docs/CODEMAPS/architecture.md)** for a detailed
breakdown of each package, data flow, and database schema.

---

## License

MIT — see [LICENSE](LICENSE).

---

*If glsync saves you time, a ⭐ on GitHub is appreciated.*
