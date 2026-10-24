package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to DB: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	cfg := app.Config{
		Secret:         "bench-secret-key-at-least-32-bytes-long",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "localhost",
	}
	a := app.New(pool, cfg)

	// Ensure an admin user exists for bench session
	adminEmail := "bench-admin@transitops.com"
	pwHash, _ := auth.HashPassword("BenchAdminPass123!")
	_, _ = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000001', $1, $2, 'Admin', true, now(), now())
		ON CONFLICT (email) DO UPDATE SET is_active = true`, adminEmail, pwHash)

	token, _, err := auth.CreateSession(ctx, pool, adminEmail, "127.0.0.1", "bench-agent")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
		os.Exit(1)
	}

	endpoints := []string{
		"/api/v1/fleet/vehicles",
		"/api/v1/fleet/drivers",
		"/api/v1/destinations/",
		"/api/v1/trips/",
	}

	iterations := 100
	results := make(map[string][]time.Duration)

	// Warm up
	for _, ep := range endpoints {
		req := httptest.NewRequest("GET", ep, nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		a.ServeHTTP(w, req)
	}

	// Benchmark runs
	for _, ep := range endpoints {
		durations := make([]time.Duration, 0, iterations)
		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest("GET", ep, nil)
			req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
			w := httptest.NewRecorder()

			start := time.Now()
			a.ServeHTTP(w, req)
			durations = append(durations, time.Since(start))
		}
		sort.Slice(durations, func(i, j int) bool {
			return durations[i] < durations[j]
		})
		results[ep] = durations
	}

	out := "# Phase 2 Baseline Latency Benchmark\n\n"
	out += fmt.Sprintf("Measured %d iterations per endpoint against PostgreSQL seed.\n\n", iterations)
	out += "| Endpoint | p50 (ms) | p95 (ms) | p99 (ms) | Min (ms) | Max (ms) |\n"
	out += "|---|---|---|---|---|---|\n"

	for _, ep := range endpoints {
		d := results[ep]
		p50 := float64(d[int(float64(iterations)*0.50)].Microseconds()) / 1000.0
		p95 := float64(d[int(float64(iterations)*0.95)].Microseconds()) / 1000.0
		p99 := float64(d[int(float64(iterations)*0.99)].Microseconds()) / 1000.0
		min := float64(d[0].Microseconds()) / 1000.0
		max := float64(d[len(d)-1].Microseconds()) / 1000.0

		out += fmt.Sprintf("| `%s` | %.2f | %.2f | %.2f | %.2f | %.2f |\n", ep, p50, p95, p99, min, max)
	}

	fmt.Print(out)

	// Write to docs/phase-2-baseline.md
	docPath := filepath.Join("docs", "phase-2-baseline.md")
	if err := os.WriteFile(docPath, []byte(out), 0644); err != nil {
		// Try relative to repo
		_ = os.WriteFile(filepath.Join("backend-go", "docs", "phase-2-baseline.md"), []byte(out), 0644)
	}
}
