package handler

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type SettingsHandler struct {
	settings *storage.SettingsRepo
}

func NewSettingsHandler(settings *storage.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) GetAll(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	settings, err := h.settings.GetAll(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(settings)
}

func (h *SettingsHandler) Patch(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	category := storage.SettingsCategory(c.Params("category"))
	if !storage.IsValidSettingsCategory(category) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid category"})
	}

	payload := json.RawMessage(c.Body())
	if !json.Valid(payload) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid json payload"})
	}

	if err := h.settings.UpsertDraft(c.Context(), userID, category, payload); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *SettingsHandler) Apply(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	effectiveFrom := nextWeekStart(time.Now())
	if err := h.settings.Apply(c.Context(), userID, effectiveFrom); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h *SettingsHandler) Reset(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := h.settings.Reset(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func nextWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysUntilNextMonday := 8 - weekday
	next := t.AddDate(0, 0, daysUntilNextMonday)
	return time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
}
