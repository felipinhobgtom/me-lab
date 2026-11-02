package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipinho/me-lab/internal/repository/sqlite"
	transport "github.com/felipinho/me-lab/internal/transport/http"
)

const defaultPort = "8080"

func main() {
	// --- Configuration from environment ---
	port := envOrDefault("PORT", defaultPort)
	databaseURL := envOrDefault("DATABASE_URL", "me-lab.db")
	jwtSecret := envOrDefault("JWT_SECRET", "dev-secret-change-me")
	logLevel := envOrDefault("LOG_LEVEL", "info")

	// --- Structured logger ---
	logger := newLogger(logLevel)
	slog.SetDefault(logger)

	logger.Info("starting ME Lab API",
		"port", port,
		"database", databaseURL,
		"log_level", logLevel,
	)

	// --- Database ---
	db, err := sqlite.NewDB(databaseURL)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("database connected", "url", databaseURL)

	// --- Router ---
	router := transport.NewRouter(transport.Deps{
		Logger:    logger,
		JWTSecret: jwtSecret,
	})

	// --- HTTP Server ---
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Graceful shutdown ---
	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}

	// Give outstanding requests 10 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

// envOrDefault reads an environment variable, returning def if not set.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// newLogger creates a structured slog.Logger at the requested level.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}
