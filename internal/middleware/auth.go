package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/voloshmiak/silpo-agent-backend/internal/auth"
)

const UserIDKey = "userID"

// RequireAuth validates the Bearer JWT and injects the userID into locals.
func RequireAuth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing Authorization header"})
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid Authorization header format"})
		}

		userID, err := auth.ValidateToken(parts[1], jwtSecret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		c.Locals(UserIDKey, userID)
		return c.Next()
	}
}

// GetUserID extracts the authenticated user ID from Fiber locals.
func GetUserID(c *fiber.Ctx) uuid.UUID {
	return c.Locals(UserIDKey).(uuid.UUID)
}
