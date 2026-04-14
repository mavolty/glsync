package jira

import "fmt"

// maxErrorBodyBytes limits how much of a Jira error response we read into memory.
const maxErrorBodyBytes = 4096

// JiraHTTPError represents a non-success HTTP response from Jira.
// The StatusCode field lets callers distinguish retryable (5xx) from
// non-retryable (4xx) failures without string-parsing.
type JiraHTTPError struct {
	StatusCode int
	Body       string
}

func (e *JiraHTTPError) Error() string {
	return fmt.Sprintf("jira returned %d: %s", e.StatusCode, e.Body)
}

// IsNonRetryable returns true for HTTP status codes that indicate a permanent
// failure — retrying the same request will always produce the same result.
func (e *JiraHTTPError) IsNonRetryable() bool {
	switch e.StatusCode {
	case 400, 404, 405, 409, 422:
		return true
	default:
		return false
	}
}

// TransitionRequest is the payload for POST /rest/api/2/issue/{key}/transitions
type TransitionRequest struct {
	Transition TransitionID           `json:"transition"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
}

type TransitionID struct {
	ID string `json:"id"`
}

// TransitionsResponse is the response from GET /rest/api/2/issue/{key}/transitions
type TransitionsResponse struct {
	Transitions []Transition `json:"transitions"`
}

type Transition struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	To   TransitionTo   `json:"to"`
}

type TransitionTo struct {
	Name string `json:"name"`
}

// IssueResponse is the partial response from GET /rest/api/2/issue/{key}
type IssueResponse struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

type IssueFields struct {
	Status      IssueStatus `json:"status"`
	StoryPoints *float64    `json:"customfield_10027"` // populated when queried with the right field
}

type IssueStatus struct {
	Name string `json:"name"`
}
