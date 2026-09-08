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
          created_at TIMESTAMPTZ DEFAULT NOW()
       );

       ALTER TABLE users DROP COLUMN IF EXISTS weight;
       ALTER TABLE users DROP COLUMN IF EXISTS height;

       -- Единая правильная таблица настроек с поддержкой категорий и черновиков
       CREATE TABLE IF NOT EXISTS user_settings (
          id             UUID PRIMARY KEY,
          user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
          category       TEXT NOT NULL CHECK (category IN ('physical', 'sport', 'diet', 'budget')),
          payload        JSONB NOT NULL,
          version        INT NOT NULL DEFAULT 1,
          effective_from TIMESTAMPTZ NOT NULL,
          is_draft       BOOLEAN NOT NULL DEFAULT false,
          created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
       );

       CREATE UNIQUE INDEX IF NOT EXISTS idx_user_settings_draft
          ON user_settings (user_id, category)
          WHERE is_draft = true;

       CREATE INDEX IF NOT EXISTS idx_user_settings_active
          ON user_settings (user_id, category, effective_from DESC, version DESC)
          WHERE is_draft = false;

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

       CREATE TABLE IF NOT EXISTS dish_ratings (
          id         UUID PRIMARY KEY,
          user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
          plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
          dish_name  TEXT NOT NULL,
          rating     SMALLINT NOT NULL CHECK (rating IN (-1, 0, 1)),
          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
          UNIQUE (user_id, plan_id, dish_name)
       );

       CREATE TABLE IF NOT EXISTS feedback_tags (
          id         UUID PRIMARY KEY,
          user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
          plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
          tag        TEXT NOT NULL,
          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
          UNIQUE (user_id, plan_id, tag)
       );

       CREATE TABLE IF NOT EXISTS plan_adjustments (
          id         UUID PRIMARY KEY,
          user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
          plan_id    UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
          type       TEXT NOT NULL CHECK (type IN ('excluded_dish', 'simplified', 'substitution', 'calorie_check')),
          payload    JSONB NOT NULL,
          created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
       );
    `)
	return err
}
