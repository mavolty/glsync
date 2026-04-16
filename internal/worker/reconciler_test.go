package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/worker"
)

// noopJobRepo is a no-op implementation of store.JobRepository for reconciler tests.
type noopJobRepo struct{}

func (noopJobRepo) Enqueue(_ context.Context, _ domain.Job) error                    { return nil }
func (noopJobRepo) Dequeue(_ context.Context, _ int) ([]domain.Job, error)           { return nil, nil }
func (noopJobRepo) MarkCompleted(_ context.Context, _ string) error                  { return nil }
func (noopJobRepo) MarkFailed(_ context.Context, _ string, _ string, _ time.Time) error { return nil }
func (noopJobRepo) MarkDead(_ context.Context, _ string, _ string) error             { return nil }
func (noopJobRepo) ListFailed(_ context.Context, _ int) ([]domain.Job, error)        { return nil, nil }
func (noopJobRepo) ListStale(_ context.Context, _ time.Duration) ([]domain.Job, error) {
	return nil, nil
}
func (noopJobRepo) ResetStuck(_ context.Context, _ time.Duration) (int64, error) { return 0, nil }

// noopAuditRepo is a no-op implementation of store.AuditRepository for reconciler tests.
type noopAuditRepo struct{}

func (noopAuditRepo) Insert(_ context.Context, _ domain.AuditEntry) error { return nil }

// countingJobRepo counts calls to ResetStuck and optionally returns an error.
type countingJobRepo struct {
	noopJobRepo
	resetCalls atomic.Int32
	resetErr   error
	resetCount int64
}

func (r *countingJobRepo) ResetStuck(_ context.Context, _ time.Duration) (int64, error) {
	r.resetCalls.Add(1)
	return r.resetCount, r.resetErr
}

func TestReconciler_RunCallsResetStuck(t *testing.T) {
	repo := &countingJobRepo{resetCount: 0}
	audit := &noopAuditRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec := worker.NewReconciler(repo, audit, worker.ReconcileConfig{
		Interval:        10 * time.Millisecond,
		StuckJobTimeout: 5 * time.Minute,
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	rec.Run(ctx)

	assert.GreaterOrEqual(t, int(repo.resetCalls.Load()), 1)
}

func TestReconciler_ResetStuckError_DoesNotPanic(t *testing.T) {
	repo := &countingJobRepo{resetErr: errors.New("db down")}
	audit := &noopAuditRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec := worker.NewReconciler(repo, audit, worker.ReconcileConfig{
		Interval:        10 * time.Millisecond,
		StuckJobTimeout: 5 * time.Minute,
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	assert.NotPanics(t, func() { rec.Run(ctx) })
	assert.GreaterOrEqual(t, int(repo.resetCalls.Load()), 1)
}

func TestReconciler_WritesAuditWhenJobsReset(t *testing.T) {
	repo := &countingJobRepo{resetCount: 3} // simulate 3 stuck jobs reset
	auditRepo := &capturingAuditRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	rec := worker.NewReconciler(repo, auditRepo, worker.ReconcileConfig{
		Interval:        10 * time.Millisecond,
		StuckJobTimeout: 5 * time.Minute,
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	rec.Run(ctx)

	assert.GreaterOrEqual(t, auditRepo.insertCount.Load(), int32(1))
}

// capturingAuditRepo records Insert calls.
type capturingAuditRepo struct {
	insertCount atomic.Int32
}

func (r *capturingAuditRepo) Insert(_ context.Context, _ domain.AuditEntry) error {
	r.insertCount.Add(1)
	return nil
}
