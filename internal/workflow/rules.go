package workflow

import (
	"github.com/mavolty/glsync/internal/domain"
)

// RuleConfig holds the branch names used to determine target states.
type RuleConfig struct {
	DevelopBranch string
	MasterBranch  string
}

// Classify determines the target workflow state for a normalized event.
// Returns (state, true) when the event should trigger a Jira transition.
// Returns ("", false) when the event should be silently skipped.
func Classify(event domain.NormalizedEvent, cfg RuleConfig) (domain.WorkflowState, bool) {
	switch event.EventType {
	case domain.EventPush:
		return classifyPush(event, cfg)

	case domain.EventMROpened:
		return domain.StateCodeReview, true

	case domain.EventMRMerged:
		return classifyMerge(event, cfg)

	case domain.EventMRDraft, domain.EventUnrecognized:
		return "", false

	default:
		return "", false
	}
}

// classifyPush returns the target state for a push event.
// Pushes to develop/master trigger RFQA/Done respectively.
// All other pushes are classified as InProgress (caller filters new branches).
func classifyPush(event domain.NormalizedEvent, cfg RuleConfig) (domain.WorkflowState, bool) {
	switch event.SourceBranch {
	case cfg.DevelopBranch:
		return domain.StateRFQA, true
	case cfg.MasterBranch:
		return domain.StateDone, true
	default:
		return domain.StateInProgress, true
	}
}

func classifyMerge(event domain.NormalizedEvent, cfg RuleConfig) (domain.WorkflowState, bool) {
	switch event.TargetBranch {
	case cfg.DevelopBranch:
		return domain.StateRFQA, true
	case cfg.MasterBranch:
		// No auto-transition — done is triggered by emoji reaction on the merged MR
		return "", false
	default:
		// Merges into feature branches or other branches are ignored
		return "", false
	}
}
