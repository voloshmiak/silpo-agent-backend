package handler

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type ProgressHandler struct {
	progress *storage.ProgressRepo
	settings *storage.SettingsRepo
}

func NewProgressHandler(progress *storage.ProgressRepo, settings *storage.SettingsRepo) *ProgressHandler {
	return &ProgressHandler{
		progress: progress,
		settings: settings,
	}
}

// GET /progress — summary for "АРХІВ ТА ПРОГРЕС" screen (weight dynamics + food expenses)
func (h *ProgressHandler) GetSummary(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Determine period limit in weeks
	weeksParam := strings.ToLower(strings.TrimSpace(c.Query("weeks", "12")))
	limitWeeks := 12
	if weeksParam == "4" {
		limitWeeks = 4
	} else if weeksParam == "all" || weeksParam == "все" || weeksParam == "0" {
		limitWeeks = 0
	} else if w, err := strconv.Atoi(weeksParam); err == nil && w > 0 {
		limitWeeks = w
	}

	userSettings, _ := h.settings.GetByUserID(c.Context(), userID)
	if userSettings == nil {
		userSettings = &storage.UserSettings{
			UserID:       userID,
			Weight:       78.4,
			TargetWeight: 75.0,
			WeeklyBudget: 2000.0,
			WeeklyPace:   -0.5,
		}
	}

	// 1. Process Weight History & Forecast
	weights, err := h.progress.ListWeights(c.Context(), userID, limitWeeks)
	if err != nil {
		weights = []storage.WeightRecord{}
	}

	startWeight := userSettings.Weight
	currentWeight := userSettings.Weight

	if len(weights) > 0 {
		if earliest, err := h.progress.GetEarliestWeight(c.Context(), userID); err == nil && earliest != nil {
			startWeight = earliest.Weight
		} else {
			startWeight = weights[0].Weight
		}
		currentWeight = weights[len(weights)-1].Weight
	}

	weightChange := math.Round((currentWeight-startWeight)*10) / 10

	historyPoints := make([]fiber.Map, 0, len(weights))
	for i, w := range weights {
		historyPoints = append(historyPoints, fiber.Map{
			"date":        w.RecordedAt.Format("2006-01-02"),
			"week_label":  fmt.Sprintf("T%d", i+1),
			"weight":      w.Weight,
			"is_forecast": false,
		})
	}

	// If no records in DB yet, give a baseline point from current settings
	if len(historyPoints) == 0 {
		historyPoints = append(historyPoints, fiber.Map{
			"date":        time.Now().Format("2006-01-02"),
			"week_label":  "T1",
			"weight":      currentWeight,
			"is_forecast": false,
		})
	}

	// Generate forecast points toward target weight
	forecastPoints := make([]fiber.Map, 0)
	targetWeight := userSettings.TargetWeight
	if targetWeight <= 0 {
		targetWeight = currentWeight
	}

	pace := userSettings.WeeklyPace
	if pace == 0 {
		if targetWeight < currentWeight {
			pace = -0.5
		} else if targetWeight > currentWeight {
			pace = 0.5
		}
	} else {
		// Ensure sign of pace aligns with direction to target weight
		if targetWeight < currentWeight && pace > 0 {
			pace = -pace
		} else if targetWeight > currentWeight && pace < 0 {
			pace = -pace
		}
	}

	currForecast := currentWeight
	lastWeekNum := len(historyPoints)
	if (pace < 0 && currForecast > targetWeight) || (pace > 0 && currForecast < targetWeight) {
		for step := 1; step <= 8; step++ {
			lastWeekNum++
			currForecast += pace
			if (pace < 0 && currForecast < targetWeight) || (pace > 0 && currForecast > targetWeight) {
				currForecast = targetWeight
			}
			forecastPoints = append(forecastPoints, fiber.Map{
				"date":        time.Now().AddDate(0, 0, step*7).Format("2006-01-02"),
				"week_label":  fmt.Sprintf("T%d", lastWeekNum),
				"weight":      math.Round(currForecast*10) / 10,
				"is_forecast": true,
			})
			if currForecast == targetWeight {
				break
			}
		}
	}

	// 2. Process Expenses History
	expenses, err := h.progress.ListExpenses(c.Context(), userID, limitWeeks)
	if err != nil {
		expenses = []storage.ExpenseRecord{}
	}

	totalWeeks := len(expenses)
	var sumSpend float64
	var weeksWithinLimit int
	var overspentCount int
	overspentLabels := make([]string, 0)
	expenseItems := make([]fiber.Map, 0, len(expenses))

	for _, exp := range expenses {
		sumSpend += exp.TotalCost
		if exp.IsOverspent {
			overspentCount++
			overspentLabels = append(overspentLabels, exp.WeekLabel)
		} else {
			weeksWithinLimit++
		}

		expenseItems = append(expenseItems, fiber.Map{
			"id":           exp.ID,
			"plan_id":      exp.PlanID,
			"week_number":  exp.WeekNumber,
			"week_label":   exp.WeekLabel,
			"total_cost":   exp.TotalCost,
			"budget_limit": exp.BudgetLimit,
			"is_overspent": exp.IsOverspent,
			"date":         exp.RecordedAt.Format("2006-01-02"),
		})
	}

	var averageSpend float64
	if totalWeeks > 0 {
		averageSpend = math.Round(sumSpend / float64(totalWeeks))
	}

	return c.JSON(fiber.Map{
		"weight": fiber.Map{
			"current_weight": currentWeight,
			"start_weight":   startWeight,
			"target_weight":  targetWeight,
			"change_kg":      weightChange,
			"period_weeks":   limitWeeks,
			"history":        historyPoints,
			"forecast":       forecastPoints,
		},
		"expenses": fiber.Map{
			"weekly_limit":       userSettings.WeeklyBudget,
			"average_spend":      averageSpend,
			"weeks_within_limit": weeksWithinLimit,
			"total_weeks":        totalWeeks,
			"overspent_count":    overspentCount,
			"overspent_labels":   overspentLabels,
			"items":              expenseItems,
		},
	})
}

