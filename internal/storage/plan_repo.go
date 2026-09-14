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
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	WeekNumber    int       `json:"week_number"`
	WeekStartDate time.Time `json:"week_start_date"`
	CreatedAt     time.Time `json:"created_at"`
}

type PlanRepo struct {
	pool *pgxpool.Pool
}

func NewPlanRepo(pool *pgxpool.Pool) *PlanRepo {
	return &PlanRepo{pool: pool}
}

func mondayOf(t time.Time) time.Time {
	t = t.UTC()
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysSinceMonday := weekday - 1
	monday := t.AddDate(0, 0, -daysSinceMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// WeekStart is the Monday (UTC) of the week a plan is made for: the current
// week, or the one after it when the user plans ahead — typically on a Sunday,
// when the groceries for the coming week get bought.
func WeekStart(now time.Time, next bool) time.Time {
	monday := mondayOf(now)
	if next {
		return monday.AddDate(0, 0, 7)
	}
	return monday
}

// NextWeekInfo numbers a plan for the week starting at weekStart. Only plans up
// to that week count: another plan for the same week keeps its number, a plan
// for the week right before continues the count, and anything older — or no
// plan at all — starts over at 1. Plans already made for later weeks are
// ignored, so planning ahead on a Sunday doesn't renumber the current week.
func (r *PlanRepo) NextWeekInfo(ctx context.Context, userID uuid.UUID, weekStart time.Time) (int, error) {
	var lastWeekNumber int
	var lastWeekStart time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT week_number, week_start_date
		FROM plans
		WHERE user_id = $1 AND week_start_date <= $2
		ORDER BY week_start_date DESC, created_at DESC
		LIMIT 1
	`, userID, weekStart).Scan(&lastWeekNumber, &lastWeekStart)

	switch {
	case err == pgx.ErrNoRows:
		return 1, nil
	case err != nil:
		return 0, fmt.Errorf("get latest plan week info: %w", err)
	}

	lastWeekStart = lastWeekStart.UTC()
	switch {
	case lastWeekStart.Equal(weekStart):
		return lastWeekNumber, nil
	case lastWeekStart.Equal(weekStart.AddDate(0, 0, -7)):
		return lastWeekNumber + 1, nil
	default:
		return 1, nil
	}
}

func (r *PlanRepo) Create(ctx context.Context, userID uuid.UUID, title, content string, weekNumber int, weekStartDate time.Time) (*Plan, error) {
	p := &Plan{
		ID:     uuid.New(),
		UserID: userID,
	}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO plans (id, user_id, title, content, week_number, week_start_date)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, title, content, week_number, week_start_date, created_at`,
		p.ID, userID, title, content, weekNumber, weekStartDate,
	).Scan(&p.ID, &p.UserID, &p.Title, &p.Content, &p.WeekNumber, &p.WeekStartDate, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	return p, nil
}

func (r *PlanRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Plan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, title, content, week_number, week_start_date, created_at FROM plans
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
		`SELECT id, user_id, title, content, week_number, week_start_date, created_at FROM plans WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&p.ID, &p.UserID, &p.Title, &p.Content, &p.WeekNumber, &p.WeekStartDate, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}
