package jira

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
