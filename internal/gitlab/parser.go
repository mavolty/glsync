package gitlab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
)

const zeroSHA = "0000000000000000000000000000000000000000"

// Parse converts a raw GitLab webhook payload into a NormalizedEvent.
// The object_kind field determines which event type is parsed.
// Unrecognized or unsupported event kinds return EventUnrecognized (not an error).
func Parse(raw json.RawMessage) (domain.NormalizedEvent, error) {
	var probe struct {
		ObjectKind string `json:"object_kind"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return domain.NormalizedEvent{}, fmt.Errorf("read object_kind: %w", err)
	}

	switch probe.ObjectKind {
	case "push":
		return parsePush(raw)
	case "merge_request":
		return parseMR(raw)
	default:
		return domain.NormalizedEvent{
			EventType:   domain.EventUnrecognized,
			RawPayload:  raw,
			ReceivedAt:  time.Now(),
		}, nil
	}
}

func parsePush(raw json.RawMessage) (domain.NormalizedEvent, error) {
	var ev PushEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return domain.NormalizedEvent{}, fmt.Errorf("parse push event: %w", err)
	}

	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	idempKey := fmt.Sprintf("push:%d:%s:%s", ev.ProjectID, branch, ev.Before)

	return domain.NormalizedEvent{
		IdempotencyKey: idempKey,
		EventType:      domain.EventPush,
		SourceBranch:   branch,
		ProjectID:      ev.ProjectID,
		AuthorEmail:    ev.UserEmail,
		RawPayload:     raw,
		ReceivedAt:     time.Now(),
	}, nil
}

// IsNewBranch returns true when the push event represents a newly created branch.
func IsNewBranch(before string) bool {
	return before == zeroSHA || before == ""
}

func parseMR(raw json.RawMessage) (domain.NormalizedEvent, error) {
	var ev MergeRequestEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return domain.NormalizedEvent{}, fmt.Errorf("parse merge_request event: %w", err)
	}

	oa := ev.ObjectAttributes
	isDraft := oa.Draft || oa.WorkInProgress

	eventType := classifyMR(oa.Action, isDraft)
	idempKey := fmt.Sprintf("mr:%d:%d:%s", ev.Project.ID, oa.IID, oa.Action)

	return domain.NormalizedEvent{
		IdempotencyKey: idempKey,
		EventType:      eventType,
		SourceBranch:   oa.SourceBranch,
		TargetBranch:   oa.TargetBranch,
		MRTitle:        oa.Title,
		MRIID:          oa.IID,
		ProjectID:      ev.Project.ID,
		AuthorEmail:    ev.User.Email,
		RawPayload:     raw,
		ReceivedAt:     time.Now(),
	}, nil
}

func classifyMR(action string, isDraft bool) domain.EventType {
	switch action {
	case "open", "reopen":
		if isDraft {
			return domain.EventMRDraft
		}
		return domain.EventMROpened
	case "merge":
		return domain.EventMRMerged
	case "update":
		// Could be a draft->ready transition; treat conservatively
		if !isDraft {
			return domain.EventMROpened
		}
		return domain.EventMRDraft
	default:
		return domain.EventUnrecognized
	}
}
