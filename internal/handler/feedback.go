package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type FeedbackHandler struct {
	feedbacks *storage.FeedbackRepo
}

func NewFeedbackHandler(feedbacks *storage.FeedbackRepo) *FeedbackHandler {
	return &FeedbackHandler{
		feedbacks: feedbacks,
	}
}

type submitFeedbackRequest struct {
	PlanID      *string              `json:"plan_id,omitempty"`
	DishRatings []storage.DishRating `json:"dish_ratings"`
	Tags        []string             `json:"tags"`
}

// POST /feedbacks — submit user feedback for the week (dish ratings & tags)
func (h *FeedbackHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req submitFeedbackRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}

	f := &storage.Feedback{
		UserID:      userID,
		DishRatings: req.DishRatings,
		Tags:        req.Tags,
	}

	if req.PlanID != nil && *req.PlanID != "" {
		if pid, err := uuid.Parse(*req.PlanID); err == nil {
			f.PlanID = &pid
		}
	}

	saved, err := h.feedbacks.Create(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save feedback: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(saved)
}

// GET /feedbacks/latest — get user's most recent feedback
func (h *FeedbackHandler) GetLatest(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	f, err := h.feedbacks.GetLatestByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no feedback found"})
	}

	return c.JSON(f)
}
