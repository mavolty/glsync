package worker_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/mavolty/glsync/internal/domain"
	"github.com/mavolty/glsync/internal/worker"
)

func TestNewJob(t *testing.T) {
	job := worker.NewJob("evt-1", "RIS-42", domain.StateInProgress, 5)

	assert.NotEmpty(t, job.ID)
	assert.Equal(t, "evt-1", job.EventID)
	assert.Equal(t, domain.JobJiraTransition, job.Type)
	assert.Equal(t, domain.JobPending, job.Status)
	assert.Equal(t, "RIS-42", job.IssueKey)
	assert.Equal(t, domain.StateInProgress, job.TargetState)
	assert.Equal(t, 0, job.Attempts)
	assert.Equal(t, 5, job.MaxAttempts)
	assert.NotNil(t, job.Payload)
}

func TestNewJob_UniqueIDs(t *testing.T) {
	j1 := worker.NewJob("e", "RIS-1", domain.StateDone, 3)
	j2 := worker.NewJob("e", "RIS-1", domain.StateDone, 3)
	assert.NotEqual(t, j1.ID, j2.ID)
}

func TestBackoffForAttempt(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		// formula: min(2^attempt * 30s, 30min)
		{0, 30 * time.Second},
		{1, 60 * time.Second},
		{2, 120 * time.Second},
		{3, 240 * time.Second},
	}

	for _, tt := range tests {
		d := worker.BackoffForAttempt(tt.attempt)
		assert.Equal(t, tt.want, d, "attempt %d", tt.attempt)
	}
}

func TestBackoffForAttempt_CapsAtMax(t *testing.T) {
	// Very high attempt count must not exceed 30 minutes.
	d := worker.BackoffForAttempt(100)
	assert.Equal(t, 30*time.Minute, d)
}
