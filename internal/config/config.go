package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port             string
	JWTSecret        string
	DatabaseURL      string
	CoreAgentURL     string
	CoreServiceToken string
	SilpoRefreshURL  string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		CoreAgentURL:     getEnv("CORE_AGENT_URL", "http://localhost:9000"),
		CoreServiceToken: os.Getenv("CORE_SERVICE_TOKEN"),
		SilpoRefreshURL:  getEnv("SILPO_REFRESH_URL", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
