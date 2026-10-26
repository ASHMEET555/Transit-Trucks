package realtime_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
	"github.com/jackc/pgx/v5/pgxpool"
)

func getDB(t *testing.T) *pgxpool.Pool {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping db test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping db ping failed: %v", err)
	}
	return pool
}

func TestTickets_LifecycleAndSingleUse(t *testing.T) {
	pool := getDB(t)
	defer pool.Close()
	ctx := context.Background()

	rawTicket, ttl, err := realtime.IssueTicket(ctx, pool, "user-test-1", rbac.RoleFleetManager)
	if err != nil {
		t.Fatalf("unexpected error issuing ticket: %v", err)
	}
	if rawTicket == "" || ttl != 30 {
		t.Fatalf("expected 30s ttl and non-empty ticket, got ttl=%d, ticket=%s", ttl, rawTicket)
	}

	// 1. Consume ticket first time: succeeds
	uID, role, err := realtime.ConsumeTicket(ctx, pool, rawTicket)
	if err != nil {
		t.Fatalf("failed to consume valid ticket: %v", err)
	}
	if uID != "user-test-1" || role != rbac.RoleFleetManager {
		t.Fatalf("mismatched user/role: got %s / %s", uID, role)
	}

	// 2. Consume ticket second time: fails with ErrInvalidTicket (single-use)
	_, _, err = realtime.ConsumeTicket(ctx, pool, rawTicket)
	if err == nil {
		t.Fatalf("expected error consuming already used ticket, got nil")
	}

	// 3. Invalid ticket: fails
	_, _, err = realtime.ConsumeTicket(ctx, pool, "invalid-random-ticket")
	if err == nil {
		t.Fatalf("expected error consuming non-existent ticket, got nil")
	}
}

func TestPolicy_RoleEventMatrix(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		role      string
		ownsTrip  bool
		allowed   bool
	}{
		// Admin: receives all
		{"Admin receives trip.created", realtime.EventTripCreated, rbac.RoleAdmin, false, true},
		{"Admin receives expense.created", realtime.EventExpenseCreated, rbac.RoleAdmin, false, true},
		{"Admin receives maintenance.opened", realtime.EventMaintenanceOpened, rbac.RoleAdmin, false, true},

		// Fleet Manager: receives all
		{"Fleet Manager receives trip.dispatched", realtime.EventTripDispatched, rbac.RoleFleetManager, false, true},
		{"Fleet Manager receives driver.status_changed", realtime.EventDriverStatusChange, rbac.RoleFleetManager, false, true},

		// Driver: receives own trips, own status change, never maintenance or expenses
		{"Driver receives own trip.created", realtime.EventTripCreated, rbac.RoleDriver, true, true},
		{"Driver receives own trip.completed", realtime.EventTripCompleted, rbac.RoleDriver, true, true},
		{"Driver blocked from other driver's trip.completed", realtime.EventTripCompleted, rbac.RoleDriver, false, false},
		{"Driver blocked from expense.created", realtime.EventExpenseCreated, rbac.RoleDriver, true, false},
		{"Driver blocked from maintenance.opened", realtime.EventMaintenanceOpened, rbac.RoleDriver, true, false},
		{"Driver receives own driver.status_changed", realtime.EventDriverStatusChange, rbac.RoleDriver, true, true},
		{"Driver blocked from other driver.status_changed", realtime.EventDriverStatusChange, rbac.RoleDriver, false, false},

		// Financial Analyst: receives expense.created and trip.completed, never driver or maintenance
		{"Financial Analyst receives expense.created", realtime.EventExpenseCreated, rbac.RoleFinancialAnalyst, false, true},
		{"Financial Analyst receives trip.completed", realtime.EventTripCompleted, rbac.RoleFinancialAnalyst, false, true},
		{"Financial Analyst blocked from trip.created", realtime.EventTripCreated, rbac.RoleFinancialAnalyst, false, false},
		{"Financial Analyst blocked from maintenance.opened", realtime.EventMaintenanceOpened, rbac.RoleFinancialAnalyst, false, false},
		{"Financial Analyst blocked from driver.status_changed", realtime.EventDriverStatusChange, rbac.RoleFinancialAnalyst, false, false},

		// Safety Officer: receives driver status, maintenance, notifications; never expenses or trips
		{"Safety Officer receives driver.status_changed", realtime.EventDriverStatusChange, rbac.RoleSafetyOfficer, false, true},
		{"Safety Officer receives maintenance.opened", realtime.EventMaintenanceOpened, rbac.RoleSafetyOfficer, false, true},
		{"Safety Officer receives notification.created", realtime.EventNotificationCreated, rbac.RoleSafetyOfficer, false, true},
		{"Safety Officer blocked from trip.created", realtime.EventTripCreated, rbac.RoleSafetyOfficer, false, false},
		{"Safety Officer blocked from expense.created", realtime.EventExpenseCreated, rbac.RoleSafetyOfficer, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := realtime.CanReceive(tt.eventType, tt.role, tt.ownsTrip)
			if got != tt.allowed {
				t.Errorf("CanReceive(%s, %s, %v) = %v; want %v", tt.eventType, tt.role, tt.ownsTrip, got, tt.allowed)
			}
		})
	}
}

func TestPolicy_SanitizedClientPayloadPII(t *testing.T) {
	raw := map[string]any{
		"trip_id":        int64(42),
		"vehicle_reg":    "TRK-001",
		"driver_license": "DL-12345",
		"driver_name":    "Alice Driver",
		"name":           "Alice Driver",
		"email":          "alice@example.com",
		"phone":          "+15551234567",
		"contact_number": "+15559876543",
		"status":         "Dispatched",
	}

	sanitized := realtime.SanitizedClientPayload(raw)

	// Safe fields present
	if sanitized["trip_id"] != int64(42) || sanitized["vehicle_reg"] != "TRK-001" || sanitized["status"] != "Dispatched" {
		t.Fatalf("safe fields were stripped: %+v", sanitized)
	}

	// PII fields stripped
	forbidden := []string{"driver_name", "name", "email", "phone", "contact_number", "driver_license"}
	for _, f := range forbidden {
		if _, exists := sanitized[f]; exists {
			t.Errorf("forbidden PII field %s still present in sanitized payload: %+v", f, sanitized)
		}
	}
}

func TestEvents_PayloadSizeCheck(t *testing.T) {
	pool := getDB(t)
	defer pool.Close()
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	// 1. Normal size: succeeds
	err = realtime.PublishTx(ctx, tx, realtime.EventTripCreated, map[string]any{"trip_id": 1})
	if err != nil {
		t.Fatalf("expected success for normal payload, got: %v", err)
	}

	// 2. Exceeds 8000 bytes: rejected
	hugeMap := map[string]any{
		"big_data": strings.Repeat("A", 8500),
	}
	err = realtime.PublishTx(ctx, tx, realtime.EventTripCreated, hugeMap)
	if err == nil {
		t.Fatalf("expected error for payload exceeding 8KB, got nil")
	}
}
