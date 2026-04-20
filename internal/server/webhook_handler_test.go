package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mavolty/glsync/internal/config"
	"github.com/mavolty/glsync/internal/domain"
	"github.com/mavolty/glsync/internal/server"
	"github.com/mavolty/glsync/internal/store"
	"github.com/mavolty/glsync/internal/workflow"
)

// --- mock repositories ---

type mockEventRepo struct {
	insertErr     error
	existsResult  bool
	existsErr     error
	insertedEvent *domain.NormalizedEvent
}

func (m *mockEventRepo) Insert(_ context.Context, e domain.NormalizedEvent) error {
	m.insertedEvent = &e
	return m.insertErr
}
func (m *mockEventRepo) ExistsByIdempotencyKey(_ context.Context, _ string) (bool, error) {
	return m.existsResult, m.existsErr
}
func (m *mockEventRepo) GetByID(_ context.Context, _ string) (*domain.NormalizedEvent, error) {
	return nil, nil
}
func (m *mockEventRepo) ListRecent(_ context.Context, _ int) ([]domain.NormalizedEvent, error) {
	return nil, nil
}

type mockJobRepo struct {
	enqueueErr    error
	enqueueCount  int
}

func (m *mockJobRepo) Enqueue(_ context.Context, _ domain.Job) error {
	m.enqueueCount++
	return m.enqueueErr
}
func (m *mockJobRepo) Dequeue(_ context.Context, _ int) ([]domain.Job, error) { return nil, nil }
func (m *mockJobRepo) MarkCompleted(_ context.Context, _ string) error         { return nil }
func (m *mockJobRepo) MarkFailed(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}
func (m *mockJobRepo) MarkDead(_ context.Context, _ string, _ string) error       { return nil }
func (m *mockJobRepo) ListFailed(_ context.Context, _ int) ([]domain.Job, error)   { return nil, nil }
func (m *mockJobRepo) ListStale(_ context.Context, _ time.Duration) ([]domain.Job, error) {
	return nil, nil
}
func (m *mockJobRepo) ResetStuck(_ context.Context, _ time.Duration) (int64, error) { return 0, nil }

type mockAuditRepo struct{}

func (m *mockAuditRepo) Insert(_ context.Context, _ domain.AuditEntry) error { return nil }

// --- test helpers ---

// buildServer creates a real server.Server using mock repos.
func buildServer(events store.EventRepository, jobs store.JobRepository, secret string) http.Handler {
	cfg := config.Config{
		GitLab: config.GitLabConfig{WebhookSecret: secret},
		Workflow: config.WorkflowConfig{
			ProjectKey:         "RIS",
			DevelopBranch:      "develop",
			MasterBranch:       "master",
			Transitions:        map[string]string{"code_review": "14", "rfqa": "15", "done": "31"},
			DoneEmoji:          "thumbsup",
					},
		Worker: config.WorkerConfig{MaxAttempts: 5},
		Server: config.ServerConfig{
			Port:            8090,
			ReadTimeout:     config.Duration(5 * time.Second),
			WriteTimeout:    config.Duration(5 * time.Second),
			ShutdownTimeout: config.Duration(5 * time.Second),
		},
	}
	resolver := workflow.NewResolver(cfg.Workflow.Transitions)
	return server.NewHandler(cfg, nil, events, jobs, &mockAuditRepo{}, resolver, nil)
}

