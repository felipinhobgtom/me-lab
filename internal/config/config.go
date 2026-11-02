package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds validated runtime configuration loaded from environment variables.
type Config struct {
	Host               string
	Port               string
	DatabaseURL        string
	DBBusyTimeoutMS    int
	JWTSecret          string
	JWTExpiry          time.Duration
	LogLevel           string
	CORSAllowedOrigins []string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Host:            envOrDefault("HOST", "0.0.0.0"),
		Port:            envOrDefault("PORT", "8080"),
		DatabaseURL:     envOrDefault("DATABASE_URL", "me-lab.db"),
		DBBusyTimeoutMS: envIntOrDefault("DB_BUSY_TIMEOUT_MS", 5000),
		JWTSecret:       envOrDefault("JWT_SECRET", "dev-secret-change-me-in-production-32b"),
		JWTExpiry:       time.Duration(envIntOrDefault("JWT_EXPIRY_HOURS", 24)) * time.Hour,
		LogLevel:        strings.ToLower(envOrDefault("LOG_LEVEL", "info")),
	}

	rawOrigins := envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:*,http://127.0.0.1:*")
	for _, o := range strings.Split(rawOrigins, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, trimmed)
		}
	}

	if p, err := strconv.Atoi(cfg.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q: must be integer 1..65535", cfg.Port)
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	if len(cfg.JWTSecret) < 16 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}
	if cfg.JWTExpiry < time.Minute {
		return Config{}, fmt.Errorf("JWT_EXPIRY_HOURS must be >= 1")
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
