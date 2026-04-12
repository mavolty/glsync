[Root](../../../CLAUDE.md) > [internal](../../) > **config**

# internal/config

Layered configuration loader. Reads a YAML file then overrides any value with `GLSYNC_`-prefixed environment variables.

---

## Module Responsibility

Provide a single `*Config` struct to the entry point. Hide the koanf loading mechanics from all other packages.

---

## Config Sections

| Struct | YAML prefix | Key fields |
|---|---|---|
| `ServerConfig` | `server` | `port` (8090), `read_timeout`, `write_timeout`, `shutdown_timeout` |
| `DatabaseConfig` | `database` | `url`, `max_connections` (20), `min_connections` (5) |
| `GitLabConfig` | `gitlab` | `webhook_secret` |
| `JiraConfig` | `jira` | `base_url`, `username`, `api_token`, `timeout` (15s) |
| `WorkflowConfig` | `workflow` | `project_key`, `develop_branch`, `master_branch`, `transitions` map, `in_progress` |
| `InProgressConfig` | `workflow.in_progress` | `transition_id`, `due_date_strategy`, `sprint_end_weekday`, `start_date_field`, `due_date_field`, `story_points_field` |
| `WorkerConfig` | `worker` | `concurrency` (3), `poll_interval` (5s), `max_attempts` (5) |
| `ReconcileConfig` | `reconcile` | `enabled`, `interval` (15m), `stuck_job_timeout` (5m), `drift_lookback` (1h) |

## Env-var Override Mapping

Prefix `GLSYNC_`, then lowercase and replace `_` with `.` to get the koanf path.

Examples:
- `GLSYNC_SERVER_PORT` → `server.port`
- `GLSYNC_JIRA_API_TOKEN` → `jira.api_token`
- `GLSYNC_DATABASE_URL` → `database.url`

---

## Key Files

| File | Content |
|---|---|
| `config.go` | `Config` and all sub-structs, `Load`, `defaults`, `replaceEnvKey` |

---

## Tests & Quality

No tests. The config loader is straightforward koanf wiring; the main risk is misconfigured secrets (missing API tokens). Startup validation (checking that required secrets are non-empty) would be a valuable addition.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
