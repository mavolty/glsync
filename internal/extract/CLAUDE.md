[Root](../../../CLAUDE.md) > [internal](../../) > **extract**

# internal/extract

Stateless regex helpers that pull Jira issue keys out of free-form text strings.

---

## Module Responsibility

Extract and deduplicate Jira issue keys (e.g. `RIS-123`) from branch names and MR titles. The project key is configurable at call time.

---

## API

```go
// Returns deduplicated, uppercased keys in order of first occurrence.
// Returns empty slice (never nil) when nothing matches.
extract.IssueKeys(projectKey, text string) []string

// Branch is preferred; falls back to title when branch yields nothing.
extract.IssueKeysFromBranchAndTitle(projectKey, branch, title string) []string
```

Matching is case-insensitive and word-boundary anchored, so `NORIS-123` is not matched when `projectKey = "RIS"`.

---

## Key Files

| File | Content |
|---|---|
| `issuekey.go` | `IssueKeys`, `IssueKeysFromBranchAndTitle` |
| `issuekey_test.go` | 10 table-driven unit tests |

---

## Tests & Quality

Full coverage of the happy path, deduplication, case normalisation, word-boundary enforcement, fallback logic, and the no-match case. No gaps identified.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
