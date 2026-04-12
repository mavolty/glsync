package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/store"
)

const (
	baseBackoff = 30 * time.Second
	maxBackoff  = 30 * time.Minute
)

// Processor is the background worker that dequeues and executes jobs.
type Processor struct {
	jobs    store.JobRepository
	audit   store.AuditRepository
	exec    *Executor
	cfg     ProcessorConfig
	logger  *slog.Logger
}

type ProcessorConfig struct {
	Concurrency  int
	PollInterval time.Duration
	MaxAttempts  int
}

func NewProcessor(
	jobs store.JobRepository,
	audit store.AuditRepository,
	exec *Executor,
	cfg ProcessorConfig,
	logger *slog.Logger,
) *Processor {
	return &Processor{
		jobs:   jobs,
		audit:  audit,
		exec:   exec,
		cfg:    cfg,
		logger: logger,
	}
}

// Run starts the processor loop. It blocks until ctx is cancelled.
// All in-flight jobs are completed before Run returns.
func (p *Processor) Run(ctx context.Context) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.cfg.Concurrency)

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			jobs, err := p.jobs.Dequeue(ctx, p.cfg.Concurrency)
			if err != nil {
				p.logger.Error("dequeue jobs", "error", err)
				continue
			}

			for _, j := range jobs {
				sem <- struct{}{}
				wg.Add(1)
				go func(job domain.Job) {
					defer func() { <-sem; wg.Done() }()
					p.process(ctx, job)
				}(j)
			}
		}
	}
}

func (p *Processor) process(ctx context.Context, job domain.Job) {
	log := p.logger.With("job_id", job.ID, "issue_key", job.IssueKey, "type", job.Type)

	err := p.exec.Execute(ctx, job)
	if err == nil {
		if markErr := p.jobs.MarkCompleted(ctx, job.ID); markErr != nil {
			log.Error("mark job completed", "error", markErr)
		}
		p.writeAudit(ctx, job, "transition_success", map[string]any{
			"target_state": job.TargetState,
		})
		log.Info("job completed", "target_state", job.TargetState)
		return
	}

	log.Warn("job failed", "error", err, "attempt", job.Attempts+1)
	p.writeAudit(ctx, job, "transition_failed", map[string]any{
		"error":   err.Error(),
		"attempt": job.Attempts + 1,
	})

	if job.Attempts+1 >= job.MaxAttempts {
		if markErr := p.jobs.MarkDead(ctx, job.ID, err.Error()); markErr != nil {
			log.Error("mark job dead", "error", markErr)
		}
		p.writeAudit(ctx, job, "job_dead", map[string]any{"reason": "exhausted retries"})
		log.Error("job dead — exhausted retries", "max_attempts", job.MaxAttempts)
		return
	}

	next := nextBackoff(job.Attempts)
	if markErr := p.jobs.MarkFailed(ctx, job.ID, err.Error(), time.Now().Add(next)); markErr != nil {
		log.Error("mark job failed", "error", markErr)
	}
	log.Info("job scheduled for retry", "next_run_in", next)
}

func (p *Processor) writeAudit(ctx context.Context, job domain.Job, action string, detail map[string]any) {
	raw, _ := json.Marshal(detail)
	entry := domain.AuditEntry{
		ID:        uuid.NewString(),
		EventID:   job.EventID,
		JobID:     job.ID,
		Action:    action,
		IssueKey:  job.IssueKey,
		Detail:    raw,
		CreatedAt: time.Now(),
	}
	if err := p.audit.Insert(ctx, entry); err != nil {
		p.logger.Error("write audit log", "action", action, "error", err)
	}
}

// nextBackoff calculates the delay before the next retry attempt.
// Formula: min(2^attempts * baseBackoff, maxBackoff)
func nextBackoff(attempts int) time.Duration {
	factor := math.Pow(2, float64(attempts))
	d := time.Duration(factor) * baseBackoff
	if d > maxBackoff {
		return maxBackoff
	}
	return d
}

// BackoffForAttempt is exported for testing and reconciler use.
func BackoffForAttempt(attempt int) time.Duration {
	return nextBackoff(attempt)
}

// NewJob constructs a Job ready for enqueuing.
func NewJob(eventID, issueKey string, targetState domain.WorkflowState, maxAttempts int) domain.Job {
	return domain.Job{
		ID:          uuid.NewString(),
		EventID:     eventID,
		Type:        domain.JobJiraTransition,
		Status:      domain.JobPending,
		IssueKey:    issueKey,
		TargetState: targetState,
		Payload:     json.RawMessage("{}"),
		Attempts:    0,
		MaxAttempts: maxAttempts,
		NextRunAt:   time.Now(),
		CreatedAt:   time.Now(),
	}
}
