package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mavolty/glsync/internal/domain"
	"github.com/mavolty/glsync/internal/workflow"
)

var testCfg = workflow.RuleConfig{
	DevelopBranch: "develop",
	MasterBranch:  "master",
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name          string
		event         domain.NormalizedEvent
		wantState     domain.WorkflowState
		wantProcess   bool
	}{
		{
			name:        "push event -> in progress",
			event:       domain.NormalizedEvent{EventType: domain.EventPush, SourceBranch: "feature/RIS-123"},
			wantState:   domain.StateInProgress,
			wantProcess: true,
		},
		{
			name:        "push to develop -> rfqa",
			event:       domain.NormalizedEvent{EventType: domain.EventPush, SourceBranch: "develop"},
			wantState:   domain.StateRFQA,
			wantProcess: true,
		},
		{
			name:        "push to master -> done",
			event:       domain.NormalizedEvent{EventType: domain.EventPush, SourceBranch: "master"},
			wantState:   domain.StateDone,
			wantProcess: true,
		},
		{
			name:        "MR opened -> code review",
			event:       domain.NormalizedEvent{EventType: domain.EventMROpened},
			wantState:   domain.StateCodeReview,
			wantProcess: true,
		},
		{
			name:        "MR merged into develop -> rfqa",
			event:       domain.NormalizedEvent{EventType: domain.EventMRMerged, TargetBranch: "develop"},
			wantState:   domain.StateRFQA,
			wantProcess: true,
		},
		{
			name:        "MR merged into master -> no auto-done (emoji trigger)",
			event:       domain.NormalizedEvent{EventType: domain.EventMRMerged, TargetBranch: "master"},
			wantState:   "",
			wantProcess: false,
		},
		{
			name:        "MR merged into other branch -> ignored",
			event:       domain.NormalizedEvent{EventType: domain.EventMRMerged, TargetBranch: "feature/other"},
			wantState:   "",
			wantProcess: false,
		},
		{
			name:        "draft MR -> ignored",
			event:       domain.NormalizedEvent{EventType: domain.EventMRDraft},
			wantState:   "",
			wantProcess: false,
		},
		{
			name:        "unrecognized event -> ignored",
			event:       domain.NormalizedEvent{EventType: domain.EventUnrecognized},
			wantState:   "",
			wantProcess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, process := workflow.Classify(tt.event, testCfg)
			assert.Equal(t, tt.wantState, state)
			assert.Equal(t, tt.wantProcess, process)
		})
	}
}
