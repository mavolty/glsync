[Root](../../../CLAUDE.md) > [internal](../../) > **server**

# internal/server

chi-based HTTP server. Mounts all routes, owns middleware, and hosts three handler groups: webhook receiver, health probes, and admin inspection endpoints.

---

## Module Responsibility

- Validate incoming GitLab webhooks (token, body size, parse).
- Implement the full 10-step accept pipeline: validate → parse → idempotency check → new-branch filter → extract keys → classify state → persist event → enqueue jobs.
- Expose `/healthz` (liveness) and `/readyz` (DB ping readiness).
- Expose read-only admin endpoints for operational visibility.

---

## Routes

| Method | Path | Handler |
|---|---|---|
| `POST` | `/api/v1/webhooks/gitlab` | `webhookHandler.handleGitLab` |
| `GET` | `/healthz` | `healthHandler.liveness` |
| `GET` | `/readyz` | `healthHandler.readiness` |
| `GET` | `/api/v1/admin/events` | `adminHandler.listEvents` |
| `GET` | `/api/v1/admin/jobs/failed` | `adminHandler.listFailedJobs` |

Middleware stack (in order): `chi/middleware.RequestID` → `chi/middleware.Recoverer` → `RequestLogger`.

---

## Webhook Accept Pipeline (`webhook_handler.go`)

1. Validate `X-Gitlab-Token` header.
2. Read body, limit to 5 MB.
3. Parse into `domain.NormalizedEvent` via `gitlab.Parse`.
4. Drop `unrecognized` events silently (200 `{"status":"ignored"}`).
5. Idempotency check — return `already_processed` if key exists.
6. For push events, drop if `before != zeroSHA` (not a new branch).
7. Extract issue keys (`extract.IssueKeysFromBranchAndTitle`).
8. Classify → target state (`workflow.Classify`).
9. Drop if no issue keys or no actionable state.
10. Persist event, enqueue one job per issue key; respond 202.

---

## Key Files

| File | Content |
|---|---|
| `server.go` | `Server`, `New`, `Start`, `Shutdown` |
| `webhook_handler.go` | `webhookHandler.handleGitLab` — accept pipeline |
| `health_handler.go` | `healthHandler.liveness`, `healthHandler.readiness` |
| `admin_handler.go` | `adminHandler.listEvents`, `adminHandler.listFailedJobs`, `writeJSON`, `writeError` |
| `middleware.go` | `RequestLogger` — structured slog request logging |

---

## Tests & Quality

No tests in this package. Integration tests covering the full handler pipeline (mocked repositories, table-driven request scenarios) are the highest-value gap to close.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
