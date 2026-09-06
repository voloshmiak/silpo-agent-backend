package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SilpoToken struct {
	UserID       uuid.UUID  `json:"user_id"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresAt    *time.Time `json:"expires_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type TokenRepo struct {
	pool *pgxpool.Pool
}

func NewTokenRepo(pool *pgxpool.Pool) *TokenRepo {
	return &TokenRepo{pool: pool}
}

func (r *TokenRepo) Upsert(ctx context.Context, t *SilpoToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO silpo_tokens (user_id, access_token, refresh_token, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE
		  SET access_token  = EXCLUDED.access_token,
		      refresh_token = EXCLUDED.refresh_token,
		      expires_at    = EXCLUDED.expires_at,
		      updated_at    = NOW()
	`, t.UserID, t.AccessToken, t.RefreshToken, t.ExpiresAt)
	if err != nil {
		return fmt.Errorf("upsert silpo token: %w", err)
	}
	return nil
}

func (r *TokenRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*SilpoToken, error) {
	t := &SilpoToken{}
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, access_token, refresh_token, expires_at, updated_at FROM silpo_tokens WHERE user_id = $1`,
		userID,
	).Scan(&t.UserID, &t.AccessToken, &t.RefreshToken, &t.ExpiresAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get silpo token: %w", err)
	}
	return t, nil
}
