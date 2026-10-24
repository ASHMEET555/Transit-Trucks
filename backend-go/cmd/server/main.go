package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/app"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/config"
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
		Secret: cfg.SessionSecret,
		Logger: logger,
	})
	logger.Info("listening", "address", cfg.Port)
	if err := a.ListenAndServe(":" + cfg.Port); err != nil {
		logger.Error("server stopped", "error", err)
	}
}
