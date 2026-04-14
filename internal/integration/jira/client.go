package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Transitioner defines the operations glsync needs from Jira.
// This interface is the only boundary; the rest of the codebase does not import this package directly.
type Transitioner interface {
	TransitionIssue(ctx context.Context, issueKey string, transitionID string) error
	TransitionIssueWithFields(ctx context.Context, issueKey string, transitionID string, fields map[string]interface{}) error
	UpdateIssueFields(ctx context.Context, issueKey string, fields map[string]interface{}) error
	GetIssueStatus(ctx context.Context, issueKey string) (string, error)
	GetStoryPoints(ctx context.Context, issueKey string, storyPointsField string) (float64, error)
	GetTransitions(ctx context.Context, issueKey string) ([]Transition, error)
}

type Client struct {
	baseURL    string
	username   string
	apiToken   string
	httpClient *http.Client
}

func NewClient(baseURL, username, apiToken string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		username:   username,
		apiToken:   apiToken,
		httpClient: httpClient,
	}
}

func (c *Client) TransitionIssue(ctx context.Context, issueKey, transitionID string) error {
	return c.TransitionIssueWithFields(ctx, issueKey, transitionID, nil)
}

func (c *Client) TransitionIssueWithFields(ctx context.Context, issueKey, transitionID string, fields map[string]interface{}) error {
	payload := TransitionRequest{
		Transition: TransitionID{ID: transitionID},
		Fields:     fields,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal transition request: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create transition request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute transition request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return &JiraHTTPError{StatusCode: resp.StatusCode, Body: string(errBody)}
	}
	return nil
}

// UpdateIssueFields updates arbitrary fields on a Jira issue via PUT /rest/api/2/issue/{key}.
// Use this to set fields that are not on the transition screen.
func (c *Client) UpdateIssueFields(ctx context.Context, issueKey string, fields map[string]interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"fields": fields})
	if err != nil {
		return fmt.Errorf("marshal update request: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute update request: %w", err)
	}
	defer resp.Body.Close()

	// Jira returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return &JiraHTTPError{StatusCode: resp.StatusCode, Body: string(errBody)}
	}
	return nil
}

// GetStoryPoints fetches the story points value for an issue using the configured custom field.
// Returns 1.0 as a safe default if the field is null or missing.
func (c *Client) GetStoryPoints(ctx context.Context, issueKey, storyPointsField string) (float64, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=%s", c.baseURL, issueKey, storyPointsField)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("create get story points request: %w", err)
	}
	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("execute get story points request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return 0, &JiraHTTPError{StatusCode: resp.StatusCode, Body: string(errBody)}
	}

	// Use a dynamic map to handle any custom field name
	var result struct {
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode story points response: %w", err)
	}

	val, ok := result.Fields[storyPointsField]
	if !ok || val == nil {
		return 1, nil // safe default
	}
	sp, ok := val.(float64)
	if !ok {
		return 1, nil // safe default
	}
	return sp, nil
}

func (c *Client) GetIssueStatus(ctx context.Context, issueKey string) (string, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s?fields=status", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create get issue request: %w", err)
	}
	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute get issue request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return "", &JiraHTTPError{StatusCode: resp.StatusCode, Body: string(errBody)}
	}

	var issue IssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return "", fmt.Errorf("decode issue response: %w", err)
	}
	return issue.Fields.Status.Name, nil
}

func (c *Client) GetTransitions(ctx context.Context, issueKey string) ([]Transition, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", c.baseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create get transitions request: %w", err)
	}
	req.SetBasicAuth(c.username, c.apiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get transitions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, &JiraHTTPError{StatusCode: resp.StatusCode, Body: string(errBody)}
	}

	var result TransitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode transitions response: %w", err)
	}
	return result.Transitions, nil
}
