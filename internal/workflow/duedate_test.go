package workflow_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mavolty/glsync/internal/workflow"
)

func TestCalculateDueDate(t *testing.T) {
	// Fix a reference Monday for deterministic tests
	monday := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday

	tests := []struct {
		name             string
		strategy         string
		sprintEndWeekday string
		storyPoints      float64
		now              time.Time
		wantDate         time.Time
		wantErr          bool
	}{
		{
			name:             "sprint_end: monday -> next tuesday",
			strategy:         workflow.StrategySprintEnd,
			sprintEndWeekday: "tuesday",
			now:              monday,
			wantDate:         time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
		},
		{
			name:             "sprint_end: tuesday -> same tuesday",
			strategy:         workflow.StrategySprintEnd,
			sprintEndWeekday: "tuesday",
			now:              monday.AddDate(0, 0, 1), // Tuesday
			wantDate:         monday.AddDate(0, 0, 1),
		},
		{
			name:             "sprint_end: wednesday -> next tuesday",
			strategy:         workflow.StrategySprintEnd,
			sprintEndWeekday: "tuesday",
			now:              monday.AddDate(0, 0, 2), // Wednesday Apr 8
			wantDate:         monday.AddDate(0, 0, 8), // Tuesday Apr 14 (+6 days from Wed)
		},
		{
			name:        "story_points: SP=1 -> today+1",
			strategy:    workflow.StrategyStoryPoints,
			storyPoints: 1,
			now:         monday,
			wantDate:    monday.AddDate(0, 0, 1),
		},
		{
			name:        "story_points: SP=3 -> today+3",
			strategy:    workflow.StrategyStoryPoints,
			storyPoints: 3,
			now:         monday,
			wantDate:    monday.AddDate(0, 0, 3),
		},
		{
			name:        "story_points: SP=0 defaults to 1 day",
			strategy:    workflow.StrategyStoryPoints,
			storyPoints: 0,
			now:         monday,
			wantDate:    monday.AddDate(0, 0, 1),
		},
		{
			name:     "unknown strategy returns error",
			strategy: "unknown",
			now:      monday,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workflow.CalculateDueDate(tt.strategy, tt.sprintEndWeekday, tt.storyPoints, tt.now)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDate, got)
		})
	}
}
