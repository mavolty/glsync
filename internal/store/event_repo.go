package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
)

type EventRepository interface {
	Insert(ctx context.Context, event domain.NormalizedEvent) error
	ExistsByIdempotencyKey(ctx context.Context, key string) (bool, error)
	GetByID(ctx context.Context, id string) (*domain.NormalizedEvent, error)
	ListRecent(ctx context.Context, limit int) ([]domain.NormalizedEvent, error)
}

type pgEventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) EventRepository {
	return &pgEventRepo{pool: pool}
}

func (r *pgEventRepo) Insert(ctx context.Context, e domain.NormalizedEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO events (
			id, idempotency_key, event_type, issue_keys,
			source_branch, target_branch, mr_title, mr_iid,
			project_id, author_email, raw_payload, received_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		e.ID, e.IdempotencyKey, string(e.EventType), e.IssueKeys,
		e.SourceBranch, e.TargetBranch, e.MRTitle, e.MRIID,
		e.ProjectID, e.AuthorEmail, e.RawPayload, e.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (r *pgEventRepo) ExistsByIdempotencyKey(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM events WHERE idempotency_key = $1)`, key,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check idempotency key: %w", err)
	}
	return exists, nil
}

func (r *pgEventRepo) GetByID(ctx context.Context, id string) (*domain.NormalizedEvent, error) {
	row, err := r.pool.Query(ctx, `
		SELECT id, idempotency_key, event_type, issue_keys,
		       source_branch, target_branch, mr_title, mr_iid,
		       project_id, author_email, raw_payload, received_at
		FROM events WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("query event by id: %w", err)
	}
	defer row.Close()

	e, err := pgx.CollectOneRow(row, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}
	return &e, nil
}

func (r *pgEventRepo) ListRecent(ctx context.Context, limit int) ([]domain.NormalizedEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, idempotency_key, event_type, issue_keys,
		       source_branch, target_branch, mr_title, mr_iid,
		       project_id, author_email, raw_payload, received_at
		FROM events ORDER BY received_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent events: %w", err)
	}
	defer rows.Close()

	events, err := pgx.CollectRows(rows, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return events, nil
}

func scanEvent(row pgx.CollectableRow) (domain.NormalizedEvent, error) {
	var e domain.NormalizedEvent
	var eventType string
	err := row.Scan(
		&e.ID, &e.IdempotencyKey, &eventType, &e.IssueKeys,
		&e.SourceBranch, &e.TargetBranch, &e.MRTitle, &e.MRIID,
		&e.ProjectID, &e.AuthorEmail, &e.RawPayload, &e.ReceivedAt,
	)
	e.EventType = domain.EventType(eventType)
	return e, err
}
