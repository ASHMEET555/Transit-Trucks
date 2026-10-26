package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	pathFlag := flag.String("path", "migrations", "Path to migration files directory")
	flag.Parse()

	args := flag.Args()
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}

	absPath, err := filepath.Abs(*pathFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid migrations path: %v\n", err)
		os.Exit(1)
	}

	m, err := migrate.New("file://"+filepath.ToSlash(absPath), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not initialize migrations: %v\n", err)
		os.Exit(1)
	}
	defer m.Close()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintf(os.Stderr, "Migration up failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully (up).")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintf(os.Stderr, "Migration down failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations reverted successfully (down).")
	case "step":
		if err := m.Steps(1); err != nil && err != migrate.ErrNoChange {
			fmt.Fprintf(os.Stderr, "Migration step failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migration step executed successfully.")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s. Use 'up' or 'down'\n", cmd)
		os.Exit(1)
	}
}
