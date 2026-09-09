package handler

import (
	"log"
	"math"
	"strings"

	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

// coreRequest mirrors the PlanRequest schema of the core agent (see API.md).
//
// The core is stateless: everything it knows about the user arrives in this one
// body. Every field here is read from user_settings — the browser cannot
// override a stored value any more, so a screen holding a stale copy of the
// profile can no longer plan a week against numbers the user never saved. The
// only per-run inputs left are the ones that exist nowhere in the DB: the free
// note, the fridge, the previous plan and the apply flag.
//
// missed_workout_today is absent on purpose: the core builds a whole week and
// has no idea what day it is for the user, so a flag about today would land on
// an arbitrary day of the next generation. API.md keeps that correction for a
// separate endpoint.
type coreRequest struct {
	SilpoAccessToken string            `json:"silpo_access_token"`
	Profile          coreProfile       `json:"profile"`
	Goal             string            `json:"goal"`
	WeeklyPaceKg     float64           `json:"weekly_pace_kg"`
	BudgetUAH        float64           `json:"budget_uah"`
	PromoPriority    string            `json:"promo_priority"`
	DeliveryIncluded bool              `json:"delivery_included"`
	WorkoutsPerWeek  int               `json:"workouts_per_week"`
	WorkoutSchedule  map[string]string `json:"workout_schedule"`
	DietType         string            `json:"diet_type"`
	Allergens        []string          `json:"allergens"`
	ExcludedProducts []string          `json:"excluded_products"`
	FridgeItems      []string          `json:"fridge_items"`
	Note             string            `json:"note"`
	PreviousPlan     *interface{}      `json:"previous_plan"`
	Apply            bool              `json:"apply"`
}

// coreProfile is PlanRequest.profile. Height, age and sex are nullable: the
// core would rather assume a value and say so in summary.notes than be handed
// an invented one.
type coreProfile struct {
	WeightKg       float64  `json:"weight_kg"`
	TargetWeightKg float64  `json:"target_weight_kg"`
	HeightCm       *float64 `json:"height_cm"`
	Age            *int     `json:"age"`
	Sex            *string  `json:"sex"`
}

const (
	// Used only when the stored budget is missing or non-positive; the core
	// requires a number and a zero budget would plan an empty week.
	defaultBudgetUAH = 1500.0
	// Same idea for weight: the core rejects weight_kg <= 0 with a 422.
	defaultWeightKg = 70.0
	// Both limits come from the core's validation (API.md, "Валідація").
	maxWorkoutsPerWeek = 7
	maxWeeklyPaceKg    = 2.0
)

// Settings hold the Ukrainian labels the UI shows. The core accepts those too,
// but API.md asks for the latin identifiers, so the mapping happens here, where
// the stored vocabulary is known.
var (
	goalAliases = map[string]string{
		"схуднення": "lose", "схуднути": "lose", "втрата ваги": "lose", "зниження ваги": "lose",
		"підтримання": "maintain", "підтримка": "maintain", "підтримання ваги": "maintain", "утримання ваги": "maintain",
		"набір": "gain", "набір ваги": "gain", "набір маси": "gain",
		"набір мʼязової маси": "gain", "набір м'язової маси": "gain",
	}

	dietAliases = map[string]string{
		"без обмежень": "none", "немає": "none", "звичайна": "none",
		"вегетаріанська": "vegetarian", "вегетаріанство": "vegetarian",
		"веганська": "vegan", "веганство": "vegan",
		"кето": "keto", "кетогенна": "keto",
		"палео": "paleo", "палеодієта": "paleo",
		"низький fodmap": "low_fodmap", "низькофодмап": "low_fodmap",
	}

	promoAliases = map[string]string{
		"низький": "low", "мінімальний": "low", "ігнорувати": "low",
		"середній": "medium", "помірний": "medium", "звичайний": "medium",
		"високий": "high", "максимальний": "high", "тільки акції": "high",
	}

	sexAliases = map[string]string{
		"чол.": "male", "чол": "male", "чоловіча": "male", "чоловік": "male", "ч": "male",
		"жін.": "female", "жін": "female", "жіноча": "female", "жінка": "female", "ж": "female",
	}

	weekdayAliases = map[string]string{
		"пн": "monday", "пон": "monday", "понеділок": "monday",
		"вт": "tuesday", "вів": "tuesday", "вівторок": "tuesday",
		"ср": "wednesday", "сер": "wednesday", "середа": "wednesday",
		"чт": "thursday", "чет": "thursday", "четвер": "thursday",
		"пт": "friday", "пятниця": "friday", "пʼятниця": "friday", "п'ятниця": "friday",
		"сб": "saturday", "суб": "saturday", "субота": "saturday",
		"нд": "sunday", "нед": "sunday", "неділя": "sunday",
	}
)

// normalize maps a stored label onto the identifier the core expects.
//
// A value with no mapping is passed through as it was stored, not dropped: the
// core knows aliases of its own, and if it does not recognise the value either
// it answers 422 — which is the behaviour we want. A restriction the user set
// and the plan silently ignored is worse than a visible error.
func normalize(aliases map[string]string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if id, ok := aliases[strings.ToLower(trimmed)]; ok {
		return id
	}
	return trimmed
}

// normalizeSchedule rewrites the day keys ("ПН") into the core's ("monday").
// An entry with a blank day is dropped: an empty key cannot name a weekday.
func normalizeSchedule(schedule map[string]string) map[string]string {
	normalized := make(map[string]string, len(schedule))
	for day, load := range schedule {
		if key := normalize(weekdayAliases, day); key != "" {
			normalized[key] = strings.TrimSpace(load)
		}
	}
	return normalized
}

// nonNilStrings keeps the JSON body honest: the core defaults a missing list to
// empty anyway, but `null` on the wire reads as "unknown", not "none".
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// buildCoreRequest turns the stored settings into one PlanRequest.
//
// note, fridgeItems, previousPlan and apply are the per-run arguments: they
// describe this generation rather than the user, so there is no row to read
// them from.
func buildCoreRequest(
	s *storage.UserSettings,
	silpoToken string,
	note string,
	fridgeItems []string,
	previousPlan *interface{},
	apply bool,
) coreRequest {
	weight := s.Weight
	if weight <= 0 {
		log.Printf("[WARN] user %s has no stored weight, falling back to %.0f kg", s.UserID, defaultWeightKg)
		weight = defaultWeightKg
	}

	targetWeight := s.TargetWeight
	if targetWeight <= 0 {
		targetWeight = weight
	}

	budget := s.WeeklyBudget
	if budget <= 0 {
		budget = defaultBudgetUAH
	}

	workouts := s.WorkoutsPerWeek
	if workouts < 0 {
		workouts = 0
	}
	if workouts > maxWorkoutsPerWeek {
		workouts = maxWorkoutsPerWeek
	}

	// The core ignores the sign — direction is goal's job — and rejects
	// anything past 2 kg/week as a units mistake. Clamping a corrupt stored
	// value costs the user a slightly different pace; a 422 costs them the
	// whole run.
	pace := math.Abs(s.WeeklyPace)
	if pace > maxWeeklyPaceKg {
		log.Printf("[WARN] weekly_pace %.2f for user %s is past %.1f kg/week, clamping",
			s.WeeklyPace, s.UserID, maxWeeklyPaceKg)
		pace = maxWeeklyPaceKg
	}

	profile := coreProfile{WeightKg: weight, TargetWeightKg: targetWeight}
	if s.Height > 0 {
		height := s.Height
		profile.HeightCm = &height
	}
	if s.Age > 0 {
		age := s.Age
		profile.Age = &age
	}
	if sex := normalize(sexAliases, s.Sex); sex != "" {
		profile.Sex = &sex
	}

	return coreRequest{
		SilpoAccessToken: silpoToken,
		Profile:          profile,
		Goal:             normalize(goalAliases, s.Focus),
		WeeklyPaceKg:     pace,
		BudgetUAH:        budget,
		PromoPriority:    normalize(promoAliases, s.PromoPriority),
		DeliveryIncluded: s.DeliveryIncluded,
		WorkoutsPerWeek:  workouts,
		WorkoutSchedule:  normalizeSchedule(s.WorkoutSchedule),
		DietType:         normalize(dietAliases, s.DietType),
		Allergens:        nonNilStrings(s.Allergens),
		ExcludedProducts: nonNilStrings(s.ExcludedProducts),
		FridgeItems:      nonNilStrings(fridgeItems),
		Note:             note,
		PreviousPlan:     previousPlan,
		Apply:            apply,
	}
}
