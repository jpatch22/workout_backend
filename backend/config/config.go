// Package config loads runtime configuration from the environment.
//
// All config comes from env vars so the same binary runs locally, on Render,
// and in CI without code changes. Missing required values fail fast at startup
// rather than at first request.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved application configuration.
type Config struct {
	Port           string
	Env            string // "development" | "production"
	DatabaseURL    string
	JWTSecret      []byte
	JWTTTL         time.Duration
	AllowedOrigins []string
	AutoMigrate    bool
}

// IsProduction reports whether the server is running in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads configuration from the environment, applying development-friendly
// defaults for local work. DATABASE_URL and JWT_SECRET have no defaults in
// production but fall back to local values otherwise.
func Load() (Config, error) {
	cfg := Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("APP_ENV", "development"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/workout_tracker?sslmode=disable"),
		JWTSecret:      []byte(getEnv("JWT_SECRET", "dev-insecure-secret-change-me")),
		AllowedOrigins: splitAndTrim(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_TTL: %w", err)
	}
	cfg.JWTTTL = ttl

	autoMigrate, err := strconv.ParseBool(getEnv("AUTO_MIGRATE", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("AUTO_MIGRATE: %w", err)
	}
	cfg.AutoMigrate = autoMigrate

	if cfg.IsProduction() {
		if len(cfg.JWTSecret) < 32 || string(cfg.JWTSecret) == "dev-insecure-secret-change-me" {
			return Config{}, fmt.Errorf("JWT_SECRET must be set to a strong value (>=32 chars) in production")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
