package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	emailFlag := flag.String("email", "", "Admin user email address (required)")
	passwordFlag := flag.String("password", "", "Admin user password (optional; random generated if empty)")
	forceFlag := flag.Bool("force", false, "Force create admin even if active admin already exists")
	flag.Parse()

	if *emailFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --email is required")
		flag.Usage()
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL or POSTGRES_DSN must be set")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Check if active admin exists
	var activeAdminCount int
	err = db.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'Admin' AND is_active = true`).Scan(&activeAdminCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: database query failed: %v\n", err)
		os.Exit(1)
	}

	if activeAdminCount > 0 && !*forceFlag {
		fmt.Fprintf(os.Stderr, "Error: Active administrator already exists (%d found). Refusing to create another without --force\n", activeAdminCount)
		os.Exit(1)
	}

	pw := *passwordFlag
	generated := false
	if pw == "" {
		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: random generation failed: %v\n", err)
			os.Exit(1)
		}
		pw = hex.EncodeToString(randBytes)
		generated = true
	}

	hash, err := auth.HashPassword(pw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: password hashing failed: %v\n", err)
		os.Exit(1)
	}

	id := uuid.NewString()
	now := time.Now().UTC()
	email := strings.TrimSpace(*emailFlag)

	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Admin', true, $4, $4)
		 ON CONFLICT (email) DO UPDATE
		 SET password_hash = EXCLUDED.password_hash, role = 'Admin', is_active = true, updated_at = EXCLUDED.updated_at`,
		id, email, hash, now)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: inserting admin user failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Admin user %s created/updated successfully.\n", email)
	if generated {
		fmt.Println("-----------------------------------------------------------------")
		fmt.Printf("TEMPORARY INITIAL PASSWORD (printed once, store securely):\n%s\n", pw)
		fmt.Println("-----------------------------------------------------------------")
	}
}
