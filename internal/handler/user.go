package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/voloshmiak/silpo-agent-backend/internal/auth"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type UserHandler struct {
	users     *storage.UserRepo
	tokens    *storage.TokenRepo
	settings  *storage.SettingsRepo
	jwtSecret string
}

func NewUserHandler(
	users *storage.UserRepo,
	tokens *storage.TokenRepo,
	settings *storage.SettingsRepo,
	jwtSecret string,
) *UserHandler {
	return &UserHandler{
		users:     users,
		tokens:    tokens,
		settings:  settings,
		jwtSecret: jwtSecret,
	}
}

// Register handles POST /users — an open endpoint that accepts an optional silpo_token / access_token.
func (h *UserHandler) Register(c *fiber.Ctx) error {
	var body struct {
		Name         string `json:"name"`
		SilpoToken   string `json:"silpo_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	user, err := h.users.Create(c.Context(), body.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// If an MCP token was provided during registration, save it immediately.
	mcpToken := body.SilpoToken
	if mcpToken == "" {
		mcpToken = body.AccessToken
	}
	if mcpToken != "" {
		_ = h.tokens.Upsert(c.Context(), &storage.SilpoToken{
			UserID:       user.ID,
			AccessToken:  mcpToken,
			RefreshToken: body.RefreshToken,
		})
	}

	token, err := auth.GenerateToken(user.ID, h.jwtSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":  user,
		"token": token,
	})
}

// GetMe handles GET /users/me — returns the basic profile (name, id, created_at).
func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

// UpdateMe handles PUT /users/me — updates the user's name.
func (h *UserHandler) UpdateMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	user, err := h.users.Update(c.Context(), userID, body.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(user)
}

// SaveSilpoToken handles POST /users/me/silpo-token — saves the Silpo access and refresh tokens.
func (h *UserHandler) SaveSilpoToken(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if body.AccessToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "access_token is required"})
	}

	t := &storage.SilpoToken{
		UserID:       userID,
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
	}
	if err := h.tokens.Upsert(c.Context(), t); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
