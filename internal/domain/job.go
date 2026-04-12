package domain

import (
	"encoding/json"
	"time"
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
	JobLarkBaseSync   JobType = "larkbase_sync"
	JobFeishuNotify   JobType = "feishu_notify"
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
