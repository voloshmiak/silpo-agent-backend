package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WeightRecord struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Weight     float64   `json:"weight"`
	RecordedAt time.Time `json:"recorded_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type ExpenseRecord struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	PlanID      *uuid.UUID `json:"plan_id,omitempty"`
	WeekNumber  int        `json:"week_number"`
	WeekLabel   string     `json:"week_label"`
	TotalCost   float64    `json:"total_cost"`
	BudgetLimit float64    `json:"budget_limit"`
	IsOverspent bool       `json:"is_overspent"`
	RecordedAt  time.Time  `json:"recorded_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ProgressRepo struct {
	pool *pgxpool.Pool
}

func NewProgressRepo(pool *pgxpool.Pool) *ProgressRepo {
	return &ProgressRepo{pool: pool}
}

// RecordWeight inserts or updates the weight for the specified day.
func (r *ProgressRepo) RecordWeight(ctx context.Context, userID uuid.UUID, weight float64, recordedAt time.Time) (*WeightRecord, error) {
	if recordedAt.IsZero() {
		recordedAt = time.Now()
	}
	// Truncate to date only
	recordedAtDate := time.Date(recordedAt.Year(), recordedAt.Month(), recordedAt.Day(), 0, 0, 0, 0, time.UTC)
	recID := uuid.New()

	rec := &WeightRecord{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO weight_history (id, user_id, weight, recorded_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, recorded_at) DO UPDATE SET
			weight = EXCLUDED.weight
		RETURNING id, user_id, weight, recorded_at, created_at
	`, recID, userID, weight, recordedAtDate).Scan(
		&rec.ID, &rec.UserID, &rec.Weight, &rec.RecordedAt, &rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("record weight: %w", err)
	}
	return rec, nil
}

// ListWeights returns weight records ordered chronologically (ASC).
// If limit > 0, returns the most recent `limit` records in chronological order.
func (r *ProgressRepo) ListWeights(ctx context.Context, userID uuid.UUID, limit int) ([]WeightRecord, error) {
	var query string
	var args []interface{}

	if limit > 0 {
		query = `
			SELECT id, user_id, weight, recorded_at, created_at
			FROM (
				SELECT id, user_id, weight, recorded_at, created_at
				FROM weight_history
				WHERE user_id = $1
				ORDER BY recorded_at DESC
				LIMIT $2
			) sub
			ORDER BY recorded_at ASC
		`
		args = []interface{}{userID, limit}
	} else {
		query = `
			SELECT id, user_id, weight, recorded_at, created_at
			FROM weight_history
			WHERE user_id = $1
			ORDER BY recorded_at ASC
		`
		args = []interface{}{userID}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list weights: %w", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, pgx.RowToStructByName[WeightRecord])
	if err != nil {
		return nil, fmt.Errorf("collect weights: %w", err)
	}
	return records, nil
}

// GetEarliestWeight returns the first recorded weight for the user.
func (r *ProgressRepo) GetEarliestWeight(ctx context.Context, userID uuid.UUID) (*WeightRecord, error) {
	rec := &WeightRecord{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, weight, recorded_at, created_at
		FROM weight_history
		WHERE user_id = $1
		ORDER BY recorded_at ASC
		LIMIT 1
	`, userID).Scan(&rec.ID, &rec.UserID, &rec.Weight, &rec.RecordedAt, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// RecordExpense saves a weekly expense entry.
func (r *ProgressRepo) RecordExpense(ctx context.Context, exp *ExpenseRecord) (*ExpenseRecord, error) {
	if exp.ID == uuid.Nil {
		exp.ID = uuid.New()
	}
	if exp.RecordedAt.IsZero() {
		exp.RecordedAt = time.Now()
	}
	exp.IsOverspent = exp.TotalCost > exp.BudgetLimit

	err := r.pool.QueryRow(ctx, `
		INSERT INTO weekly_expenses (id, user_id, plan_id, week_number, week_label, total_cost, budget_limit, is_overspent, recorded_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, user_id, plan_id, week_number, week_label, total_cost, budget_limit, is_overspent, recorded_at, created_at
	`, exp.ID, exp.UserID, exp.PlanID, exp.WeekNumber, exp.WeekLabel, exp.TotalCost, exp.BudgetLimit, exp.IsOverspent, exp.RecordedAt).Scan(
		&exp.ID, &exp.UserID, &exp.PlanID, &exp.WeekNumber, &exp.WeekLabel, &exp.TotalCost, &exp.BudgetLimit, &exp.IsOverspent, &exp.RecordedAt, &exp.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("record expense: %w", err)
	}
	return exp, nil
}

// ListExpenses returns weekly expenses ordered chronologically (ASC).
// If limit > 0, returns the most recent `limit` records in chronological order.
func (r *ProgressRepo) ListExpenses(ctx context.Context, userID uuid.UUID, limit int) ([]ExpenseRecord, error) {
	var query string
	var args []interface{}

	if limit > 0 {
		query = `
			SELECT id, user_id, plan_id, week_number, week_label, total_cost, budget_limit, is_overspent, recorded_at, created_at
			FROM (
				SELECT id, user_id, plan_id, week_number, week_label, total_cost, budget_limit, is_overspent, recorded_at, created_at
				FROM weekly_expenses
				WHERE user_id = $1
				ORDER BY recorded_at DESC, week_number DESC
				LIMIT $2
			) sub
			ORDER BY recorded_at ASC, week_number ASC
		`
		args = []interface{}{userID, limit}
	} else {
		query = `
			SELECT id, user_id, plan_id, week_number, week_label, total_cost, budget_limit, is_overspent, recorded_at, created_at
			FROM weekly_expenses
			WHERE user_id = $1
			ORDER BY recorded_at ASC, week_number ASC
		`
		args = []interface{}{userID}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	records, err := pgx.CollectRows(rows, pgx.RowToStructByName[ExpenseRecord])
	if err != nil {
		return nil, fmt.Errorf("collect expenses: %w", err)
	}
	return records, nil
}

// CountExpenses returns the total count of weekly expenses for the user.
func (r *ProgressRepo) CountExpenses(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM weekly_expenses WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count expenses: %w", err)
	}
	return count, nil
}
