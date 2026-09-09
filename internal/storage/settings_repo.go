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

func (r *SettingsRepo) Reset(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM user_settings WHERE user_id = $1 AND is_draft = true`, userID)
	if err != nil {
		return fmt.Errorf("reset drafts: %w", err)
	}
	return nil
}

type PhysicalSettings struct {
	Weight       float64 `json:"weight"`
	TargetWeight float64 `json:"target_weight"`
	Height       float64 `json:"height"`
	Age          int     `json:"age"`
	Sex          string  `json:"sex"`
	Focus        string  `json:"focus"`
	WeeklyPace   float64 `json:"weekly_pace"`
}

type SportSettings struct {
	WorkoutsPerWeek    int               `json:"workouts_per_week"`
	WorkoutSchedule    map[string]string `json:"workout_schedule"`
	MissedWorkoutToday bool              `json:"missed_workout_today"`
}

type DietSettings struct {
	Allergens        []string `json:"allergens"`
	ExcludedProducts []string `json:"excluded_products"`
	DietType         string   `json:"diet_type"`
}

type BudgetSettings struct {
	WeeklyBudget     float64 `json:"weekly_budget"`
	PromoPriority    string  `json:"promo_priority"`
	DeliveryIncluded bool    `json:"delivery_included"`
}

type FlatSettingsInput struct {
	Weight             float64           `json:"weight"`
	TargetWeight       float64           `json:"target_weight"`
	Height             float64           `json:"height"`
	Age                int               `json:"age"`
	Sex                string            `json:"sex"`
	Focus              string            `json:"focus"`
	WeeklyPace         float64           `json:"weekly_pace"`
	WorkoutsPerWeek    int               `json:"workouts_per_week"`
	WorkoutSchedule    map[string]string `json:"workout_schedule"`
	MissedWorkoutToday bool              `json:"missed_workout_today"`
	Allergens          []string          `json:"allergens"`
	ExcludedProducts   []string          `json:"excluded_products"`
	DietType           string            `json:"diet_type"`
	WeeklyBudget       float64           `json:"weekly_budget"`
	PromoPriority      string            `json:"promo_priority"`
	DeliveryIncluded   bool              `json:"delivery_included"`
}
type FlatSettings struct {
	UserID uuid.UUID `json:"user_id"`
	FlatSettingsInput
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *SettingsRepo) getActiveByCategory(ctx context.Context, userID uuid.UUID) (map[SettingsCategory]json.RawMessage, time.Time, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (category)
			category, payload, created_at
		FROM user_settings
		WHERE user_id = $1 AND is_draft = false
		ORDER BY category, effective_from DESC, version DESC
	`, userID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("query active settings: %w", err)
	}
	defer rows.Close()

	result := make(map[SettingsCategory]json.RawMessage)
	var updatedAt time.Time
	for rows.Next() {
		var category SettingsCategory
		var payload json.RawMessage
		var createdAt time.Time
		if err := rows.Scan(&category, &payload, &createdAt); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan active setting: %w", err)
		}
		result[category] = payload
		if createdAt.After(updatedAt) {
			updatedAt = createdAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, fmt.Errorf("iterate active settings: %w", err)
	}
	return result, updatedAt, nil
}

func (r *SettingsRepo) GetFlat(ctx context.Context, userID uuid.UUID) (*FlatSettings, error) {
	byCategory, updatedAt, err := r.getActiveByCategory(ctx, userID)
	if err != nil {
		return nil, err
	}

	flat := &FlatSettings{UserID: userID, UpdatedAt: updatedAt}
	flat.Allergens = []string{}
	flat.ExcludedProducts = []string{}
	flat.WorkoutSchedule = map[string]string{}

	if raw, ok := byCategory[CategoryPhysical]; ok {
		var p PhysicalSettings
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("unmarshal physical settings: %w", err)
		}
		flat.Weight = p.Weight
		flat.TargetWeight = p.TargetWeight
		flat.Height = p.Height
		flat.Age = p.Age
		flat.Sex = p.Sex
		flat.Focus = p.Focus
		flat.WeeklyPace = p.WeeklyPace
	}
	if raw, ok := byCategory[CategorySport]; ok {
		var s SportSettings
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("unmarshal sport settings: %w", err)
		}
		flat.WorkoutsPerWeek = s.WorkoutsPerWeek
		if s.WorkoutSchedule != nil {
			flat.WorkoutSchedule = s.WorkoutSchedule
		}
		flat.MissedWorkoutToday = s.MissedWorkoutToday
	}
	if raw, ok := byCategory[CategoryDiet]; ok {
		var d DietSettings
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("unmarshal diet settings: %w", err)
		}
		if d.Allergens != nil {
			flat.Allergens = d.Allergens
		}
		if d.ExcludedProducts != nil {
			flat.ExcludedProducts = d.ExcludedProducts
		}
		flat.DietType = d.DietType
	}
	if raw, ok := byCategory[CategoryBudget]; ok {
		var b BudgetSettings
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, fmt.Errorf("unmarshal budget settings: %w", err)
		}
		flat.WeeklyBudget = b.WeeklyBudget
		flat.PromoPriority = b.PromoPriority
		flat.DeliveryIncluded = b.DeliveryIncluded
	}

	return flat, nil
}

func (r *SettingsRepo) PutFlat(ctx context.Context, userID uuid.UUID, in FlatSettingsInput) (*FlatSettings, error) {
	payloads := map[SettingsCategory]any{
		CategoryPhysical: PhysicalSettings{
			Weight:       in.Weight,
			TargetWeight: in.TargetWeight,
			Height:       in.Height,
			Age:          in.Age,
			Sex:          in.Sex,
			Focus:        in.Focus,
			WeeklyPace:   in.WeeklyPace,
		},
		CategorySport: SportSettings{
			WorkoutsPerWeek:    in.WorkoutsPerWeek,
			WorkoutSchedule:    in.WorkoutSchedule,
			MissedWorkoutToday: in.MissedWorkoutToday,
		},
		CategoryDiet: DietSettings{
			Allergens:        in.Allergens,
			ExcludedProducts: in.ExcludedProducts,
			DietType:         in.DietType,
		},
		CategoryBudget: BudgetSettings{
			WeeklyBudget:     in.WeeklyBudget,
			PromoPriority:    in.PromoPriority,
			DeliveryIncluded: in.DeliveryIncluded,
		},
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	for category, data := range payloads {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("marshal %s payload: %w", category, err)
		}

		var lastVersion int
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(version), 0) FROM user_settings
			WHERE user_id = $1 AND category = $2 AND is_draft = false
		`, userID, category).Scan(&lastVersion)
		if err != nil {
			return nil, fmt.Errorf("get last version for %s: %w", category, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO user_settings (id, user_id, category, payload, version, effective_from, is_draft)
			VALUES ($1, $2, $3, $4, $5, $6, false)
		`, uuid.New(), userID, category, raw, lastVersion+1, now)
		if err != nil {
			return nil, fmt.Errorf("insert %s settings: %w", category, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit settings update: %w", err)
	}

	return r.GetFlat(ctx, userID)
}
