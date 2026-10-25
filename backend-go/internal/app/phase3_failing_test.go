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

// The following tests document the bugs identified in Task 3.0.
// They assert the desired Phase 3 behavior and fail against the legacy Phase 2 trip implementation.

func TestFailing_DraftTripCreation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.draft.fail@transitops.com", rbac.RoleAdmin)
	vReg := "FAIL-V-DRAFT"
	dLic := "FAIL-D-DRAFT"

	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Available', true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'Available', is_active = true`, vReg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Draft Driver', 'Heavy', $2, '+15551234', 90.0, 'Available', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'Available', is_active = true`, dLic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, dLic)
	}()

	body, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     5000,
		"planned_distance": 800,
		"revenue":          2500,
		// No "dispatch": true -> Must create a Draft trip without locking resources
	})

	req := httptest.NewRequest("POST", "/api/v1/trips/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for draft trip, got %d (body: %s)", w.Code, w.Body.String())
	}

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res["status"] != "Draft" {
		t.Fatalf("expected trip status 'Draft', got %v (current code creates Dispatched directly)", res["status"])
	}

	// Verify vehicle and driver status did NOT change
	var vStatus, dStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dLic).Scan(&dStatus)
	if vStatus != "Available" || dStatus != "Available" {
		t.Fatalf("expected vehicle and driver to remain Available, got vehicle=%s, driver=%s", vStatus, dStatus)
	}
}

func TestFailing_CompleteOwnershipCheck_S04(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	driver1Lic := "FAIL-D-LIC1"
	driver2Lic := "FAIL-D-LIC2"
	vReg := "FAIL-V-S04"

	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'On Trip', true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'On Trip'`, vReg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Driver 1', 'Heavy', $2, '+15551234', 90.0, 'On Trip', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'On Trip'`, driver1Lic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Driver 2', 'Heavy', $2, '+15555678', 90.0, 'Available', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'Available'`, driver2Lic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}

	driver2Email := fmt.Sprintf("driver2.fail.%s@transitops.com", uuid.NewString()[:6])
	driver2Cookie := createTestUserAndLogin(t, app, driver2Email, rbac.RoleDriver)
	_, _ = db.Exec(ctx, `UPDATE users SET driver_license = $1 WHERE email = $2`, driver2Lic, driver2Email)

	// Create trip belonging to driver1
	var tripID int64
	err = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 5000, 800, 2500, 'Dispatched', now(), now())
		RETURNING id`, vReg, driver1Lic).Scan(&tripID)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE id = $1`, tripID)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number IN ($1, $2)`, driver1Lic, driver2Lic)
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, driver2Email)
	}()

	// Driver 2 attempts to complete Driver 1's trip
	completeBody, _ := json.Marshal(map[string]any{
		"final_odometer": 10500,
	})
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/trips/%d/complete?final_odometer=10500", tripID), bytes.NewReader(completeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.AddCookie(driver2Cookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Bug S-04: Must return 403 Forbidden because driver2 does not own driver1's trip. Current code returns 200 OK!
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for driver completing another driver's trip, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestFailing_IdempotencyHeaderRequired(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.idem.fail@transitops.com", rbac.RoleAdmin)
	vReg := "FAIL-V-IDEM"
	dLic := "FAIL-D-IDEM"

	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Van', 'Van', 10000, 1000, 25000, 'Available', true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'Available'`, vReg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Idem Driver', 'Heavy', $2, '+15551234', 90.0, 'Available', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'Available'`, dLic, time.Now().AddDate(1, 0, 0).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, dLic)
	}()

	body, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     5000,
		"planned_distance": 800,
		"revenue":          2500,
	})

	// Request missing Idempotency-Key header
	req := httptest.NewRequest("POST", "/api/v1/trips/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Phase 3 requires 400 IDEMPOTENCY_KEY_REQUIRED. Current code returns 201 without header.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 IDEMPOTENCY_KEY_REQUIRED for missing header, got %d", w.Code)
	}

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	errObj, _ := res["error"].(map[string]any)
	if errObj["code"] != "IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("expected code IDEMPOTENCY_KEY_REQUIRED, got %v", errObj["code"])
	}
}
