[Root](../../../CLAUDE.md) > [internal](../../) > **workflow**

# internal/workflow

Pure business logic: classify incoming events into target workflow states, resolve those states to Jira transition IDs, and calculate due dates for "In Progress" transitions.

---

## Module Responsibility

Three orthogonal concerns kept in separate files:

1. **Classification** — which `WorkflowState` does an event map to?
2. **Transition resolution** — what Jira transition ID corresponds to a state?
3. **Due-date calculation** — how is the due date computed for `in_progress`?

No I/O in this package.

---

## API

### rules.go

```go
type RuleConfig struct {
    DevelopBranch string
    MasterBranch  string
}

// Returns (state, shouldProcess).
// shouldProcess == false means the event should be silently dropped.
func Classify(event domain.NormalizedEvent, cfg RuleConfig) (domain.WorkflowState, bool)
```

Classification table:

| EventType | Condition | State |
|---|---|---|
| `push` | (caller filters new-branch) | `in_progress` |
| `mr_opened` | — | `code_review` |
| `mr_merged` | target == `DevelopBranch` | `rfqa` |
| `mr_merged` | target == `MasterBranch` | `done` |
| `mr_draft`, `unrecognized` | — | `""`, false |

### transition.go

```go
type Resolver struct { /* transitions map[string]string */ }

func NewResolver(transitions map[string]string) *Resolver
func (r *Resolver) ResolveTransitionID(state domain.WorkflowState) (string, error)
```

Transition IDs come from `config.WorkflowConfig.Transitions`. Returns an error for any unconfigured or empty state — fail loudly rather than silently.

### duedate.go

```go
const StrategySprintEnd   = "sprint_end"   // next occurrence of configured weekday
const StrategyStoryPoints = "story_points" // today + SP days (min 1)

func CalculateDueDate(strategy, sprintEndWeekday string, storyPoints float64, now time.Time) (time.Time, error)
func NextWeekday(from time.Time, weekday time.Weekday) time.Time
func ParseWeekday(s string) (time.Weekday, error)
```

---

## Key Files

| File | Content |
|---|---|
| `rules.go` | `Classify`, `RuleConfig` |
| `transition.go` | `Resolver`, `NewResolver`, `ResolveTransitionID` |
| `duedate.go` | `CalculateDueDate`, `NextWeekday`, `ParseWeekday`, strategy constants |
| `rules_test.go` | 7 table-driven tests for `Classify` |
| `transition_test.go` | 2 tests for `Resolver` |
| `duedate_test.go` | 7 table-driven tests for `CalculateDueDate` |

---

## Tests & Quality

Good coverage of all three sub-concerns. Deterministic dates (fixed Monday 2026-04-06) are used in due-date tests. No gaps identified.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
