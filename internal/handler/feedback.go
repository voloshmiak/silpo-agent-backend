package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type FeedbackHandler struct {
	feedback *storage.FeedbackRepo
}

func NewFeedbackHandler(feedback *storage.FeedbackRepo) *FeedbackHandler {
	return &FeedbackHandler{feedback: feedback}
}

type rateDishRequest struct {
	DishName string `json:"dish_name"`
	Rating   int16  `json:"rating"`
}

func (h *FeedbackHandler) RateDish(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan id"})
	}

	var req rateDishRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.DishName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "dish_name is required"})
	}
	if req.Rating != -1 && req.Rating != 0 && req.Rating != 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "rating must be -1, 0 or 1"})
	}
	if err := h.feedback.UpsertDishRating(c.Context(), planID, userID, req.DishName, req.Rating); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *FeedbackHandler) ListRatings(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan_id"})
	}

	ratings, err := h.feedback.ListDishRatings(c.Context(), planID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ratings)
}

type toggleTagRequest struct {
	Tag string `json:"tag"`
}

func (h *FeedbackHandler) ToggleTag(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan_id"})
	}

	var req toggleTagRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.Tag == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tag is required"})
	}

	active, err := h.feedback.ToggleTag(c.Context(), planID, userID, req.Tag)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"tag": req.Tag, "active": active})
}

func (h *FeedbackHandler) ListTags(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan_id"})
	}

	tags, err := h.feedback.ListTags(c.Context(), planID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tags)
}

func (h *FeedbackHandler) generateAdjustments(ctx context.Context, userID, planID uuid.UUID) ([]storage.PlanAdjustment, error) {
	existing, err := h.feedback.ListAdjustments(ctx, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("check existing adjustments: %w", err)
	}
	if len(existing) > 0 {
		return existing, nil
	}

	ratings, err := h.feedback.ListDishRatings(ctx, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("load ratings: %w", err)
	}
	tags, err := h.feedback.ListTags(ctx, userID, planID)
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	tagSet := make(map[string]bool, len(tags))
	for _, tag := range tags {
		tagSet[tag.Tag] = true
	}
	// Rule 1
	for _, r := range ratings {
		if r.Rating != -1 {
			continue
		}
		payload, _ := json.Marshal(fiber.Map{
			"dish_name": r.DishName,
			"reason":    "Низька оцінка користувача",
		})
		if err := h.feedback.CreatedAdjustment(ctx, userID, planID, storage.AdjustmentExcludedDish, payload); err != nil {
			return nil, fmt.Errorf("create excluded_dish adjustment: %w", err)
		}
	}
	// Rule 2
	if tagSet["Занадто складно готувати"] {
		payload, _ := json.Marshal(fiber.Map{
			"reason": "тег «Занадто складно готувати» ",
			"action": "зменшити середню складність та час приготування страв",
		})
		if err := h.feedback.CreatedAdjustment(ctx, userID, planID, storage.AdjustmentSimplified, payload); err != nil {
			return nil, fmt.Errorf("create simplified adjustment: %w", err)
		}
	}
	// Rule 3
	if tagSet["Набридла курка"] {
		payload, _ := json.Marshal(fiber.Map{
			"reason": "тег «Набридла курка» ",
			"action": "змінити основний білок на альтернативу",
		})
		if err := h.feedback.CreatedAdjustment(ctx, userID, planID, storage.AdjustmentSimplified, payload); err != nil {
			return nil, fmt.Errorf("create simplified adjustment: %w", err)
		}
	}
	// Rule 4
	payload, _ := json.Marshal(fiber.Map{
		"note": "план буде перевірено на відповідність калорійній цілі",
	})
	if err := h.feedback.CreatedAdjustment(ctx, userID, planID, storage.AdjustmentCalorieCheck, payload); err != nil {
		return nil, fmt.Errorf("create calorie_check adjustment: %w", err)
	}

	return h.feedback.ListAdjustments(ctx, userID, planID)
}

func (h *FeedbackHandler) ListAdjustments(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan_id"})
	}

	adjustments, err := h.feedback.ListAdjustments(c.Context(), planID, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(adjustments)
}
