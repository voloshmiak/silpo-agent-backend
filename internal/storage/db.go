package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	if err := migrate(ctx, pool); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return pool, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id         UUID PRIMARY KEY,
			name       TEXT,
			weight     DOUBLE PRECISION,
			height     DOUBLE PRECISION,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS silpo_tokens (
			user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			access_token  TEXT NOT NULL,
			refresh_token TEXT,
			expires_at    TIMESTAMPTZ,
			updated_at    TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS plans (
			id         UUID PRIMARY KEY,
			user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
			title      TEXT,
			content    TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);
	`)
	return err
}
