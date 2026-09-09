package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never expose in JSON
	CreatedAt    time.Time `json:"created_at"`
}

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, name, email, passwordHash string) (*User, error) {
	u := &User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (id, name, email, password_hash)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		RETURNING id, name, COALESCE(email, ''), COALESCE(password_hash, ''), created_at
	`, u.ID, u.Name, u.Email, u.PasswordHash).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(email, ''), COALESCE(password_hash, ''), created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(email, ''), COALESCE(password_hash, ''), created_at
		FROM users WHERE LOWER(email) = LOWER($1)
	`, email).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
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

func (r *UserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, newPasswordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		newPasswordHash, id,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