// POST /progress/weight — log user weight
func (h *ProgressHandler) RecordWeight(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		Weight float64 `json:"weight"`
		Date   string  `json:"date,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if req.Weight <= 0 || req.Weight > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "weight must be between 1 and 500 kg"})
	}

	recDate := time.Now()
	if req.Date != "" {
		if parsed, err := time.Parse("2006-01-02", req.Date); err == nil {
			recDate = parsed
		}
	}

	rec, err := h.progress.RecordWeight(c.Context(), userID, req.Weight, recDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Keep user_settings in sync
	if s, err := h.settings.GetByUserID(c.Context(), userID); err == nil && s != nil {
		s.Weight = req.Weight
		_, _ = h.settings.Upsert(c.Context(), s)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"record": fiber.Map{
			"id":          rec.ID,
			"weight":      rec.Weight,
			"recorded_at": rec.RecordedAt.Format("2006-01-02"),
		},
	})
}

// POST /progress/expenses — manually add or adjust weekly expense
func (h *ProgressHandler) RecordExpense(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		PlanID      *string  `json:"plan_id,omitempty"`
		WeekNumber  *int     `json:"week_number,omitempty"`
		WeekLabel   *string  `json:"week_label,omitempty"`
		TotalCost   float64  `json:"total_cost"`
		BudgetLimit *float64 `json:"budget_limit,omitempty"`
		Date        string   `json:"date,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}
	if req.TotalCost < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "total_cost cannot be negative"})
	}

	recDate := time.Now()
	if req.Date != "" {
		if parsed, err := time.Parse("2006-01-02", req.Date); err == nil {
			recDate = parsed
		}
	}

	userSettings, _ := h.settings.GetByUserID(c.Context(), userID)
	budgetLimit := 2000.0
	if userSettings != nil && userSettings.WeeklyBudget > 0 {
		budgetLimit = userSettings.WeeklyBudget
	}
	if req.BudgetLimit != nil && *req.BudgetLimit > 0 {
		budgetLimit = *req.BudgetLimit
	}

	weekNumber := 1
	if req.WeekNumber != nil && *req.WeekNumber > 0 {
		weekNumber = *req.WeekNumber
	} else {
		count, _ := h.progress.CountExpenses(c.Context(), userID)
		weekNumber = count + 1
	}

	weekLabel := fmt.Sprintf("Т%d", weekNumber)
	if req.WeekLabel != nil && *req.WeekLabel != "" {
		weekLabel = *req.WeekLabel
	}

	var planUUID *uuid.UUID
	if req.PlanID != nil && *req.PlanID != "" {
		if pid, err := uuid.Parse(*req.PlanID); err == nil {
			planUUID = &pid
		}
	}

	rec, err := h.progress.RecordExpense(c.Context(), &storage.ExpenseRecord{
		UserID:      userID,
		PlanID:      planUUID,
		WeekNumber:  weekNumber,
		WeekLabel:   weekLabel,
		TotalCost:   req.TotalCost,
		BudgetLimit: budgetLimit,
		RecordedAt:  recDate,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  "ok",
		"expense": rec,
	})
}

// GET /progress/export/csv — download CSV report for "↓ ЕКСПОРТ CSV" button
func (h *ProgressHandler) ExportCSV(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	weights, _ := h.progress.ListWeights(c.Context(), userID, 0)
	expenses, _ := h.progress.ListExpenses(c.Context(), userID, 0)

	var buf bytes.Buffer
	// UTF-8 BOM for Microsoft Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"Тиждень", "Дата", "Вага (кг)", "Витрати (грн)", "Ліміт (грн)", "Статус витрат"})

	// Map weights by date string
	weightByDate := make(map[string]float64)
	for _, w := range weights {
		weightByDate[w.RecordedAt.Format("2006-01-02")] = w.Weight
	}

	if len(expenses) > 0 {
		for _, exp := range expenses {
			dateStr := exp.RecordedAt.Format("2006-01-02")
			weightStr := "-"
			if w, ok := weightByDate[dateStr]; ok {
				weightStr = fmt.Sprintf("%.1f", w)
			}
			statusStr := "В межах ліміту"
			if exp.IsOverspent {
				statusStr = "Перевитрата"
			}
			_ = writer.Write([]string{
				exp.WeekLabel,
				dateStr,
				weightStr,
				fmt.Sprintf("%.2f", exp.TotalCost),
				fmt.Sprintf("%.2f", exp.BudgetLimit),
				statusStr,
			})
		}
	} else {
		// If only weights exist
		for i, w := range weights {
			_ = writer.Write([]string{
				fmt.Sprintf("Т%d", i+1),
				w.RecordedAt.Format("2006-01-02"),
				fmt.Sprintf("%.1f", w.Weight),
				"-",
				"-",
				"-",
			})
		}
	}

	writer.Flush()

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"silpo_progress.csv\"")
	return c.Send(buf.Bytes())
}
