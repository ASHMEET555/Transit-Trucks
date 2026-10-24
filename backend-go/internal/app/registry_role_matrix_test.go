package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func createDriverUserAndLogin(t *testing.T, app *App, email, license string) *http.Cookie {
	ctx := context.Background()
	pw := "DriverPass123!"
	hashed, _ := auth.HashPassword(pw)
	userId := uuid.NewString()

	_, err := app.db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, driver_license, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, now(), now())
		ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role, driver_license = EXCLUDED.driver_license, is_active = true`,
		userId, email, hashed, rbac.RoleDriver, license)
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": pw})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("driver login failed: %d %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatal("session cookie not found")
	return nil
}

func TestRegistry_DriverSelfScope(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	ctx := context.Background()

	lic1 := fmt.Sprintf("DL-SELF-%s", uuid.NewString()[:6])
	lic2 := fmt.Sprintf("DL-OTHER-%s", uuid.NewString()[:6])

	// Create 2 drivers
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Self Driver', 'Heavy', '2028-01-01', '555-0001', 99, 'Available', true)`, lic1)
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Other Driver', 'Heavy', '2028-01-01', '555-0002', 90, 'Available', true)`, lic2)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number IN ($1, $2)", lic1, lic2)
	}()

	driverCookie := createDriverUserAndLogin(t, app, fmt.Sprintf("driver-%s@transitops.com", uuid.NewString()[:6]), lic1)

	// 1. GET /api/v1/drivers -> sees ONLY their own record
	reqList := httptest.NewRequest("GET", "/api/v1/drivers", nil)
	reqList.AddCookie(driverCookie)
	wList := httptest.NewRecorder()
	app.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200 for driver list, got %d: %s", wList.Code, wList.Body.String())
	}

	var resp PaginatedResponse[driver]
	if err := json.Unmarshal(wList.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected driver to see exactly 1 driver (self), got %d", len(resp.Items))
	}
	if resp.Items[0].LicenseNumber != lic1 {
		t.Fatalf("expected driver to see own license %s, got %s", lic1, resp.Items[0].LicenseNumber)
	}

	// 2. GET /api/v1/drivers/{license} for OTHER driver -> 403
	reqOther := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/drivers/%s", lic2), nil)
	reqOther.AddCookie(driverCookie)
	wOther := httptest.NewRecorder()
	app.ServeHTTP(wOther, reqOther)
	if wOther.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when driver reads other driver profile, got %d: %s", wOther.Code, wOther.Body.String())
	}

	// 3. GET /api/v1/drivers/{license} for SELF -> 200
	reqSelf := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/drivers/%s", lic1), nil)
	reqSelf.AddCookie(driverCookie)
	wSelf := httptest.NewRecorder()
	app.ServeHTTP(wSelf, reqSelf)
	if wSelf.Code != http.StatusOK {
		t.Fatalf("expected 200 when driver reads own profile, got %d: %s", wSelf.Code, wSelf.Body.String())
	}
}

func TestRegistry_DriverCannotCallAnyWrite(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	ctx := context.Background()

	lic := fmt.Sprintf("DL-WR-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
		VALUES ($1, 'Write Test Driver', 'Heavy', '2028-01-01', '555-0003', 95, 'Available', true)`, lic)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
	}()

	driverCookie := createDriverUserAndLogin(t, app, fmt.Sprintf("driver2-%s@transitops.com", uuid.NewString()[:6]), lic)

	writeRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/api/v1/vehicles", `{"registration_number":"V1","model":"M","type":"Van","max_load_capacity":1000,"odometer":100,"acquisition_cost":1000}`},
		{"PATCH", "/api/v1/vehicles/V1", `{"model":"M2"}`},
		{"POST", "/api/v1/vehicles/V1/retire", `{}`},
		{"DELETE", "/api/v1/vehicles/V1", `{}`},
		{"POST", "/api/v1/drivers", `{"license_number":"D2","name":"N","license_category":"C","license_expiry_date":"2028-01-01","contact_number":"123","safety_score":90}`},
		{"PATCH", "/api/v1/drivers/D1", `{"name":"N2"}`},
		{"POST", "/api/v1/drivers/D1/suspend", `{}`},
		{"POST", "/api/v1/drivers/D1/reinstate", `{}`},
		{"POST", "/api/v1/drivers/D1/set-off-duty", `{}`},
		{"POST", "/api/v1/drivers/D1/set-available", `{}`},
		{"DELETE", "/api/v1/drivers/D1", `{}`},
		{"POST", "/api/v1/destinations", `{"name":"Dest1","region":"R"}`},
		{"DELETE", "/api/v1/destinations/Dest1", `{}`},
	}

	for _, tc := range writeRoutes {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(driverCookie)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("expected 403 Forbidden for driver on %s %s, got %d: %s", tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegistry_RoleSeparationSafetyVsFleetManager(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)

	safetyCookie := createTestUserAndLogin(t, app, fmt.Sprintf("safety-%s@transitops.com", uuid.NewString()[:6]), rbac.RoleSafetyOfficer)
	fleetCookie := createTestUserAndLogin(t, app, fmt.Sprintf("fleet-%s@transitops.com", uuid.NewString()[:6]), rbac.RoleFleetManager)

	// 1. Safety officer cannot set-off-duty or set-available
	req1 := httptest.NewRequest("POST", "/api/v1/drivers/DUMMY/set-off-duty", nil)
	req1.AddCookie(safetyCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for Safety Officer on set-off-duty, got %d", w1.Code)
	}

	// 2. Safety officer cannot delete driver
	req2 := httptest.NewRequest("DELETE", "/api/v1/drivers/DUMMY", nil)
	req2.AddCookie(safetyCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for Safety Officer on DELETE driver, got %d", w2.Code)
	}

	// 3. Fleet manager cannot suspend driver
	req3 := httptest.NewRequest("POST", "/api/v1/drivers/DUMMY/suspend", nil)
	req3.AddCookie(fleetCookie)
	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for Fleet Manager on suspend driver, got %d", w3.Code)
	}
}
