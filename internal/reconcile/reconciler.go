package reconcile

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/store"
)

// Reconciler periodically detects and repairs drift between expected and actual state.
type Reconciler struct {
	jobs   store.JobRepository
	audit  store.AuditRepository
	cfg    ReconcileConfig
	logger *slog.Logger
}

type ReconcileConfig struct {
	Interval        time.Duration
	StuckJobTimeout time.Duration
	DriftLookback   time.Duration
}

func New(
	jobs store.JobRepository,
	audit store.AuditRepository,
	cfg ReconcileConfig,
	logger *slog.Logger,
) *Reconciler {
	return &Reconciler{
		jobs:   jobs,
		audit:  audit,
		cfg:    cfg,
		logger: logger,
	}
}

// Run starts the reconciliation loop. It blocks until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Reconciler) runOnce(ctx context.Context) {
	r.resetStuckJobs(ctx)
}

func (r *Reconciler) resetStuckJobs(ctx context.Context) {
	count, err := r.jobs.ResetStuck(ctx, r.cfg.StuckJobTimeout)
	if err != nil {
		r.logger.Error("reconcile: reset stuck jobs", "error", err)
		return
	}
	if count > 0 {
		r.logger.Warn("reconcile: reset stuck jobs", "count", count)
		r.writeAudit(ctx, "reconcile_stuck_reset", "", map[string]any{"count": count})
	}
}

func (r *Reconciler) writeAudit(ctx context.Context, action, issueKey string, detail map[string]any) {
	raw, _ := json.Marshal(detail)
	entry := domain.AuditEntry{
		ID:        uuid.NewString(),
		Action:    action,
		IssueKey:  issueKey,
		Detail:    raw,
		CreatedAt: time.Now(),
	}
	if err := r.audit.Insert(ctx, entry); err != nil {
		r.logger.Error("reconcile: write audit log", "action", action, "error", err)
	}
}
