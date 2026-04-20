<!-- Generated: 2026-04-16 | Packages: 8 | Refactor: 14→8 packages -->

# Architecture

**Last Updated:** 2026-04-16
**Entry Point:** `cmd/glsync/main.go`
**Module:** `github.com/mavolty/glsync`

## Package Map

```
cmd/glsync/
  main.go               wiring, signal handling, graceful shutdown

internal/
  config/               YAML + GLSYNC_ env var loader; Validate() fails fast
  domain/               ALL value types: NormalizedEvent, Job, AuditEntry, WorkflowState
  gitlab/               token validation, payload parsing, issue key extraction
                          parser.go  signature.go  types.go  issuekey.go
  workflow/             Classify(event->state), Resolver(state->transitionID), CalculateDueDate
                          rules.go  transition.go  duedate.go
  store/                pgx/v5 repositories: EventRepository, JobRepository, AuditRepository
                          postgres.go  event_repo.go  job_repo.go  audit_repo.go
  server/               chi HTTP router + handlers
                          server.go  webhook_handler.go  admin_handler.go
                          health_handler.go  middleware.go
  worker/               Processor (poll loop), Executor (Jira calls), Reconciler (stuck jobs)
                          processor.go  executor.go  reconciler.go
  integration/jira/     Jira REST API v2 client implementing jira.Transitioner
                          client.go  types.go
```

## System Diagram

```
GitLab --POST--> /api/v1/webhooks/gitlab
                        |
                   [server] :8090
                   validate token (gitlab.ValidateToken)
                   parse + normalise (gitlab.Parse)
                   extract issue keys (gitlab.IssueKeysFromEvent)
                   classify state (workflow.Classify)
                   persist event (store.EventRepository)
                   enqueue jobs (store.JobRepository)
                        |
               +--------+
               |
          [worker.Processor]
          poll every 5s
          SELECT FOR UPDATE SKIP LOCKED
          semaphore: worker.concurrency goroutines
               |
          [worker.Executor]
          jira_transition --> integration/jira.Client --> Jira REST API v2
               |
          [worker.Reconciler]
          every 15 min
          ResetStuck: running AND running_since < cutoff -> pending
```

## Data Flow (happy path)

```
POST /api/v1/webhooks/gitlab
  1. ValidateToken (X-Gitlab-Token header)
  2. gitlab.Parse -> NormalizedEvent
     +-- unrecognized type? -> 200 ignored
     +-- idempotency key exists? -> 200 already_processed
     +-- push but not new-branch and not develop/master? -> 200 ignored
  3. gitlab.IssueKeysFromEvent (branch -> title -> commit messages)
     +-- emoji_award? -> handleEmojiAward -> enqueue StateDone jobs -> 202
  4. workflow.Classify -> (WorkflowState, bool)
     +-- no action or no issue keys? -> 200 ignored
  5. store.EventRepository.Insert
  6. store.JobRepository.Enqueue x len(issueKeys) -> 202 Accepted

  ... async (worker.Processor) ...
  7. Dequeue -> Execute -> TransitionIssue / TransitionIssueWithFields
     +-- success        -> MarkCompleted + audit(transition_success)
     +-- non-retryable (400/404/422) -> MarkDead
     +-- retryable      -> MarkFailed + backoff (30s base, 30min cap, 2^n)
```

## Concurrency Model

- Three goroutines started from `main.go`: server, processor, reconciler
- Processor is semaphore-bounded (`worker.concurrency`, default 1)
- Job dequeue: `SELECT FOR UPDATE SKIP LOCKED` inside a transaction — no external broker
- All goroutines share a single `context.Context` cancelled on SIGINT/SIGTERM

## Event Types to Workflow States

| GitLab Event | Condition | Target State |
|---|---|---|
| push | new branch (before=0000...) | in_progress |
| push | branch = develop | rfqa |
| push | branch = master | done |
| mr_opened | non-draft | code_review |
| mr_merged | target = develop | rfqa |
| emoji_award | thumbsup on MR | done |
| mr_draft / unrecognized | — | ignored |

## Related Codemaps

- [backend.md](./backend.md) — HTTP routes and handler detail
- [data.md](./data.md) — database schema and job lifecycle
- [dependencies.md](./dependencies.md) — external services and Go modules
