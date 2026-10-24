package app

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Mirrors the Python-suite prerequisite: an isolated PostgreSQL database.
// Set TEST_POSTGRES_DSN after applying migrations and seeding the minimal fixture.
func TestHealth(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is required for PostgreSQL integration tests")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	New(db, Config{Secret: "test-secret"}).mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("health status = %d", w.Code)
	}
}
