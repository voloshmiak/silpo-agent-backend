package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Weight    float64   `json:"weight"`
	Height    float64   `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, name string) (*User, error) {
	u := &User{
		ID:   uuid.New(),
		Name: name,
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, name) VALUES ($1, $2)`,
		u.ID, u.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return r.GetByID(ctx, u.ID)
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, COALESCE(weight, 0), COALESCE(height, 0), created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Weight, &u.Height, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, name string, weight, height float64) (*User, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET name = $1, weight = $2, height = $3 WHERE id = $4`,
		name, weight, height, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return r.GetByID(ctx, id)
}
