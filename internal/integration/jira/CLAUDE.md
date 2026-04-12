[Root](../../../../CLAUDE.md) > [internal](../../../) > [integration](../) > **jira**

# internal/integration/jira

Jira REST API v2 client. Implements the `Transitioner` interface used throughout the worker layer.

---

## Module Responsibility

Encapsulate all HTTP communication with Jira Cloud. The rest of the codebase interacts only with the `Transitioner` interface — never with the concrete `Client` or this package's imports.

---

## Interface

```go
type Transitioner interface {
    TransitionIssue(ctx context.Context, issueKey, transitionID string) error
    TransitionIssueWithFields(ctx context.Context, issueKey, transitionID string, fields map[string]interface{}) error
    GetIssueStatus(ctx context.Context, issueKey string) (string, error)
    GetStoryPoints(ctx context.Context, issueKey, storyPointsField string) (float64, error)
    GetTransitions(ctx context.Context, issueKey string) ([]Transition, error)
}
```

`TransitionIssue` delegates to `TransitionIssueWithFields` with `nil` fields.

---

## HTTP Calls

| Method | Jira Endpoint | Used by |
|---|---|---|
| `POST` | `/rest/api/2/issue/{key}/transitions` | `TransitionIssue`, `TransitionIssueWithFields` |
| `GET` | `/rest/api/2/issue/{key}?fields=status` | `GetIssueStatus` |
| `GET` | `/rest/api/2/issue/{key}?fields={storyPointsField}` | `GetStoryPoints` |
| `GET` | `/rest/api/2/issue/{key}/transitions` | `GetTransitions` |

Authentication: HTTP Basic Auth (`username:api_token`).

---

## Story Points Handling

`GetStoryPoints` returns `1.0` (safe default) when:
- The field is absent or null in the Jira response.
- The field value is not a `float64`.
- The HTTP call fails (caller in executor treats this as non-fatal).

---

## Key Files

| File | Content |
|---|---|
| `client.go` | `Transitioner` interface, `Client` struct, all HTTP methods |
| `types.go` | `TransitionRequest`, `TransitionsResponse`, `IssueResponse`, `IssueFields`, `IssueStatus` |

---

## Tests & Quality

No tests in this package. Recommended: test `TransitionIssueWithFields` and `GetStoryPoints` with `httptest.NewServer` to simulate Jira responses (success, 4xx, malformed JSON).

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