// newMRPayload builds a minimal merge_request webhook payload.
func newMRPayload(action, sourceBranch, targetBranch, title string, draft bool) []byte {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"iid":              1,
			"action":           action,
			"source_branch":    sourceBranch,
			"target_branch":    targetBranch,
			"title":            title,
			"draft":            draft,
			"work_in_progress": draft,
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

// newPushPayload builds a push webhook payload.
// commits is optional — each entry becomes a commit with that message.
func newPushPayload(branch, before string, commits ...string) []byte {
	var commitList []map[string]any
	for i, msg := range commits {
		commitList = append(commitList, map[string]any{
			"id":      fmt.Sprintf("sha%d", i),
			"message": msg,
			"author":  map[string]any{"email": "dev@example.com"},
		})
	}
	payload := map[string]any{
		"object_kind": "push",
		"user_email":  "dev@example.com",
		"project_id":  42,
		"ref":         "refs/heads/" + branch,
		"before":      before,
		"commits":     commitList,
	}
	b, _ := json.Marshal(payload)
	return b
}

func postWebhook(t *testing.T, handler http.Handler, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/gitlab", bytes.NewReader(body))
	req.Header.Set("X-Gitlab-Token", token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// --- tests ---

func TestWebhook_InvalidToken_Returns401(t *testing.T) {
	handler := buildServer(&mockEventRepo{}, &mockJobRepo{}, "correct-secret")
	body := newMRPayload("open", "feat/RIS-1", "develop", "fix RIS-1 RIS-1", false)

	rr := postWebhook(t, handler, body, "wrong-secret")

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestWebhook_DuplicateEvent_Returns200AlreadyProcessed(t *testing.T) {
	events := &mockEventRepo{existsResult: true}
	handler := buildServer(events, &mockJobRepo{}, "secret")
	body := newMRPayload("open", "feat/RIS-1", "develop", "fix RIS-1", false)

	rr := postWebhook(t, handler, body, "secret")

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "already_processed", resp["status"])
}

func TestWebhook_PushNonNewBranch_Returns200Ignored(t *testing.T) {
	handler := buildServer(&mockEventRepo{}, &mockJobRepo{}, "secret")
	// before is non-zero SHA and branch is not develop/master
	body := newPushPayload("feature/RIS-1", "abc1234abc1234abc1234abc1234abc1234abc123")

	rr := postWebhook(t, handler, body, "secret")

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "ignored", resp["status"])
}

func TestWebhook_DraftMR_Returns200Ignored(t *testing.T) {
	handler := buildServer(&mockEventRepo{}, &mockJobRepo{}, "secret")
	body := newMRPayload("open", "feat/RIS-2", "develop", "Draft: fix RIS-2 RIS-2", true)

	rr := postWebhook(t, handler, body, "secret")

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "ignored", resp["status"])
}

func TestWebhook_MROpened_HappyPath_Returns202(t *testing.T) {
	events := &mockEventRepo{}
	jobs := &mockJobRepo{}
	handler := buildServer(events, jobs, "secret")
	body := newMRPayload("open", "feature/RIS-999", "develop", "fix RIS-999 RIS-999", false)

	rr := postWebhook(t, handler, body, "secret")

	require.Equal(t, http.StatusAccepted, rr.Code)
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "accepted", resp["status"])
	assert.Equal(t, "code_review", resp["target_state"])

	// One job should be enqueued for RIS-999
	assert.Equal(t, 1, jobs.enqueueCount)
	require.NotNil(t, events.insertedEvent)
	assert.Contains(t, events.insertedEvent.IssueKeys, "RIS-999")
}

func TestWebhook_MRMergedToDevelop_Returns202WithRFQA(t *testing.T) {
	events := &mockEventRepo{}
	jobs := &mockJobRepo{}
	handler := buildServer(events, jobs, "secret")
	body := newMRPayload("merge", "feature/RIS-999", "develop", "fix RIS-999 RIS-999", false)

	rr := postWebhook(t, handler, body, "secret")

	require.Equal(t, http.StatusAccepted, rr.Code)
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "accepted", resp["status"])
	assert.Equal(t, 1, jobs.enqueueCount)
}

func TestWebhook_NoIssueKeys_Returns200Ignored(t *testing.T) {
	handler := buildServer(&mockEventRepo{}, &mockJobRepo{}, "secret")
	// Title and branch contain no RIS-xxx key
	body := newMRPayload("open", "feature/no-ticket", "develop", "general cleanup", false)

	rr := postWebhook(t, handler, body, "secret")

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "ignored", resp["status"])
}

func TestWebhook_UnrecognizedEvent_Returns200Ignored(t *testing.T) {
	handler := buildServer(&mockEventRepo{}, &mockJobRepo{}, "secret")
	body := []byte(`{"object_kind":"pipeline","id":1}`)

	rr := postWebhook(t, handler, body, "secret")

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "ignored", resp["status"])
}

func TestWebhook_PushToDevelop_WithCommitKeys_Returns202RFQA(t *testing.T) {
	events := &mockEventRepo{}
	jobs := &mockJobRepo{}
	handler := buildServer(events, jobs, "secret")

	// Simulate: local merge of RIS-500 branch into develop, then push
	body := newPushPayload("develop", "abc123def456abc123def456abc123def456abc1",
		"Merge branch 'RIS-500-fix-bug' into develop",
		"RIS-500: fix null pointer in executor",
	)

	rr := postWebhook(t, handler, body, "secret")

	require.Equal(t, http.StatusAccepted, rr.Code)
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	assert.Equal(t, "accepted", resp["status"])
	assert.Equal(t, "rfqa", resp["target_state"])
	assert.Equal(t, 1, jobs.enqueueCount)
	require.NotNil(t, events.insertedEvent)
	assert.Contains(t, events.insertedEvent.IssueKeys, "RIS-500")
}
