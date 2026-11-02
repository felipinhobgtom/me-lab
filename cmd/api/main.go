package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/config"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/felipinho/me-lab/internal/simulation"
	transport "github.com/felipinho/me-lab/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid startup configuration", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting ME Lab API",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.DatabaseURL,
		"log_level", cfg.LogLevel,
	)

	// Open SQLite connection pool (WAL + foreign_keys=ON + busy_timeout)
	db, err := sqlite.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open SQLite database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run embedded Goose SQL migrations
	if err := sqlite.Migrate(db); err != nil {
		logger.Error("failed to apply SQLite migrations", "error", err)
		os.Exit(1)
	}

	// Load embedded AE2 compatibility profiles & block catalog and seed DB
	reg, err := catalog.NewRegistry()
	if err != nil {
		logger.Error("failed to load AE2 catalog registry", "error", err)
		os.Exit(1)
	}
	if err := sqlite.SeedCatalog(context.Background(), db, reg); err != nil {
		logger.Error("failed to seed AE2 catalog into SQLite", "error", err)
		os.Exit(1)
	}

	store := sqlite.NewStore(db)
	engine := simulation.NewEngine()

	router := transport.NewRouter(transport.Deps{
		Logger:             logger,
		Store:              store,
		Catalog:            reg,
		Engine:             engine,
		JWTSecret:          cfg.JWTSecret,
		JWTExpiry:          cfg.JWTExpiry,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	})

	srv := &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown error", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped gracefully")
}

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
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
