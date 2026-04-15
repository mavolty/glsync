package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gitlab.surya-am.com/sam/risk/glsync/internal/config"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/integration/jira"
	"gitlab.surya-am.com/sam/risk/glsync/internal/workflow"
)

// Executor dispatches a job to the appropriate integration client.
type Executor struct {
	jira       jira.Transitioner
	resolver   *workflow.Resolver
	inProgress config.InProgressConfig
	logger     *slog.Logger
}

func NewExecutor(jira jira.Transitioner, resolver *workflow.Resolver, inProgress config.InProgressConfig, logger *slog.Logger) *Executor {
	return &Executor{jira: jira, resolver: resolver, inProgress: inProgress, logger: logger}
}

func (e *Executor) Execute(ctx context.Context, job domain.Job) error {
	switch job.Type {
	case domain.JobJiraTransition:
		return e.executeJiraTransition(ctx, job)
	default:
		return fmt.Errorf("unknown job type: %q", job.Type)
	}
}

func (e *Executor) executeJiraTransition(ctx context.Context, job domain.Job) error {
	// In Progress transitions require date fields — handled separately
	if job.TargetState == domain.StateInProgress {
		return e.executeInProgressTransition(ctx, job)
	}

	transitionID, err := e.resolver.ResolveTransitionID(job.TargetState)
	if err != nil {
		// Missing transition ID means the operator hasn't configured it yet.
		// Log a warning and skip rather than burning retries on a config gap.
		e.logger.Warn("transition not configured — skipping job",
			"issue_key", job.IssueKey,
			"target_state", job.TargetState,
		)
		return nil
	}
	if err := e.jira.TransitionIssue(ctx, job.IssueKey, transitionID); err != nil {
		return fmt.Errorf("transition issue %s to %s (id %s): %w",
			job.IssueKey, job.TargetState, transitionID, err)
	}
	return nil
}

func (e *Executor) executeInProgressTransition(ctx context.Context, job domain.Job) error {
	cfg := e.inProgress
	if cfg.TransitionID == "" {
		return nil
	}

	var storyPoints float64 = 1
	if cfg.DueDateStrategy == workflow.StrategyStoryPoints {
		if sp, err := e.jira.GetStoryPoints(ctx, job.IssueKey, cfg.StoryPointsField); err == nil {
			storyPoints = sp
		}
	}

	today := time.Now()
	dueDate, err := workflow.CalculateDueDate(cfg.DueDateStrategy, cfg.SprintEndWeekday, storyPoints, today)
	if err != nil {
		return fmt.Errorf("calculate due date: %w", err)
	}

	const dateFormat = "2006-01-02"
	fields := map[string]any{
		cfg.StoryPointsField: storyPoints,
		cfg.StartDateField:   today.Format(dateFormat),
		cfg.DueDateField:     dueDate.Format(dateFormat),
	}

	if err := e.jira.TransitionIssueWithFields(ctx, job.IssueKey, cfg.TransitionID, fields); err != nil {
		return fmt.Errorf("transition issue %s to in_progress: %w", job.IssueKey, err)
	}
	return nil
}
