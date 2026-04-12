package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
)

type JobRepository interface {
	Enqueue(ctx context.Context, job domain.Job) error
	Dequeue(ctx context.Context, batchSize int) ([]domain.Job, error)
	MarkCompleted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, errMsg string, nextRunAt time.Time) error
	MarkDead(ctx context.Context, id string, errMsg string) error
	ListFailed(ctx context.Context) ([]domain.Job, error)
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

// Dequeue atomically claims pending jobs for processing using SELECT FOR UPDATE SKIP LOCKED.
// This prevents multiple workers from picking up the same job.
func (r *pgJobRepo) Dequeue(ctx context.Context, batchSize int) ([]domain.Job, error) {
	rows, err := r.pool.Query(ctx, `
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

	// Mark all dequeued jobs as running
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	_, err = r.pool.Exec(ctx, `UPDATE jobs SET status = 'running' WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("mark jobs running: %w", err)
	}

	return jobs, nil
}

func (r *pgJobRepo) MarkCompleted(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'completed', completed_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark job completed: %w", err)
	}
	return nil
}

func (r *pgJobRepo) MarkFailed(ctx context.Context, id string, errMsg string, nextRunAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed', last_error = $2, next_run_at = $3,
		    attempts = attempts + 1
		WHERE id = $1`, id, errMsg, nextRunAt)
	if err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	return nil
}

func (r *pgJobRepo) MarkDead(ctx context.Context, id string, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE jobs SET status = 'dead', last_error = $2, attempts = attempts + 1 WHERE id = $1`,
		id, errMsg)
	if err != nil {
		return fmt.Errorf("mark job dead: %w", err)
	}
	return nil
}

func (r *pgJobRepo) ListFailed(ctx context.Context) ([]domain.Job, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, job_type, status, issue_key, target_state,
		       payload, attempts, max_attempts, last_error, next_run_at, created_at, completed_at
		FROM jobs WHERE status IN ('failed', 'dead') ORDER BY created_at DESC`)
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
func (r *pgJobRepo) ResetStuck(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.pool.Exec(ctx, `
		UPDATE jobs SET status = 'pending', next_run_at = now()
		WHERE status = 'running' AND next_run_at < $1`, cutoff)
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
