package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DishRating struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CookedTimes int    `json:"cookedTimes"`
	TimeMinutes int    `json:"timeMinutes"`
	Rating      string `json:"rating"` // "good", "neutral", "bad"
}
type Feedback struct {
	ID          uuid.UUID    `json:"id"`
	UserID      uuid.UUID    `json:"user_id"`
	PlanID      *uuid.UUID   `json:"plan_id,omitempty"`
	DishRatings []DishRating `json:"dish_ratings"`
	Tags        []string     `json:"tags"`
	CreatedAt   time.Time    `json:"created_at"`
}

type FeedbackRepo struct {
	pool *pgxpool.Pool
}

func NewFeedbackRepo(pool *pgxpool.Pool) *FeedbackRepo {
	return &FeedbackRepo{pool: pool}
}

func (r *FeedbackRepo) Create(ctx context.Context, f *Feedback) (*Feedback, error) {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.DishRatings == nil {
		f.DishRatings = []DishRating{}
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}

	dishRatingsJSON, err := json.Marshal(f.DishRatings)
	if err != nil {
		dishRatingsJSON = []byte("[]")
	}

	err = r.pool.QueryRow(ctx, `
		INSERT INTO feedbacks (id, user_id, plan_id, dish_ratings, tags, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, user_id, plan_id, created_at
	`, f.ID, f.UserID, f.PlanID, dishRatingsJSON, f.Tags).Scan(
		&f.ID, &f.UserID, &f.PlanID, &f.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create feedback: %w", err)
	}

	return f, nil
}

func (r *FeedbackRepo) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*Feedback, error) {
	f := &Feedback{}
	var dishRatingsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, plan_id, dish_ratings, tags, created_at
		FROM feedbacks
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&f.ID, &f.UserID, &f.PlanID, &dishRatingsJSON, &f.Tags, &f.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest feedback: %w", err)
	}

	_ = json.Unmarshal(dishRatingsJSON, &f.DishRatings)
	if f.Tags == nil {
		f.Tags = []string{}
	}

	return f, nil
}

func (r *FeedbackRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Feedback, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, plan_id, dish_ratings, tags, created_at
		FROM feedbacks
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list feedbacks: %w", err)
	}
	defer rows.Close()

	var feedbacks []*Feedback
	for rows.Next() {
		f := &Feedback{}
		var dishRatingsJSON []byte
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.PlanID, &dishRatingsJSON, &f.Tags, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan feedback: %w", err)
		}
		_ = json.Unmarshal(dishRatingsJSON, &f.DishRatings)
		if f.Tags == nil {
			f.Tags = []string{}
		}
		feedbacks = append(feedbacks, f)
	}

	return feedbacks, nil
}
