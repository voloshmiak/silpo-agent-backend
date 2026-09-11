package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/voloshmiak/silpo-agent-backend/internal/auth"
	"github.com/voloshmiak/silpo-agent-backend/internal/email"
	"github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

type UserHandler struct {
	users     *storage.UserRepo
	settings  *storage.SettingsRepo
	tokens    *storage.TokenRepo
	progress  *storage.ProgressRepo
	mailer    *email.Mailer
	jwtSecret string
}

func NewUserHandler(
	users *storage.UserRepo,
	settings *storage.SettingsRepo,
	tokens *storage.TokenRepo,
	progress *storage.ProgressRepo,
	mailer *email.Mailer,
	jwtSecret string,
) *UserHandler {
	return &UserHandler{
		users:     users,
		settings:  settings,
		tokens:    tokens,
		progress:  progress,
		mailer:    mailer,
		jwtSecret: jwtSecret,
	}
}

// POST /users — register (onboarding: accepts name, email, silpo_token; generates password and emails it)
func (h *UserHandler) Register(c *fiber.Ctx) error {
	var body struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		SilpoToken   string `json:"silpo_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		body.Name = "Користувач"
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))

	// Check if email already registered
	if body.Email != "" {
		existing, _ := h.users.GetByEmail(c.Context(), body.Email)
		if existing != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Користувач із такою електронною поштою вже зареєстрований. Скористайтеся входом за паролем.",
			})
		}
	}

	// Generate a secure random password if email was provided
	var generatedPassword string
	var passwordHash string
	if body.Email != "" {
		generatedPassword = auth.GenerateRandomPassword(10)
		var hashErr error
		passwordHash, hashErr = auth.HashPassword(generatedPassword)
		if hashErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
		}
	}

	user, err := h.users.Create(c.Context(), body.Name, body.Email, passwordHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// If MCP token was provided during registration, save it immediately
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

	// Initialize default user settings in DB and record baseline weight
	if defSettings, err := h.settings.GetByUserID(c.Context(), user.ID); err == nil && defSettings != nil {
		if h.progress != nil && defSettings.Weight > 0 {
			_, _ = h.progress.RecordWeight(c.Context(), user.ID, defSettings.Weight, time.Now())
		}
	}

	token, err := auth.GenerateToken(user.ID, h.jwtSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
	}

	resp := fiber.Map{
		"user":  user,
		"token": token,
	}
	if generatedPassword != "" {
		resp["generated_password"] = generatedPassword
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

// POST /users/login — login with email & password (for frontend login tab)
func (h *UserHandler) Login(c *fiber.Ctx) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email та password обов'язкові для входу",
		})
	}

	user, err := h.users.GetByEmail(c.Context(), body.Email)
	if err != nil || user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Невірний email або пароль",
		})
	}

	if user.PasswordHash == "" || !auth.CheckPasswordHash(body.Password, user.PasswordHash) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Невірний email або пароль",
		})
	}

	token, err := auth.GenerateToken(user.ID, h.jwtSecret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
	}

	return c.JSON(fiber.Map{
		"user":  user,
		"token": token,
	})
}

// PUT /users/me/password — change user password in profile
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var body struct {
		OldPassword string `json:"old_password,omitempty"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}

	body.NewPassword = strings.TrimSpace(body.NewPassword)
	if len(body.NewPassword) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Новий пароль має бути не менше 6 символів",
		})
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}

	if err := h.users.UpdatePassword(c.Context(), userID, newHash); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "Пароль успішно оновлено"})
}

// GET /users/me — get basic profile (name, email, id, created_at)
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

	if h.progress != nil && s.Weight > 0 {
		_, _ = h.progress.RecordWeight(c.Context(), userID, s.Weight, time.Now())
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
