package main

import (
	"context"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		logger.Error("POSTGRES_DSN is not set")
		os.Exit(1)
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	a := app.New(db, app.Config{Secret: env("SECRET_KEY", "default_secret_key"), Logger: logger})
	logger.Info("listening", "address", env("PORT", "8000"))
	if err := a.ListenAndServe(":" + env("PORT", "8000")); err != nil {
		logger.Error("server stopped", "error", err)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
