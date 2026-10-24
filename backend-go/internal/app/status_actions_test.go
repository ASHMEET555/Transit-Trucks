package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func TestStatus_GenericPatchRejectsStatus(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "status_admin1@example.com", rbac.RoleAdmin)

	reg := fmt.Sprintf("VEH-ST-%s", uuid.NewString()[:6])
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Transit', 'Van', 1200, 5000, 25000, 'Available', true)`, reg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", reg)
	}()

	// 1. PATCH vehicle with status field -> must return 422
	patchBody, _ := json.Marshal(map[string]any{
		"status": "Retired",
	})
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/vehicles/%s", reg), bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when PATCH contains status, got %d: %s", w.Code, w.Body.String())
	}

	// 2. PATCH driver with status field -> must return 422
	lic := fmt.Sprintf("LIC-ST-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Bob Driver', 'Heavy', '2028-01-01', '555-1234', 98, 'Available', true)`, lic)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
	}()

	dPatchBody, _ := json.Marshal(map[string]any{
		"status": "Suspended",
	})
	req2 := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/drivers/%s", lic), bytes.NewReader(dPatchBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when PATCH contains status for driver, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestStatus_NamedActions(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "status_admin2@example.com", rbac.RoleAdmin)

	ctx := context.Background()
	reg := fmt.Sprintf("VEH-ACT-%s", uuid.NewString()[:6])
	_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'Volvo Truck', 'Semi', 15000, 20000, 80000, 'Available', true)`, reg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", reg)
	}()

	// Test retire vehicle (Available -> Retired)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/vehicles/%s/retire", reg), nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retire vehicle failed, expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status string
	_ = pool.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", reg).Scan(&status)
	if status != "Retired" {
		t.Errorf("expected status 'Retired', got %s", status)
	}

	// Test driver state transitions:
	lic := fmt.Sprintf("LIC-ACT-%s", uuid.NewString()[:6])
	_, err = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Alice Driver', 'Medium', '2028-06-01', '555-4321', 95, 'Available', true)`, lic)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
	}()

	// 1. Available -> Off Duty
	reqOff := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drivers/%s/set-off-duty", lic), nil)
	reqOff.AddCookie(adminCookie)
	wOff := httptest.NewRecorder()
	app.ServeHTTP(wOff, reqOff)
	if wOff.Code != http.StatusOK {
		t.Fatalf("set-off-duty failed: %d: %s", wOff.Code, wOff.Body.String())
	}
	_ = pool.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", lic).Scan(&status)
	if status != "Off Duty" {
		t.Errorf("expected status 'Off Duty', got %s", status)
	}

	// 2. Off Duty -> Available
	reqAvail := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drivers/%s/set-available", lic), nil)
	reqAvail.AddCookie(adminCookie)
	wAvail := httptest.NewRecorder()
	app.ServeHTTP(wAvail, reqAvail)
	if wAvail.Code != http.StatusOK {
		t.Fatalf("set-available failed: %d: %s", wAvail.Code, wAvail.Body.String())
	}

	// 3. Available -> Suspend
	reqSusp := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drivers/%s/suspend", lic), nil)
	reqSusp.AddCookie(adminCookie)
	wSusp := httptest.NewRecorder()
	app.ServeHTTP(wSusp, reqSusp)
	if wSusp.Code != http.StatusOK {
		t.Fatalf("suspend failed: %d: %s", wSusp.Code, wSusp.Body.String())
	}
	_ = pool.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", lic).Scan(&status)
	if status != "Suspended" {
		t.Errorf("expected status 'Suspended', got %s", status)
	}

	// 4. While Suspended -> set-off-duty should be rejected with 409
	reqOffBad := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drivers/%s/set-off-duty", lic), nil)
	reqOffBad.AddCookie(adminCookie)
	wOffBad := httptest.NewRecorder()
	app.ServeHTTP(wOffBad, reqOffBad)
	if wOffBad.Code != http.StatusConflict {
		t.Fatalf("expected 409 when setting suspended driver off-duty, got %d: %s", wOffBad.Code, wOffBad.Body.String())
	}

	// 5. Suspend -> Reinstate
	reqReinst := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/drivers/%s/reinstate", lic), nil)
	reqReinst.AddCookie(adminCookie)
	wReinst := httptest.NewRecorder()
	app.ServeHTTP(wReinst, reqReinst)
	if wReinst.Code != http.StatusOK {
		t.Fatalf("reinstate failed: %d: %s", wReinst.Code, wReinst.Body.String())
	}
	_ = pool.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", lic).Scan(&status)
	if status != "Available" {
		t.Errorf("expected status 'Available' after reinstate, got %s", status)
	}
}
