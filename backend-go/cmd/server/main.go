package main

import (
	"context"
	"log/slog"
	"os"

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

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
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
	logger.Info("listening", "address", cfg.Port)
	if err := a.ListenAndServe(":" + cfg.Port); err != nil {
		logger.Error("server stopped", "error", err)
	}
}
