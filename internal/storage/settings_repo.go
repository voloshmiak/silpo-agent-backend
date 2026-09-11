package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserSettings struct {
	UserID uuid.UUID `json:"user_id"`
	// Фізичні дані та ціль
	Weight       float64 `json:"weight"`        // Поточна вага (кг)
	TargetWeight float64 `json:"target_weight"` // Цільова вага (кг)
	Height       float64 `json:"height"`        // Зріст (см)
	Age          int     `json:"age"`           // Вік
	Sex          string  `json:"sex"`           // Стать (male / female або чол. / жін.)
	Focus        string  `json:"focus"`         // Фокус: Схуднення / Підтримання / Набір маси
	WeeklyPace   float64 `json:"weekly_pace"`   // Темп: наприклад -0.6 кг / тиждень
	// Спортивний режим
	WorkoutsPerWeek    int               `json:"workouts_per_week"`    // Кількість занять на тиждень
	WorkoutSchedule    map[string]string `json:"workout_schedule"`     // Розклад, наприклад {"ПН": "силові", "ВТ": "кардіо"}
	MissedWorkoutToday bool              `json:"missed_workout_today"` // Пропустив тренування сьогодні
	// Харчові обмеження
	Allergens        []string `json:"allergens"`         // Алергени, наприклад ["лактоза", "горіхи"]
	ExcludedProducts []string `json:"excluded_products"` // Стоп-продукти, наприклад ["гриби", "кінза", "печінка"]
	DietType         string   `json:"diet_type"`         // Тип харчування, наприклад "БЕЗ ОБМЕЖЕНЬ"
	// Бюджет на тиждень
	WeeklyBudget     float64 `json:"weekly_budget"`     // Ліміт витрат (грн), наприклад 2000
	PromoPriority    string  `json:"promo_priority"`    // Пріоритет акцій: Високий / Середній / Низький
	DeliveryIncluded bool    `json:"delivery_included"` // Доставка включена в бюджет
	// Службові
	UpdatedAt time.Time `json:"updated_at"` // Дата останнього оновлення
}

type SettingsRepo struct {
	pool *pgxpool.Pool
}

func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

func (r *SettingsRepo) Upsert(ctx context.Context, s *UserSettings) (*UserSettings, error) {
	scheduleJSON, err := json.Marshal(s.WorkoutSchedule)
	if err != nil {
		scheduleJSON = []byte("{}")
	}

	if s.Allergens == nil {
		s.Allergens = []string{}
	}
	if s.ExcludedProducts == nil {
		s.ExcludedProducts = []string{}
	}

	var rawSchedule []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO user_settings (
			user_id, weight, target_weight, height, age, sex, focus, weekly_pace,
			workouts_per_week, workout_schedule, missed_workout_today,
			allergens, excluded_products, diet_type,
			weekly_budget, promo_priority, delivery_included, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, NOW()
		)
		ON CONFLICT (user_id) DO UPDATE SET
			weight               = EXCLUDED.weight,
			target_weight        = EXCLUDED.target_weight,
			height               = EXCLUDED.height,
			age                  = EXCLUDED.age,
			sex                  = EXCLUDED.sex,
			focus                = EXCLUDED.focus,
			weekly_pace          = EXCLUDED.weekly_pace,
			workouts_per_week    = EXCLUDED.workouts_per_week,
			workout_schedule     = EXCLUDED.workout_schedule,
			missed_workout_today = EXCLUDED.missed_workout_today,
			allergens            = EXCLUDED.allergens,
			excluded_products    = EXCLUDED.excluded_products,
			diet_type            = EXCLUDED.diet_type,
			weekly_budget        = EXCLUDED.weekly_budget,
			promo_priority       = EXCLUDED.promo_priority,
			delivery_included    = EXCLUDED.delivery_included,
			updated_at           = NOW()
		RETURNING
			user_id, weight, target_weight, height, age, sex, focus, weekly_pace,
			workouts_per_week, workout_schedule, missed_workout_today,
			allergens, excluded_products, diet_type,
			weekly_budget, promo_priority, delivery_included, updated_at
	`,
		s.UserID, s.Weight, s.TargetWeight, s.Height, s.Age, s.Sex, s.Focus, s.WeeklyPace,
		s.WorkoutsPerWeek, scheduleJSON, s.MissedWorkoutToday,
		s.Allergens, s.ExcludedProducts, s.DietType,
		s.WeeklyBudget, s.PromoPriority, s.DeliveryIncluded,
	).Scan(
		&s.UserID, &s.Weight, &s.TargetWeight, &s.Height, &s.Age, &s.Sex, &s.Focus, &s.WeeklyPace,
		&s.WorkoutsPerWeek, &rawSchedule, &s.MissedWorkoutToday,
		&s.Allergens, &s.ExcludedProducts, &s.DietType,
		&s.WeeklyBudget, &s.PromoPriority, &s.DeliveryIncluded, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user settings: %w", err)
	}

	_ = json.Unmarshal(rawSchedule, &s.WorkoutSchedule)
	return s, nil
}

func (r *SettingsRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	s := &UserSettings{UserID: userID}
	var rawSchedule []byte

	err := r.pool.QueryRow(ctx, `
		SELECT
			user_id, weight, target_weight, height, age, sex, focus, weekly_pace,
			workouts_per_week, workout_schedule, missed_workout_today,
			allergens, excluded_products, diet_type,
			weekly_budget, promo_priority, delivery_included, updated_at
		FROM user_settings WHERE user_id = $1
	`, userID).Scan(
		&s.UserID, &s.Weight, &s.TargetWeight, &s.Height, &s.Age, &s.Sex, &s.Focus, &s.WeeklyPace,
		&s.WorkoutsPerWeek, &rawSchedule, &s.MissedWorkoutToday,
		&s.Allergens, &s.ExcludedProducts, &s.DietType,
		&s.WeeklyBudget, &s.PromoPriority, &s.DeliveryIncluded, &s.UpdatedAt,
	)
	if err != nil {
		// Only a missing row is a normal state: a user who has not opened the
		// settings page yet gets the demo profile. Anything else — a dead pool,
		// a scan mismatch after a migration — is a real failure and must reach
		// the handler, which refuses to plan a week on somebody else's numbers.
		// Swallowing it used to pin every run to the fallback budget of 2000 UAH
		// no matter what the user had saved.
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load user settings %s: %w", userID, err)
		}
		log.Printf("[WARN] user %s has no settings row, planning on demo defaults", userID)
		return &UserSettings{
			UserID:             userID,
			Weight:             78.4,
			TargetWeight:       72.5,
			Height:             182.0,
			Age:                29,
			Sex:                "чол.",
			Focus:              "Схуднення",
			WeeklyPace:         -0.6,
			WorkoutsPerWeek:    4,
			WorkoutSchedule:    map[string]string{"ПН": "силові", "ВТ": "кардіо", "ЧТ": "силові", "СБ": "силові"},
			MissedWorkoutToday: false,
			Allergens:          []string{"лактоза", "горіхи"},
			ExcludedProducts:   []string{"гриби", "кінза", "печінка"},
			DietType:           "БЕЗ ОБМЕЖЕНЬ",
			WeeklyBudget:       2000.0,
			PromoPriority:      "Високий",
			DeliveryIncluded:   true,
			UpdatedAt:          time.Now(),
		}, nil
	}

	_ = json.Unmarshal(rawSchedule, &s.WorkoutSchedule)
	return s, nil
}
