package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mavolty/glsync/internal/domain"
)

type AuditRepository interface {
	Insert(ctx context.Context, entry domain.AuditEntry) error
}

type pgAuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) AuditRepository {
	return &pgAuditRepo{pool: pool}
}

func (r *pgAuditRepo) Insert(ctx context.Context, e domain.AuditEntry) error {
	eventID := nullableString(e.EventID)
	jobID := nullableString(e.JobID)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, event_id, job_id, action, issue_key, detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, eventID, jobID, e.Action, e.IssueKey, e.Detail, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// nullableString returns nil for empty strings so PostgreSQL stores NULL instead of empty text.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
