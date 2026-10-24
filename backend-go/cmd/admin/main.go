package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Support optional subcommand "create-admin" as first arg
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "create-admin" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("admin", flag.ExitOnError)
	emailFlag := fs.String("email", "", "Admin user email address")
	passwordFlag := fs.String("password", "", "Admin user password")
	forceFlag := fs.Bool("force", false, "Force create admin even if active admin already exists")
	_ = fs.Parse(args)

	email := strings.TrimSpace(*emailFlag)
	if email == "" {
		email = strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	}
	if email == "" {
		// Attempt prompt from stdin
		fmt.Print("Enter admin email: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			email = strings.TrimSpace(scanner.Text())
		}
	}
	if email == "" {
		fmt.Fprintln(os.Stderr, "Error: admin email is required (via --email, ADMIN_EMAIL, or interactive prompt)")
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

	pw := strings.TrimSpace(*passwordFlag)
	if pw == "" {
		pw = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	}
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
