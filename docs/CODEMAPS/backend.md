<!-- Generated: 2026-04-16 | Packages: 8 | Refactor: 14->8 packages -->

# Backend

**Last Updated:** 2026-04-16

## HTTP Routes

| Method | Path | Handler | Auth | Notes |
|--------|------|---------|------|-------|
| POST | `/api/v1/webhooks/gitlab` | `webhookHandler.handleGitLab` | X-Gitlab-Token header | Rate-limited: 100 req/min per IP |
| GET | `/healthz` | `healthHandler.liveness` | None | Always 200 |
| GET | `/readyz` | `healthHandler.readiness` | None | Pings DB pool |
| GET | `/api/v1/admin/events?limit=N` | `adminHandler.listEvents` | Bearer token | Default 50, max 500; 404 if token unset |
| GET | `/api/v1/admin/jobs/failed` | `adminHandler.listFailedJobs` | Bearer token | status IN (failed, dead) |

## Middleware Chain

```
RequestID -> Recoverer -> RequestLogger
  +-- webhook route: httprate.LimitByIP(100/min)
  +-- /api/v1/admin/*: RequireAdminToken (bearer token check)
```

## Handler to Store Mapping

```
webhookHandler.handleGitLab
  +-- gitlab.ValidateToken        (X-Gitlab-Token header)
  +-- gitlab.Parse                (raw JSON -> NormalizedEvent)
  +-- store.EventRepository.ExistsByIdempotencyKey
  +-- gitlab.IssueKeysFromEvent   (branch -> title -> commit messages)
  +-- workflow.Classify           (event -> target WorkflowState)
  +-- store.EventRepository.Insert
  +-- store.JobRepository.Enqueue (one job per issue key)

webhookHandler.handleEmojiAward   (emoji_award path inside handleGitLab)
  +-- checks event.EmojiName == cfg.DoneEmoji (default: "thumbsup")
  +-- store.EventRepository.Insert
  +-- store.JobRepository.Enqueue with StateDone

adminHandler.listEvents
  +-- store.EventRepository.ListRecent(limit)

adminHandler.listFailedJobs
  +-- store.JobRepository.ListFailed(limit)

healthHandler.readiness
  +-- pgxpool.Ping
```

## Workflow Classification

`workflow.Classify(event, RuleConfig)` in `internal/workflow/rules.go`:

```
EventPush
  source == develop  -> StateRFQA
  source == master   -> StateDone
  other              -> StateInProgress

EventMROpened        -> StateCodeReview

EventMRMerged
  target == develop  -> StateRFQA
  target == master   -> "" (ignored; done triggered by emoji)
  other              -> "" (ignored)

EventMRDraft         -> "" (ignored)
EventUnrecognized    -> "" (ignored)
```

Emoji award events bypass `Classify` entirely and always target `StateDone`.

## Transition Resolution

`workflow.Resolver.ResolveTransitionID(state)` in `internal/workflow/transition.go`:
- Looks up state string in `workflow.transitions` map from config
- Returns error if state not configured — executor logs a warning and skips the job
- `StateInProgress` is handled by `Executor.executeInProgressTransition` which also
  sets start/due date fields via `jira.TransitionIssueWithFields`

## Key Files

| File | Role |
|------|------|
| `internal/server/server.go` | chi router wiring, `NewHandler`, `New`, `Start`, `Shutdown` |
| `internal/server/webhook_handler.go` | main webhook path + `handleEmojiAward` |
| `internal/server/admin_handler.go` | `listEvents`, `listFailedJobs` |
| `internal/server/health_handler.go` | liveness + readiness |
| `internal/server/middleware.go` | `RequestLogger`, `RequireAdminToken` |
| `internal/gitlab/parser.go` | push / MR / emoji event parsing |
| `internal/gitlab/signature.go` | `ValidateToken` |
| `internal/gitlab/issuekey.go` | `IssueKeysFromEvent` regex extraction |
| `internal/workflow/rules.go` | `Classify` |
| `internal/workflow/transition.go` | `Resolver.ResolveTransitionID` |
| `internal/workflow/duedate.go` | `CalculateDueDate` (sprint_end / story_points) |
| `internal/worker/executor.go` | `Execute`, `executeInProgressTransition` |
| `internal/worker/processor.go` | poll loop, semaphore, exponential backoff |
| `internal/worker/reconciler.go` | `ResetStuck` loop |

## Interfaces

| Interface | Defined In | Key Methods |
|-----------|-----------|-------------|
| `store.EventRepository` | `internal/store/event_repo.go` | Insert, ExistsByIdempotencyKey, ListRecent |
| `store.JobRepository` | `internal/store/job_repo.go` | Enqueue, Dequeue, MarkCompleted, MarkFailed, MarkDead, ListFailed, ResetStuck |
| `store.AuditRepository` | `internal/store/audit_repo.go` | Insert |
| `jira.Transitioner` | `internal/integration/jira/client.go` | TransitionIssue, TransitionIssueWithFields, UpdateIssueFields, GetIssueStatus, GetStoryPoints, GetTransitions |

## Related Codemaps

- [architecture.md](./architecture.md) — full system diagram and data flow
- [data.md](./data.md) — database schema and job lifecycle
