package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client calls the GitLab REST API v4.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a GitLab API client.
// baseURL should be the GitLab instance root (e.g. "https://gitlab.example.com").
func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: baseURL, token: token, httpClient: httpClient}
}

// CountAwardEmoji returns the number of users who awarded the given emoji on an MR.
// Calls GET /api/v4/projects/:id/merge_requests/:iid/award_emoji
func (c *Client) CountAwardEmoji(ctx context.Context, projectID, mrIID int, emojiName string) (int, error) {
	path := fmt.Sprintf("/api/v4/projects/%d/merge_requests/%d/award_emoji", projectID, mrIID)
	params := url.Values{}
	if emojiName != "" {
		params.Set("name", emojiName)
	}
	fullURL := c.baseURL + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gitlab api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("gitlab api returned %d: %s", resp.StatusCode, string(body))
	}

	var awards []awardEmoji
	if err := json.NewDecoder(resp.Body).Decode(&awards); err != nil {
		return 0, fmt.Errorf("decode award emojis: %w", err)
	}

	if emojiName == "" {
		return len(awards), nil
	}

	// Filter by name and count unique users
	seen := make(map[int]bool)
	for _, a := range awards {
		if a.Name == emojiName && !seen[a.User.ID] {
			seen[a.User.ID] = true
		}
	}
	return len(seen), nil
}

type awardEmoji struct {
	Name string `json:"name"`
	User struct {
		ID int `json:"id"`
	} `json:"user"`
}
