package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func TestSoftDelete_VehicleRules(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "del_admin1@example.com", rbac.RoleAdmin)
	ctx := context.Background()

	// 1. Vehicle currently On Trip cannot be deleted -> 409
	regOnTrip := fmt.Sprintf("VEH-ONT-%s", uuid.NewString()[:6])
	_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Truck A', 'Semi', 10000, 50000, 60000, 'On Trip', true)`, regOnTrip)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regOnTrip)
	}()

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regOnTrip), nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting vehicle On Trip, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Vehicle with Open maintenance cannot be deleted -> 409
	regMaint := fmt.Sprintf("VEH-MNT-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Truck B', 'Box Truck', 8000, 30000, 45000, 'In Shop', true)`, regMaint)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status)
		VALUES ($1, 'Brake service', 500.0, '2026-01-01', 'Open')`, regMaint)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM maintenance_logs WHERE vehicle_reg = $1", regMaint)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regMaint)
	}()

	req2 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regMaint), nil)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting vehicle with open maintenance, got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Vehicle with completed trip history -> soft deleted (is_active = false)
	regHist := fmt.Sprintf("VEH-HST-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Truck C', 'Van', 2000, 10000, 20000, 'Available', true)`, regHist)
	if err != nil {
		t.Fatal(err)
	}
	// Insert dummy destination and driver for trip
	hubA := fmt.Sprintf("HUB-A-%s", uuid.NewString()[:6])
	hubB := fmt.Sprintf("HUB-B-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'North')`, hubA)
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'South')`, hubB)
	lic := fmt.Sprintf("LIC-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Driver C', 'Van', '2028-01-01', '555-0000', 95, 'Available', true)`, lic)

	_, err = pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES ($1, $2, $3, $4, 500, 100, 1000, 'Completed')`, hubA, hubB, regHist, lic)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", regHist)
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
		_, _ = pool.Exec(ctx, "DELETE FROM destinations WHERE name IN ($1, $2)", hubA, hubB)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regHist)
	}()

	req3 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regHist), nil)
	req3.AddCookie(adminCookie)
	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 on soft delete of vehicle with history, got %d: %s", w3.Code, w3.Body.String())
	}

	// Verify vehicle still exists in DB, but is_active = false
	var isActive bool
	err = pool.QueryRow(ctx, "SELECT is_active FROM vehicles WHERE registration_number = $1", regHist).Scan(&isActive)
	if err != nil {
		t.Fatalf("vehicle should still exist in DB: %v", err)
	}
	if isActive {
		t.Fatalf("expected is_active = false, got true")
	}

	// 4. Vehicle with ZERO history -> hard deleted
	regFresh := fmt.Sprintf("VEH-FRSH-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Truck Fresh', 'Van', 1500, 100, 15000, 'Available', true)`, regFresh)
	if err != nil {
		t.Fatal(err)
	}

	req4 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regFresh), nil)
	req4.AddCookie(adminCookie)
	w4 := httptest.NewRecorder()
	app.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 on hard delete, got %d: %s", w4.Code, w4.Body.String())
	}

	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM vehicles WHERE registration_number = $1)", regFresh).Scan(&exists)
	if err != nil || exists {
		t.Fatalf("expected vehicle with no history to be hard deleted from DB")
	}
}

func TestSoftDelete_DriverRules(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "del_admin2@example.com", rbac.RoleAdmin)
	ctx := context.Background()

	// 1. Driver on trip cannot be deleted -> 409
	licOnTrip := fmt.Sprintf("LIC-ONT-%s", uuid.NewString()[:6])
	_, err := pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Driver On Trip', 'Heavy', '2028-01-01', '555-1111', 90, 'On Trip', true)`, licOnTrip)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", licOnTrip)
	}()

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/drivers/%s", licOnTrip), nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting driver On Trip, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Driver with trip history -> soft deleted
	licHist := fmt.Sprintf("LIC-HST-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Driver With Hist', 'Medium', '2028-01-01', '555-2222', 92, 'Available', true)`, licHist)
	if err != nil {
		t.Fatal(err)
	}
	hub1 := fmt.Sprintf("HUB1-%s", uuid.NewString()[:6])
	hub2 := fmt.Sprintf("HUB2-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'South')`, hub1)
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'East')`, hub2)
	vReg := fmt.Sprintf("VEH-DH-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Van X', 'Van', 1000, 2000, 10000, 'Available', true)`, vReg)

	_, err = pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES ($1, $2, $3, $4, 300, 50, 500, 'Completed')`, hub1, hub2, vReg, licHist)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE driver_license = $1", licHist)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		_, _ = pool.Exec(ctx, "DELETE FROM destinations WHERE name IN ($1, $2)", hub1, hub2)
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", licHist)
	}()

	req2 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/drivers/%s", licHist), nil)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on soft delete of driver with history, got %d: %s", w2.Code, w2.Body.String())
	}

	var isActive bool
	err = pool.QueryRow(ctx, "SELECT is_active FROM drivers WHERE license_number = $1", licHist).Scan(&isActive)
	if err != nil || isActive {
		t.Fatalf("expected driver to exist with is_active = false, got err=%v, isActive=%v", err, isActive)
	}

	// 3. Driver with no history -> hard deleted
	licFresh := fmt.Sprintf("LIC-FRSH-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Driver Fresh', 'Van', '2028-01-01', '555-3333', 99, 'Available', true)`, licFresh)
	if err != nil {
		t.Fatal(err)
	}

	req3 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/drivers/%s", licFresh), nil)
	req3.AddCookie(adminCookie)
	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 on hard delete, got %d: %s", w3.Code, w3.Body.String())
	}

	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM drivers WHERE license_number = $1)", licFresh).Scan(&exists)
	if err != nil || exists {
		t.Fatalf("expected driver with no history to be hard deleted from DB")
	}
}
