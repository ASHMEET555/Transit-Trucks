package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	envVal := strings.ToLower(os.Getenv("ENV"))
	if envVal == "production" {
		fmt.Fprintln(os.Stderr, "Error: seeding is strictly refused when ENV=production (zero rows written)")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}

	p := os.Getenv("JSON_PATH")
	if p == "" {
		p = "dummy_data.json"
	}
	f, err := os.Open(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not locate dummy JSON file at %s: %v\n", p, err)
		os.Exit(1)
	}
	defer f.Close()

	var d map[string][]map[string]any
	if err := json.NewDecoder(f).Decode(&d); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid seed JSON: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	tx, err := db.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: transaction begin failed: %v\n", err)
		os.Exit(1)
	}
	defer tx.Rollback(ctx)

	// Seed Users with secure Argon2id hashes
	seedPassEnv := strings.TrimSpace(os.Getenv("SEED_PASSWORD"))
	if seedPassEnv == "" {
		seedPassEnv = strings.TrimSpace(os.Getenv("INITIAL_ADMIN_PASSWORD"))
	}

	generatedPasswords := make(map[string]string)

	for _, u := range d["users"] {
		email, _ := u["email"].(string)
		role, _ := u["role"].(string)
		if email == "" || role == "" {
			continue
		}

		userPass := seedPassEnv
		if userPass == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			userPass = hex.EncodeToString(b)
			generatedPasswords[email] = userPass
		}

		hash, err := auth.HashPassword(userPass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: password hash error: %v\n", err)
			os.Exit(1)
		}

		id := uuid.NewString()
		now := time.Now().UTC()

		_, err = tx.Exec(ctx,
			`INSERT INTO users(id, email, password_hash, role, is_active, created_at, updated_at)
			 VALUES($1, $2, $3, $4, true, $5, $5)
			 ON CONFLICT (email) DO NOTHING`,
			id, email, hash, role, now)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: inserting user failed: %v\n", err)
			os.Exit(1)
		}
	}

	run := func(q string, fields []string, items []map[string]any) {
		for _, x := range items {
			a := make([]any, len(fields))
			for i, k := range fields {
				a[i] = x[k]
			}
			if _, e := tx.Exec(ctx, q, a...); e != nil {
				fmt.Fprintf(os.Stderr, "Error inserting row: %v\n", e)
				os.Exit(1)
			}
		}
	}

	run("INSERT INTO destinations(name,region) VALUES($1,$2) ON CONFLICT DO NOTHING", []string{"name", "region"}, d["destinations"])
	run("INSERT INTO vehicles(registration_number,model,type,max_load_capacity,odometer,acquisition_cost,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"registration_number", "model", "type", "max_load_capacity", "odometer", "acquisition_cost", "status"}, d["vehicles"])
	run("INSERT INTO drivers(license_number,name,license_category,license_expiry_date,contact_number,safety_score,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"license_number", "name", "license_category", "license_expiry_date", "contact_number", "safety_score", "status"}, d["drivers"])
	run("INSERT INTO trips(id,source_name,destination_name,vehicle_reg,driver_license,cargo_weight,planned_distance,revenue,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING", []string{"id", "source_name", "destination_name", "vehicle_reg", "driver_license", "cargo_weight", "planned_distance", "revenue", "status"}, d["trips"])
	run("INSERT INTO maintenance_logs(id,vehicle_reg,description,cost,start_date,end_date,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"id", "vehicle_reg", "description", "cost", "start_date", "end_date", "status"}, d["maintenance_logs"])
	run("INSERT INTO expenses(id,trip_id,type,liters,cost,date) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", []string{"id", "trip_id", "type", "liters", "cost", "date"}, d["fuel_expenses"])

	for _, s := range []string{"trips", "maintenance_logs", "expenses"} {
		if _, e := tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence($1, 'id'), COALESCE((SELECT MAX(id) FROM "+s+"), 1), true)", s); e != nil {
			fmt.Fprintf(os.Stderr, "Error resetting sequence: %v\n", e)
			os.Exit(1)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error committing seed transaction: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Database seeding completed cleanly.")
	if len(generatedPasswords) > 0 {
		fmt.Println("-----------------------------------------------------------------")
		fmt.Println("GENERATED SEED PASSWORDS (printed once, store securely):")
		for email, pw := range generatedPasswords {
			fmt.Printf("%s: %s\n", email, pw)
		}
		fmt.Println("-----------------------------------------------------------------")
	} else if seedPassEnv != "" {
		fmt.Println("Seed passwords set from environment variable.")
	}
}
