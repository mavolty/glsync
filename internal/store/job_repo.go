package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mavolty/glsync/internal/domain"
)

type JobRepository interface {
	Enqueue(ctx context.Context, job domain.Job) error
	Dequeue(ctx context.Context, batchSize int) ([]domain.Job, error)
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string, nextRunAt time.Time) error
	MarkDead(ctx context.Context, id string, errMsg string) error
	ListFailed(ctx context.Context, limit int) ([]domain.Job, error)
	ListStale(ctx context.Context, olderThan time.Duration) ([]domain.Job, error)
	ResetStuck(ctx context.Context, olderThan time.Duration) (int64, error)
}

type pgJobRepo struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) JobRepository {
	return &pgJobRepo{pool: pool}
}

func (r *pgJobRepo) Enqueue(ctx context.Context, j domain.Job) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO jobs (
			id, event_id, job_type, status, issue_key, target_state,
			payload, attempts, max_attempts, last_error, next_run_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		j.ID, j.EventID, string(j.Type), string(j.Status),
		j.IssueKey, string(j.TargetState),
		j.Payload, j.Attempts, j.MaxAttempts, j.LastError,
		j.NextRunAt, j.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}
	return nil
}

// Dequeue atomically claims pending jobs for processing.
// The SELECT FOR UPDATE SKIP LOCKED and the status UPDATE run inside a single
// transaction so that row locks are never released before the status change commits.
func (r *pgJobRepo) Dequeue(ctx context.Context, batchSize int) ([]domain.Job, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin dequeue tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	rows, err := tx.Query(ctx, `
		SELECT id, event_id, job_type, status, issue_key, target_state,
		       payload, attempts, max_attempts, last_error, next_run_at, created_at, completed_at
		FROM jobs
		WHERE status IN ('pending', 'failed') AND next_run_at <= now()
		ORDER BY next_run_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, batchSize)
	if err != nil {
		return nil, fmt.Errorf("dequeue jobs: %w", err)
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, scanJob)
	if err != nil {
		return nil, fmt.Errorf("scan dequeued jobs: %w", err)
	}

	if len(jobs) == 0 {
		return jobs, nil
	}

	// Mark all dequeued jobs as running within the same transaction
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	_, err = tx.Exec(ctx, `
		UPDATE jobs SET status = 'running', running_since = now()
		WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("mark jobs running: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit dequeue tx: %w", err)
	}

	return jobs, nil
}

func (r *pgJobRepo) MarkCompleted(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'completed', completed_at = now(), running_since = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark job completed: %w", err)
	}
	return nil
}

func (r *pgJobRepo) MarkFailed(ctx context.Context, id string, errMsg string, nextRunAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed', last_error = $2, next_run_at = $3,
		    attempts = attempts + 1, running_since = NULL
		WHERE id = $1`, id, errMsg, nextRunAt)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	return nil
}

func (r *pgJobRepo) MarkDead(ctx context.Context, id string, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'dead', last_error = $2, attempts = attempts + 1, running_since = NULL WHERE id = $1`,
		id, errMsg)
	if err != nil {
		return fmt.Errorf("mark job dead: %w", err)
	}
	return nil
}

func (r *pgJobRepo) ListFailed(ctx context.Context, limit int) ([]domain.Job, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, job_type, status, issue_key, target_state,
		       payload, attempts, max_attempts, last_error, next_run_at, created_at, completed_at
		FROM jobs WHERE status IN ('failed', 'dead') ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed jobs: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scanJob)
}

func (r *pgJobRepo) ListStale(ctx context.Context, olderThan time.Duration) ([]domain.Job, error) {
	cutoff := time.Now().Add(-olderThan)
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, job_type, status, issue_key, target_state,
		       payload, attempts, max_attempts, last_error, next_run_at, created_at, completed_at
		FROM jobs WHERE status = 'running' AND next_run_at < $1`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list stale jobs: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scanJob)
}

// ResetStuck resets running jobs that have been stuck beyond the timeout back to pending.
// Uses running_since (set at dequeue time) instead of next_run_at to avoid
// resetting freshly-claimed jobs whose next_run_at is in the past.
func (r *pgJobRepo) ResetStuck(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = 'pending', next_run_at = now(), running_since = NULL
		WHERE status = 'running' AND running_since < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("reset stuck jobs: %w", err)
	}
	return result.RowsAffected(), nil
}

func scanJob(row pgx.CollectableRow) (domain.Job, error) {
	var j domain.Job
	var jobType, status, targetState string
	err := row.Scan(
		&j.ID, &j.EventID, &jobType, &status,
		&j.IssueKey, &targetState,
		&j.Payload, &j.Attempts, &j.MaxAttempts, &j.LastError,
		&j.NextRunAt, &j.CreatedAt, &j.CompletedAt,
	)
	j.Type = domain.JobType(jobType)
	j.Status = domain.JobStatus(status)
	j.TargetState = domain.WorkflowState(targetState)
	return j, err
}
