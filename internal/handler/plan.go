package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type PlanHandler struct {
	plans *storage.PlanRepo
}

func NewPlanHandler(plans *storage.PlanRepo) *PlanHandler {
	return &PlanHandler{plans: plans}
}

// GET /plans — list user's plans (paginated)
func (h *PlanHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	plans, err := h.plans.ListByUserID(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(plans)
}

// GET /plans/:id — get single plan
func (h *PlanHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	planID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid plan id"})
	}

	plan, err := h.plans.GetByID(c.Context(), planID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "plan not found"})
	}
	return c.JSON(plan)
}
