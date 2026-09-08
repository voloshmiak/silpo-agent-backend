package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsCategory string

const (
	CategoryPhysical SettingsCategory = "physical"
	CategorySport    SettingsCategory = "sport"
	CategoryDiet     SettingsCategory = "diet"
	CategoryBudget   SettingsCategory = "budget"
)

func IsValidSettingsCategory(c SettingsCategory) bool {
	switch c {
	case CategoryPhysical, CategorySport, CategoryDiet, CategoryBudget:
		return true
	default:
		return false
	}
}

type UserSettings struct {
	ID            uuid.UUID        `json:"id"`
	UserID        uuid.UUID        `json:"user_id"`
	Category      SettingsCategory `json:"category"`
	Payload       json.RawMessage  `json:"payload"`
	Version       int              `json:"version"`
	EffectiveFrom time.Time        `json:"effective_from"`
	IsDraft       bool             `json:"is_draft"`
	CreatedAt     time.Time        `json:"created_at"`
}

type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) UpsertDraft(ctx context.Context, userID uuid.UUID, category SettingsCategory, payload json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_settings (id, user_id, category, payload, version, effective_from, is_draft)
		VALUES ($1, $2, $3, $4, 1, now(), true)
		ON CONFLICT (user_id, category) WHERE is_draft = true
		DO UPDATE SET payload = EXCLUDED.payload
    `, uuid.New(), userID, category, payload)
	if err != nil {
		return fmt.Errorf("upsert draft settings: %w", err)
	}
	return nil
}

func (r *SettingsRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]*UserSettings, error) {
	rows, err := r.pool.Query(ctx, `
	SELECT DISTINCT ON (category, is_draft)
		id, user_id, category, payload, version, effective_from, is_draft, created_at
	FROM user_settings
	WHERE user_id = $1
	ORDER BY category, is_draft, effective_from DESC, version DESC 
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()

	settings, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByPos[UserSettings])
	if err != nil {
		return nil, fmt.Errorf("collect settings: %w", err)
	}
	return settings, nil
}

func (r *SettingsRepo) Apply(ctx context.Context, userID uuid.UUID, effectiveFrom time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT category, payload FROM user_settings
		WHERE user_id = $1 AND is_draft = true
		`, userID)
	if err != nil {
		return fmt.Errorf("query drafts: %w", err)
	}

	type draft struct {
		Category SettingsCategory
		Payload  json.RawMessage
	}
	var drafts []draft
	for rows.Next() {
		var d draft
		if err := rows.Scan(&d.Category, &d.Payload); err != nil {
			rows.Close()
			return fmt.Errorf("scan drft: %w", err)
		}
		drafts = append(drafts, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate drfts: %w", err)
	}

	for _, d := range drafts {
		var lastVersion int
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(version), 0) FROM user_settings
			WHERE user_id = $1 AND category = $2 and is_draft = false
			`, userID, d.Category).Scan(&lastVersion)
		if err != nil {
			return fmt.Errorf("get last version for %s: %w", d.Category, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO user_settings (id, user_id, category, payload, version, effective_from, is_draft)
			VALUES ($1, $2, $3, $4, $5, $6, false)
		`, uuid.New(), userID, d.Category, d.Payload, lastVersion+1, effectiveFrom)
		if err != nil {
			return fmt.Errorf("insert active version for %s: %w", d.Category, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1 AND is_draft = true`, userID); err != nil {
		return fmt.Errorf("clear drafts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit apply: %w", err)
	}
	return nil
}

// очистка черновиков
func (r *SettingsRepo) Reset(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1 AND is_draft = true`, userID)
	if err != nil {
		return fmt.Errorf("reset drafts: %w", err)
	}
	return nil
}
