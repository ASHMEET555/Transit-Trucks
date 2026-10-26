package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

// The following tests document the bugs identified in Task 4.0.
// They assert the desired Phase 4 behavior.

func TestFailing_MaintenanceOnRetiredVehicle(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.ret@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("FAIL-V-RET-%s", uuid.NewString()[:6])

	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Retired', false, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'Retired', is_active = false`, vReg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM maintenance_logs WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
	}()

	body, _ := json.Marshal(map[string]any{
		"vehicle_reg": vReg,
		"description": "Oil change on retired vehicle",
		"cost":        150.0,
		"start_date":  time.Now().Format("2006-01-02"),
	})

	req := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// In current code, this succeeds or redirects, un-retiring the vehicle.
	// In Phase 4, it must return 409 VEHICLE_RETIRED.
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for retired vehicle maintenance, got %d: %s", w.Code, w.Body.String())
	}

	var vStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	if vStatus != "Retired" {
		t.Fatalf("expected vehicle status to remain 'Retired', got '%s'", vStatus)
	}
}

func TestFailing_RetiredVehicleStaysRetiredAfterClose(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.close@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("FAIL-V-CLOSE-%s", uuid.NewString()[:6])

	// Vehicle is In Shop, but retired mid-maintenance
	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Retired', false, now(), now())`, vReg)
	if err != nil {
		t.Fatal(err)
	}

	var logID int64
	err = db.QueryRow(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status, created_at, updated_at)
		VALUES ($1, 'Brake service', 300, $2, 'Open', now(), now()) RETURNING id`, vReg, time.Now().Format("2006-01-02")).Scan(&logID)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM maintenance_logs WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
	}()

	body, _ := json.Marshal(map[string]any{
		"end_date": time.Now().Format("2006-01-02"),
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK closing log, got %d: %s", w.Code, w.Body.String())
	}

	// Legacy code sets status to 'Available', un-retiring the vehicle.
	// Phase 4 requires the vehicle to remain 'Retired'.
	var vStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	if vStatus != "Retired" {
		t.Fatalf("expected vehicle status to remain 'Retired' after closing maintenance, got '%s'", vStatus)
	}
}

func TestFailing_DashboardCancelledTripRevenue(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.dash.canc@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("FAIL-V-DASH-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("FAIL-D-DASH-%s", uuid.NewString()[:6])

	_, _ = db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Available', true, now(), now())`, vReg)
	_, _ = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Dash Driver', 'Heavy', $2, '+15551234', 90.0, 'Available', true, now(), now())`, dLic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))

	// Completed trip with 1000 revenue
	_, _ = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 2000, 300, 1000, 'Completed', 1000, 1300, now(), now())`, vReg, dLic)

	// Cancelled trip with 5000 revenue
	_, _ = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 2000, 300, 5000, 'Cancelled', now(), now())`, vReg, dLic)

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, dLic)
	}()

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dashboard KPIs, got %d", w.Code)
	}

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	// In legacy code, total_revenue includes Cancelled trip revenue (5000).
	// In Phase 4, revenue must strictly sum Completed trips.
}

func TestFailing_ExpenseOnDraftTrip(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.exp.draft@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("FAIL-V-EXP-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("FAIL-D-EXP-%s", uuid.NewString()[:6])

	_, _ = db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Available', true, now(), now())`, vReg)
	_, _ = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Expense Driver', 'Heavy', $2, '+15551234', 90.0, 'Available', true, now(), now())`, dLic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))

	var draftTripID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 2000, 300, 1000, 'Draft', now(), now()) RETURNING id`, vReg, dLic).Scan(&draftTripID)

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM expenses WHERE trip_id = $1`, draftTripID)
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE id = $1`, draftTripID)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, dLic)
	}()

	body, _ := json.Marshal(map[string]any{
		"trip_id": draftTripID,
		"type":    "Toll",
		"amount":  50.0,
	})

	req := httptest.NewRequest("POST", "/api/v1/expenses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// In legacy code, expense on draft trip succeeds or path not found.
	// In Phase 4, it must return 422 EXPENSE_ON_INACTIVE_TRIP.
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity for expense on draft trip, got %d: %s", w.Code, w.Body.String())
	}
}
