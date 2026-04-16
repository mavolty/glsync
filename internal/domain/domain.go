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
	EventEmojiAward   EventType = "emoji_award"
	EventUnrecognized EventType = "unrecognized"
)

// NormalizedEvent is the canonical internal representation of a GitLab webhook event.
// It is transport-agnostic and is what all downstream processing operates on.
type NormalizedEvent struct {
	ID             string
	IdempotencyKey string
	EventType      EventType
	IssueKeys      []string
	SourceBranch   string
	TargetBranch   string
	MRTitle        string
	MRIID          int
	ProjectID      int
	AuthorEmail    string
	RawPayload     json.RawMessage
	ReceivedAt     time.Time
	CommitMessages []string // commit messages from push events — used for issue key extraction
	EmojiName      string   // e.g. "thumbsup" — set for emoji events
	MRState        string   // e.g. "merged" — set for emoji events
}

type WorkflowState string

const (
	StateInProgress WorkflowState = "in_progress"
	StateCodeReview WorkflowState = "code_review"
	StateRFQA       WorkflowState = "rfqa"
	StateDone       WorkflowState = "done"
)

type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobDead      JobStatus = "dead"
)

type JobType string

const (
	JobJiraTransition JobType = "jira_transition"
)

type Job struct {
	ID          string
	EventID     string
	Type        JobType
	Status      JobStatus
	IssueKey    string
	TargetState WorkflowState
	Payload     json.RawMessage
	Attempts    int
	MaxAttempts int
	LastError   string
	NextRunAt   time.Time
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type AuditEntry struct {
	ID        string
	EventID   string
	JobID     string
	Action    string
	IssueKey  string
	Detail    json.RawMessage
	CreatedAt time.Time
}
