[Root](../../../CLAUDE.md) > [internal](../../) > **gitlab**

# internal/gitlab

Handles everything specific to the GitLab webhook protocol: token validation, payload parsing, and idempotency key construction.

---

## Module Responsibility

- Validate the `X-Gitlab-Token` header using constant-time comparison.
- Parse raw JSON payloads into `domain.NormalizedEvent` for the two supported event kinds: `push` and `merge_request`.
- Classify MR events (open/reopen/merge/update/draft).
- Detect whether a push event represents a new branch (`before == 000…0`).

This package knows about GitLab shapes; no other package should import the raw GitLab types.

---

## Entry Points

| Symbol | File | Purpose |
|---|---|---|
| `ValidateToken(r, secret)` | `signature.go` | Returns `ErrInvalidSignature` if header does not match |
| `Parse(raw)` | `parser.go` | Dispatches to `parsePush` or `parseMR`; unknown kinds return `EventUnrecognized` |
| `IsNewBranch(before)` | `parser.go` | `true` when `before` is zero SHA or empty |

---

## Idempotency Key Conventions

| Event kind | Key format |
|---|---|
| Push | `push:{projectID}:{branch}:{before}` |
| MR | `mr:{projectID}:{mrIID}:{action}` |

---

## Key Files

| File | Content |
|---|---|
| `signature.go` | `ValidateToken`, `ErrInvalidSignature` |
| `parser.go` | `Parse`, `parsePush`, `parseMR`, `IsNewBranch` |
| `types.go` | `PushEvent`, `MergeRequestEvent`, `MRObjectAttributes`, `User`, `Project`, `Commit` |

---

## Tests & Quality

`signature_test.go` — 3 cases covering valid token, missing token, and wrong token. Uses `httptest.NewRequest`.

Gaps: no tests for `Parse` (push/MR payloads), `IsNewBranch`, or draft MR classification.

---

## Changelog

| Date | Change |
|---|---|
| 2026-04-08 | Initial CLAUDE.md generated |
