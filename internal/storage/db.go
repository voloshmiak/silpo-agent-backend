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
			id            UUID PRIMARY KEY,
			name          TEXT,
			email         TEXT UNIQUE,
			password_hash TEXT,
			created_at    TIMESTAMPTZ DEFAULT NOW()
		);

		ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT UNIQUE;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
		ALTER TABLE users DROP COLUMN IF EXISTS weight;
		ALTER TABLE users DROP COLUMN IF EXISTS height;

		CREATE TABLE IF NOT EXISTS user_settings (
			user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			-- Фізичні дані та ціль
			weight               DOUBLE PRECISION NOT NULL DEFAULT 70.0,
			target_weight        DOUBLE PRECISION NOT NULL DEFAULT 70.0,
			height               DOUBLE PRECISION NOT NULL DEFAULT 175.0,
			age                  INT NOT NULL DEFAULT 25,
			sex                  TEXT NOT NULL DEFAULT 'male',
			focus                TEXT NOT NULL DEFAULT 'Схуднення',
			weekly_pace          DOUBLE PRECISION NOT NULL DEFAULT -0.5,
			-- Спортивний режим
			workouts_per_week    INT NOT NULL DEFAULT 3,
			workout_schedule     JSONB NOT NULL DEFAULT '{}'::jsonb,
			missed_workout_today BOOLEAN NOT NULL DEFAULT false,
			-- Харчові обмеження
			allergens            TEXT[] NOT NULL DEFAULT '{}',
			excluded_products    TEXT[] NOT NULL DEFAULT '{}',
			diet_type            TEXT NOT NULL DEFAULT 'БЕЗ ОБМЕЖЕНЬ',
			-- Бюджет на тиждень
			weekly_budget        DOUBLE PRECISION NOT NULL DEFAULT 1500.0,
			promo_priority       TEXT NOT NULL DEFAULT 'Високий',
			delivery_included    BOOLEAN NOT NULL DEFAULT true,
			updated_at           TIMESTAMPTZ DEFAULT NOW()
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

 		ALTER TABLE plans ADD COLUMN IF NOT EXISTS week_number INT NOT NULL DEFAULT 1;
        ALTER TABLE plans ADD COLUMN IF NOT EXISTS week_start_date DATE NOT NULL DEFAULT CURRENT_DATE;

       	CREATE INDEX IF NOT EXISTS idx_plans_user_week
         	 ON plans (user_id, week_start_date DESC);

		CREATE TABLE IF NOT EXISTS feedbacks (
			id           UUID PRIMARY KEY,
			user_id      UUID REFERENCES users(id) ON DELETE CASCADE,
			plan_id      UUID REFERENCES plans(id) ON DELETE SET NULL,
			dish_ratings JSONB NOT NULL DEFAULT '[]'::jsonb,
			tags         TEXT[] NOT NULL DEFAULT '{}',
			summary      TEXT,
			decisions    JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at   TIMESTAMPTZ DEFAULT NOW()
		);
 			ALTER TABLE feedbacks DROP COLUMN IF EXISTS summary;
       		ALTER TABLE feedbacks DROP COLUMN IF EXISTS decisions;
	`)
	return err
}
