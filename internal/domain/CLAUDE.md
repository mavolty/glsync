[Root](../../CLAUDE.md) > [internal](../) > **domain**

# internal/domain

Pure value types shared across all internal packages. Contains no logic, no I/O, and no external imports beyond the standard library.

---

## Module Responsibility

Define the canonical data structures that flow through the application. All other packages depend on this package; this package depends on nothing internal.

---

## Types

### NormalizedEvent (`event.go`)

Transport-agnostic representation of a GitLab webhook event after parsing.

| Field | Type | Notes |
|---|---|---|
| `ID` | `string` | UUID assigned at accept time |
| `IdempotencyKey` | `string` | Unique key derived from event shape (e.g. `push:{projectID}:{branch}:{before}`) |
| `EventType` | `EventType` | One of `push`, `mr_opened`, `mr_merged`, `mr_draft`, `unrecognized` |
| `IssueKeys` | `[]string` | Jira keys extracted from branch/title (e.g. `["RIS-123"]`) |
| `SourceBranch` | `string` | |
| `TargetBranch` | `string` | Populated for MR events |
| `MRTitle` | `string` | |
| `MRIID` | `int` | MR internal ID within the project |
| `ProjectID` | `int` | GitLab project ID |
| `AuthorEmail` | `string` | |
| `RawPayload` | `json.RawMessage` | Original webhook body, stored verbatim |
| `ReceivedAt` | `time.Time` | |

### Job (`job.go`)

A unit of work enqueued for async execution.

| Field | Notes |
|---|---|
| `Type` | `jira_transition`, `larkbase_sync`, `feishu_notify` |
| `Status` | `pending` → `running` → `completed` / `failed` → `dead` |
| `TargetState` | The `WorkflowState` this job should move the Jira issue to |
| `Attempts` / `MaxAttempts` | Retry counter; `dead` when `attempts >= maxAttempts` |
| `NextRunAt` | Backoff timestamp; dequeue query filters on this |

### WorkflowState (`issue.go`)

String enum: `in_progress`, `code_review`, `rfqa`, `done`.

### AuditEntry (`audit.go`)

Immutable log record written after every job outcome and reconciler action.

---

## Key Files

| File | Content |
|---|---|
| `event.go` | `NormalizedEvent`, `EventType` constants |
| `job.go` | `Job`, `JobStatus`, `JobType` constants |
| `issue.go` | `WorkflowState` constants |
| `audit.go` | `AuditEntry` |

---

## Tests & Quality

No tests in this package — it is pure data types with no logic.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
