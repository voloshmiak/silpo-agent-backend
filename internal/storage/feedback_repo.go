package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DishRating struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	PlanID    uuid.UUID `json:"planId"`
	DishName  string    `json:"dishName"`
	Rating    float64   `json:"rating"`
	CreatedAt string    `json:"createdAt"`
}

type FeedbackTag struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	PlanID    uuid.UUID `json:"planId"`
	Tag       string    `json:"tag"`
	CreatedAt string    `json:"createdAt"`
}

type AdjustmentType string

const (
	AdjustmentExcludedDish AdjustmentType = "excluded_dish"
	AdjustmentSimplified   AdjustmentType = "simplified"
	AdjustmentSubstitution AdjustmentType = "substitution"
	AdjustmentCalorieCheck AdjustmentType = "calorie_check"
)

type PlanAdjustment struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"userId"`
	PlanID    uuid.UUID       `json:"planId"`
	Type      AdjustmentType  `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"createdAt"`
}

type FeedbackRepo struct {
	pool *pgxpool.Pool
}

func NewFeedbackRepo(pool *pgxpool.Pool) *FeedbackRepo {
	return &FeedbackRepo{pool: pool}
}

func (r *FeedbackRepo) UpsertDishRating(ctx context.Context, userID, planID uuid.UUID, dishName string, rating int16) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dish_ratings (id, user_id, plan_id, dish_name, rating)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, plan_id, dish_name) DO UPDATE SET rating = EXCLUDED.rating
`, uuid.New(), userID, planID, dishName, rating)
	if err != nil {
		return fmt.Errorf("upsert dish rating: %w", err)
	}
	return nil
}

func (r *FeedbackRepo) ListDishRatings(ctx context.Context, userID, planID uuid.UUID) ([]DishRating, error) {
	rows, err := r.pool.Query(ctx, `
			SELECT id, user_id, plan_id, dish_name, rating, created_at 
			FROM dish_ratings WHERE user_id = $1 AND plan_id = $2
	`, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("list dish ratings: %w", err)
	}
	defer rows.Close()

	ratings, err := pgx.CollectRows(rows, pgx.RowToStructByPos[DishRating])
	if err != nil {
		return nil, fmt.Errorf("collect dish ratings: %w", err)
	}
	return ratings, nil
}

func (r *FeedbackRepo) ToggleTag(ctx context.Context, userID uuid.UUID, planID uuid.UUID, tag string) (bool, error) {
	var existsingID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM feedback_tags WHERE user_id = $1 AND plan_id = $2 AND tag = $3
		`, userID, planID, tag).Scan(&existsingID)
	switch err {
	case nil:
		if _, err := r.pool.Exec(ctx, `DELETE FROM feedback_tags WHERE id = $1`, existsingID); err != nil {
			return false, fmt.Errorf("remove tag: %w", err)
		}
		return false, nil
	case pgx.ErrNoRows:
		_, err := r.pool.Exec(ctx, `
			INSERT INTO feedback_tags (id, user_id, plan_id, tag) VALUES ($1, $2, $3, $4)
			`, uuid.New(), userID, planID, tag)
		if err != nil {
			return false, fmt.Errorf("add tag: %w", err)
		}
		return false, nil
	default:
		return false, fmt.Errorf("check existing tag: %w", err)
	}
}

func (r *FeedbackRepo) ListTags(ctx context.Context, userID, planID uuid.UUID) ([]FeedbackTag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, plan_id, tag, created_at
		FROM feedback_tags WHERE user_id = $1 AND plan_id = $2
`, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags, err := pgx.CollectRows(rows, pgx.RowToStructByPos[FeedbackTag])
	if err != nil {
		return nil, fmt.Errorf("collect tags: %w", err)
	}
	return tags, nil
}

func (r *FeedbackRepo) CreatedAdjustment(ctx context.Context, userID, planID uuid.UUID, adjType AdjustmentType, payload json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO plan_adjustments (id, user_id, plan_id, type, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), userID, planID, adjType, payload)
	if err != nil {
		return fmt.Errorf("create adjustment: %w", err)
	}
	return nil
}

func (r *FeedbackRepo) ListAdjustments(ctx context.Context, userID, planID uuid.UUID) ([]PlanAdjustment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, plan_id, type, payload, created_at
		FROM plan_adjustments WHERE user_id = $1 AND plan_id = $2
		ORDER BY created_at
	`, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("list adjustments: %w", err)
	}
	defer rows.Close()

	adjustments, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PlanAdjustment])
	if err != nil {
		return nil, fmt.Errorf("collect adjustments: %w", err)
	}
	return adjustments, nil
}
