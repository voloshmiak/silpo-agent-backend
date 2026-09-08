package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"github.com/voloshmiak/silpo-agent-backend/internal/config"
	"github.com/voloshmiak/silpo-agent-backend/internal/handler"
	mw "github.com/voloshmiak/silpo-agent-backend/internal/middleware"
	"github.com/voloshmiak/silpo-agent-backend/internal/silpo"
	"github.com/voloshmiak/silpo-agent-backend/internal/storage"
)

func main() {
	// Load .env (optional — already set env vars take precedence).
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Database.
	pool, err := storage.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	// Repositories.
	userRepo := storage.NewUserRepo(pool)
	tokenRepo := storage.NewTokenRepo(pool)
	planRepo := storage.NewPlanRepo(pool)
	settingsRepo := storage.NewSettingsRepo(pool)
	feedbackRepo := storage.NewFeedbackRepo(pool)

	// Services.
	silpoSvc := silpo.NewService(cfg.SilpoRefreshURL)

	// Handlers.
	userHandler := handler.NewUserHandler(userRepo, tokenRepo, cfg.JWTSecret)
	planHandler := handler.NewPlanHandler(planRepo)
	settingsHandler := handler.NewSettingsHandler(settingsRepo)
	feedbackHandler := handler.NewFeedbackHandler(feedbackRepo)
	streamHandler := handler.NewStreamHandler(planRepo, tokenRepo, userRepo, feedbackRepo, settingsRepo, silpoSvc, cfg.CoreAgentURL, cfg.CoreServiceToken)

	// Fiber app.
	app := fiber.New(fiber.Config{
		AppName:      "silpo-agent-backend",
		ReadTimeout:  0, // streaming — no timeout on reads
		WriteTimeout: 0,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Routes.
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// User routes.
	app.Post("/users", userHandler.Register)
	auth := app.Group("", mw.RequireAuth(cfg.JWTSecret))
	auth.Get("/users/me", userHandler.GetMe)
	auth.Put("/users/me", userHandler.UpdateMe)
	auth.Post("/users/me/silpo-token", userHandler.SaveSilpoToken)

	// Plan routes.
	auth.Get("/plans", planHandler.List)
	auth.Get("/plans/:id", planHandler.Get)

	// Settings routes.
	auth.Get("/settings", settingsHandler.GetAll)
	auth.Patch("/settings/:category", settingsHandler.Patch)
	auth.Post("/settings/apply", settingsHandler.Apply)
	auth.Post("/settings/reset", settingsHandler.Reset)

	// Feedback routes.
	auth.Post("/plans/:id/ratings", feedbackHandler.RateDish)
	auth.Get("/plans/:id/ratings", feedbackHandler.ListRatings)
	auth.Post("/plans/:id/tags", feedbackHandler.ToggleTag)
	auth.Get("/plans/:id/tags", feedbackHandler.ListTags)
	auth.Get("/plans/:id/adjustments", feedbackHandler.ListAdjustments)

	// Streaming proxy.
	auth.Get("/plan/stream", streamHandler.Stream)

	// Graceful shutdown.
	go func() {
		<-ctx.Done()
		log.Println("shutting down...")
		_ = app.Shutdown()
	}()

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
