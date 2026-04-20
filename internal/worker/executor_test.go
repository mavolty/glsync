package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mavolty/glsync/internal/config"
	"github.com/mavolty/glsync/internal/domain"
	"github.com/mavolty/glsync/internal/integration/jira"
	"github.com/mavolty/glsync/internal/worker"
	"github.com/mavolty/glsync/internal/workflow"
)

// mockTransitioner implements jira.Transitioner for testing.
type mockTransitioner struct {
	transitionCalled          bool
	transitionIssueKey        string
	transitionID              string
	transitionErr             error

	transitionWithFieldsCalled bool
	transitionWithFieldsKey    string
	transitionWithFieldsID     string
	transitionWithFieldsFields map[string]interface{}
	transitionWithFieldsErr    error

	storyPoints    float64
	storyPointsErr error
}

func (m *mockTransitioner) TransitionIssue(_ context.Context, issueKey, transitionID string) error {
	m.transitionCalled = true
	m.transitionIssueKey = issueKey
	m.transitionID = transitionID
	return m.transitionErr
}

func (m *mockTransitioner) TransitionIssueWithFields(_ context.Context, issueKey, transitionID string, fields map[string]interface{}) error {
	m.transitionWithFieldsCalled = true
	m.transitionWithFieldsKey = issueKey
	m.transitionWithFieldsID = transitionID
	m.transitionWithFieldsFields = fields
	return m.transitionWithFieldsErr
}

func (m *mockTransitioner) UpdateIssueFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *mockTransitioner) GetIssueStatus(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockTransitioner) GetStoryPoints(_ context.Context, _ string, _ string) (float64, error) {
	return m.storyPoints, m.storyPointsErr
}

func (m *mockTransitioner) GetTransitions(_ context.Context, _ string) ([]jira.Transition, error) {
	return nil, nil
}

func newTestExecutor(mock *mockTransitioner, transitions map[string]string, inProgress config.InProgressConfig) *worker.Executor {
	resolver := workflow.NewResolver(transitions)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return worker.NewExecutor(mock, resolver, inProgress, logger)
}

func baseJob(jobType domain.JobType, state domain.WorkflowState) domain.Job {
	return domain.Job{
		ID:          "job-1",
		EventID:     "evt-1",
		Type:        jobType,
		IssueKey:    "RIS-123",
		TargetState: state,
		MaxAttempts: 5,
	}
}

func TestExecutor_JiraTransition_HappyPath(t *testing.T) {
	mock := &mockTransitioner{}
	exec := newTestExecutor(mock, map[string]string{
		"code_review": "14",
	}, config.InProgressConfig{})

	job := baseJob(domain.JobJiraTransition, domain.StateCodeReview)
	err := exec.Execute(context.Background(), job)

	require.NoError(t, err)
	assert.True(t, mock.transitionCalled)
	assert.Equal(t, "RIS-123", mock.transitionIssueKey)
	assert.Equal(t, "14", mock.transitionID)
}

func TestExecutor_JiraTransition_UnconfiguredStateSkips(t *testing.T) {
	mock := &mockTransitioner{}
	// "done" is not in the transitions map
	exec := newTestExecutor(mock, map[string]string{
		"code_review": "14",
	}, config.InProgressConfig{})

	job := baseJob(domain.JobJiraTransition, domain.StateDone)
	err := exec.Execute(context.Background(), job)

	require.NoError(t, err, "unconfigured transition should not return an error")
	assert.False(t, mock.transitionCalled, "no Jira call should be made for unconfigured state")
}

func TestExecutor_JiraTransition_EmptyTransitionIDSkips(t *testing.T) {
	mock := &mockTransitioner{}
	// "done" is explicitly empty string
	exec := newTestExecutor(mock, map[string]string{
		"done": "",
	}, config.InProgressConfig{})

	job := baseJob(domain.JobJiraTransition, domain.StateDone)
	err := exec.Execute(context.Background(), job)

	require.NoError(t, err, "empty transition ID should not return an error")
	assert.False(t, mock.transitionCalled)
}

func TestExecutor_JiraTransition_JiraErrorPropagates(t *testing.T) {
	jiraErr := errors.New("jira 500")
	mock := &mockTransitioner{transitionErr: jiraErr}
	exec := newTestExecutor(mock, map[string]string{
		"code_review": "14",
	}, config.InProgressConfig{})

	job := baseJob(domain.JobJiraTransition, domain.StateCodeReview)
	err := exec.Execute(context.Background(), job)

	require.Error(t, err)
	assert.ErrorIs(t, err, jiraErr)
}

func TestExecutor_InProgressTransition_WithDateFields(t *testing.T) {
	mock := &mockTransitioner{storyPoints: 3}
	inProgress := config.InProgressConfig{
		TransitionID:     "2",
		DueDateStrategy:  workflow.StrategySprintEnd,
		SprintEndWeekday: "tuesday",
		StartDateField:   "customfield_10040",
		DueDateField:     "duedate",
		StoryPointsField: "customfield_10027",
	}
	exec := newTestExecutor(mock, nil, inProgress)

	job := baseJob(domain.JobJiraTransition, domain.StateInProgress)
	err := exec.Execute(context.Background(), job)

	require.NoError(t, err)
	assert.True(t, mock.transitionWithFieldsCalled)
	assert.Equal(t, "2", mock.transitionWithFieldsID)
	assert.Equal(t, "RIS-123", mock.transitionWithFieldsKey)
	fields := mock.transitionWithFieldsFields
	assert.NotEmpty(t, fields["customfield_10040"], "start date should be set")
	assert.NotEmpty(t, fields["duedate"], "due date should be set")
}

func TestExecutor_InProgressTransition_EmptyTransitionIDSkips(t *testing.T) {
	mock := &mockTransitioner{}
	inProgress := config.InProgressConfig{
		TransitionID: "", // not configured
	}
	exec := newTestExecutor(mock, nil, inProgress)

	job := baseJob(domain.JobJiraTransition, domain.StateInProgress)
	err := exec.Execute(context.Background(), job)

	require.NoError(t, err)
	assert.False(t, mock.transitionWithFieldsCalled)
}


func TestExecutor_UnknownJobTypeErrors(t *testing.T) {
	mock := &mockTransitioner{}
	exec := newTestExecutor(mock, nil, config.InProgressConfig{})

	job := baseJob("unknown_type", "")
	err := exec.Execute(context.Background(), job)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown job type")
}
