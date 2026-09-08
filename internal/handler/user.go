package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/voloshmiak/silpo-agent-backend/internal/auth"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type UserHandler struct {
	users     *storage.UserRepo
	settings  *storage.SettingsRepo
	tokens    *storage.TokenRepo
	jwtSecret string
}

func NewUserHandler(
	users *storage.UserRepo,
	settings *storage.SettingsRepo,
	tokens *storage.TokenRepo,
	jwtSecret string,
) *UserHandler {
	return &UserHandler{
		users:     users,
		settings:  settings,
		tokens:    tokens,
		jwtSecret: jwtSecret,
	}
}

// POST /users — register (open endpoint)
func (h *UserHandler) Register(c *fiber.Ctx) error {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	user, err := h.users.Create(c.Context(), body.Name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

// GET /users/me — get basic profile (name, id, created_at)
func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(user)
}

// PUT /users/me — update user name
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

// GET /users/me/settings — get all user parameters, physical metrics & restrictions
func (h *UserHandler) GetSettings(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	s, err := h.settings.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(s)
}

// PUT /users/me/settings — save/update user parameters, physical metrics & restrictions
func (h *UserHandler) UpdateSettings(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var s storage.UserSettings
	if err := c.BodyParser(&s); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body: " + err.Error()})
	}
	s.UserID = userID

	updated, err := h.settings.Upsert(c.Context(), &s)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(updated)
}

// POST /users/me/silpo-token — save Silpo access + refresh token
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
