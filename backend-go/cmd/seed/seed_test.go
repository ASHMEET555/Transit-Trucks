package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
)

func TestSeedProductionGuard(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append(os.Environ(), "ENV=production", "DATABASE_URL=postgresql://invalid:invalid@localhost:5432/invalid")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected cmd/seed with ENV=production to exit non-zero, but succeeded. Output: %s", string(out))
	}
	outStr := string(out)
	if !strings.Contains(outStr, "ENV=production") {
		t.Fatalf("expected output to mention ENV=production, got: %s", outStr)
	}
}

func TestSeedPasswordUpdateOnConflict(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer pool.Close()

	testEmail := "fleet@transitops.com"
	pass1 := "first-password-123"
	pass2 := "truck@555"

	// 1. Seed with pass1
	cmd1 := exec.Command("go", "run", "main.go")
	cmd1.Env = append(os.Environ(), "ENV=local", "DATABASE_URL="+dsn, "SEED_PASSWORD="+pass1, "SEED_RANDOM_SEED=42")
	out1, err1 := cmd1.CombinedOutput()
	if err1 != nil {
		t.Fatalf("first seed run failed: %v, output: %s", err1, string(out1))
	}

	var hash1 string
	err = pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE email = $1", testEmail).Scan(&hash1)
	if err != nil {
		t.Fatalf("failed to fetch hash after first seed: %v", err)
	}

	valid1, err := auth.VerifyPassword(pass1, hash1)
	if err != nil || !valid1 {
		t.Fatalf("pass1 verify failed on first seed: %v", err)
	}

	// 2. Seed again with pass2
	cmd2 := exec.Command("go", "run", "main.go")
	cmd2.Env = append(os.Environ(), "ENV=local", "DATABASE_URL="+dsn, "SEED_PASSWORD="+pass2, "SEED_RANDOM_SEED=42")
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		t.Fatalf("second seed run failed: %v, output: %s", err2, string(out2))
	}

	var hash2 string
	err = pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE email = $1", testEmail).Scan(&hash2)
	if err != nil {
		t.Fatalf("failed to fetch hash after second seed: %v", err)
	}

	// Assert pass2 succeeds and pass1 fails
	valid2, err := auth.VerifyPassword(pass2, hash2)
	if err != nil || !valid2 {
		t.Fatalf("pass2 verify failed on second seed: %v", err)
	}

	validOld, _ := auth.VerifyPassword(pass1, hash2)
	if validOld {
		t.Fatalf("pass1 should not verify against updated hash2")
	}
}
