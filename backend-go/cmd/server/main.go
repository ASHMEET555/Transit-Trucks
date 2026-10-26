package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Pool configuration per Phase 5 spec
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid database url", "error", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 10
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.ConnConfig.ConnectTimeout = 3 * time.Second
	if poolCfg.ConnConfig.RuntimeParams == nil {
		poolCfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	poolCfg.ConnConfig.RuntimeParams["statement_timeout"] = "5000" // 5s timeout

	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	a := app.New(db, app.Config{
		Secret:         cfg.SessionSecret,
		Logger:         logger,
		AllowedOrigins: cfg.AllowedOrigins,
		Env:            cfg.Env,
		CookieDomain:   cfg.CookieDomain,
		DatabaseURL:    cfg.DatabaseURL,
		MetricsToken:   cfg.MetricsToken,
		JobSecret:      cfg.JobSecret,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)
	defer a.Close()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: a,
	}

	go func() {
		logger.Info("listening", "address", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("server shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful server shutdown failed", "error", err)
	}
	logger.Info("server shutdown complete")
}
