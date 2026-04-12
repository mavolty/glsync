package domain

import (
	"encoding/json"
	"time"
)

type AuditEntry struct {
	ID        string
	EventID   string
	JobID     string
	Action    string
	IssueKey  string
	Detail    json.RawMessage
	CreatedAt time.Time
}
