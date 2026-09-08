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
		`SELECT id, name, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET name = $1 WHERE id = $2`,
		name, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return r.GetByID(ctx, id)
}
