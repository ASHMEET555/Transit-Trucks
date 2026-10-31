package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPollSkipsNonDispatched proves the simulator never moves trucks whose
// trips are not Dispatched: Draft and Completed trips are not polled.
func TestPollSkipsNonDispatched(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("no test database: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	reg := "VH-NOND-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'M', 'Van', 1000, 0, 1000, 'Available', true)`, reg); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM vehicles WHERE registration_number = $1`, reg)
	lic := "DL-NOND-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'N', 'Heavy', '2029-01-01', '1', 90, 'Available', true)`, lic); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM drivers WHERE license_number = $1`, lic)

	for _, status := range []string{"Draft", "Completed", "Cancelled"} {
		if _, err := pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			SELECT 'Chicago Logistics Hub', 'Denver Intermodal Hub', $1, $2, 100, 100, 100, $3, now(), now()
			WHERE EXISTS (SELECT 1 FROM destinations WHERE name = 'Chicago Logistics Hub')`, reg, lic, status); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(context.Background(), `DELETE FROM trips WHERE vehicle_reg = $1`, reg)

	trucks, err := pollDispatched(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range trucks {
		if tr.vehicleReg == reg {
			t.Fatalf("non-Dispatched truck %s must never be polled", reg)
		}
	}
}
