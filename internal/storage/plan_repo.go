package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Plan struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type PlanRepo struct {
	pool *pgxpool.Pool
}

func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo {
	return &PlanRepo{pool: pool}
}

func (r *PlanRepo) Create(ctx context.Context, userID uuid.UUID, title, content string) (*Plan, error) {
	p := &Plan{
		ID:     uuid.New(),
		UserID: userID,
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO plans (id, user_id, title, content) VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, title, content, created_at`,
		p.ID, userID, title, content,
	).Scan(&p.ID, &p.UserID, &p.Title, &p.Content, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	return p, nil
}

func (r *PlanRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Plan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, title, content, created_at FROM plans
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()

	plans, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByPos[Plan])
	if err != nil {
		return nil, fmt.Errorf("collect plans: %w", err)
	}
	return plans, nil
}

func (r *PlanRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (*Plan, error) {
	p := &Plan{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, title, content, created_at FROM plans WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&p.ID, &p.UserID, &p.Title, &p.Content, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}
