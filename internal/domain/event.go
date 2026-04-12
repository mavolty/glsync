package domain

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	EventPush         EventType = "push"
	EventMROpened     EventType = "mr_opened"
	EventMRMerged     EventType = "mr_merged"
	EventMRDraft      EventType = "mr_draft"
	EventUnrecognized EventType = "unrecognized"
)

// NormalizedEvent is the canonical internal representation of a GitLab webhook event.
// It is transport-agnostic and is what all downstream processing operates on.
type NormalizedEvent struct {
	ID              string
	IdempotencyKey  string
	EventType       EventType
	IssueKeys       []string
	SourceBranch    string
	TargetBranch    string
	MRTitle         string
	MRIID           int
	ProjectID       int
	AuthorEmail    string
	RawPayload     json.RawMessage
	ReceivedAt      time.Time
}
