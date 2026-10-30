package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

// ---------------- PHASE 6 BACKEND: geo, utilization, live trips ----------------

func TestPhase6_DestinationsGeoValidation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.geo@transitops.com", rbac.RoleAdmin)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/destinations/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		return w
	}

	// Valid coords
	if w := post(`{"name":"Geo Hub A","region":"Test","latitude":40.7,"longitude":-74.0}`); w.Code != http.StatusCreated {
		t.Errorf("valid coords expected 201, got %d (%s)", w.Code, w.Body.String())
	}
	// Latitude out of range
	if w := post(`{"name":"Geo Hub B","region":"Test","latitude":95.0,"longitude":0}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad latitude expected 422, got %d", w.Code)
	}
	// Only one of lat/lng
	if w := post(`{"name":"Geo Hub C","region":"Test","latitude":40.7}`); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("half coords expected 422, got %d", w.Code)
	}
	// DB CHECK backstop: raw SQL with bad latitude must fail
	_, err := db.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1,'x',200,0)`,
		"Raw Bad Geo "+uuid.NewString()[:6])
	if err == nil {
		t.Errorf("raw SQL bad latitude should violate CHECK constraint")
	}
}

func TestPhase6_UtilizationRoleMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.util@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.util@transitops.com", rbac.RoleFleetManager)
	finCookie := createTestUserAndLogin(t, app, "fin.util@transitops.com", rbac.RoleFinancialAnalyst)
	safetyCookie := createTestUserAndLogin(t, app, "safety.util@transitops.com", rbac.RoleSafetyOfficer)

	driverLic := fmt.Sprintf("UTIL-DRV-%s", uuid.NewString()[:6])
	createP4Driver(t, db, driverLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))
	driverEmail := fmt.Sprintf("driver.util.%s@transitops.com", uuid.NewString()[:6])
	driverCookie := createTestUserAndLogin(t, app, driverEmail, rbac.RoleDriver)
	_, _ = db.Exec(context.Background(), `UPDATE users SET driver_license = $1 WHERE email = $2`, driverLic, driverEmail)

	check := func(cookie *http.Cookie, path string, expected int) {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expected {
			t.Errorf("GET %s expected %d, got %d", path, expected, w.Code)
		}
	}

	check(adminCookie, "/api/v1/dashboard/utilization", http.StatusOK)
	check(fleetCookie, "/api/v1/dashboard/utilization", http.StatusOK)
	check(finCookie, "/api/v1/dashboard/utilization", http.StatusOK)
	check(safetyCookie, "/api/v1/dashboard/utilization", http.StatusOK)
	check(driverCookie, "/api/v1/dashboard/utilization", http.StatusOK)
	check(adminCookie, "/api/v1/dashboard/utilization?days=0", http.StatusUnprocessableEntity)
	check(adminCookie, "/api/v1/dashboard/utilization?days=91", http.StatusUnprocessableEntity)
}

func TestPhase6_LiveTripsScopeAndEstimate(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()
	suffix := uuid.NewString()[:6]

	// Two geo destinations
	_, err := db.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES
		($1, 'T', 40.0, -74.0), ($2, 'T', 41.0, -87.0)
		ON CONFLICT (name) DO UPDATE SET latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude`,
		"Live Src "+suffix, "Live Dst "+suffix)
	if err != nil {
		t.Fatalf("seed destinations: %v", err)
	}
	// One geo-less destination pair must NOT appear on the map
	_, _ = db.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1,'T'), ($2,'T')
		ON CONFLICT (name) DO NOTHING`, "Flat Src "+suffix, "Flat Dst "+suffix)

	reg := "LIVE-VH-" + suffix
	createP4Vehicle(t, db, reg, "On Trip", "Semi", 20000, 50000, 80000, true)
	lic := "LIVE-DL-" + suffix
	createP4Driver(t, db, lic, "On Trip", "Heavy", time.Now().AddDate(1, 0, 0))

	reg2 := "LIVE2-VH-" + suffix
	createP4Vehicle(t, db, reg2, "On Trip", "Semi", 20000, 50000, 80000, true)
	lic2 := "LIVE2-DL-" + suffix
	createP4Driver(t, db, lic2, "On Trip", "Heavy", time.Now().AddDate(1, 0, 0))

	_, err = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license,
		cargo_weight, planned_distance, revenue, status, start_odometer, dispatched_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 800, 500, 'Dispatched', 50000, now() - INTERVAL '2 hours', now(), now())`,
		"Live Src "+suffix, "Live Dst "+suffix, reg, lic)
	if err != nil {
		t.Fatalf("seed live trip: %v", err)
	}
	_, err = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license,
		cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1000, 800, 500, 'Dispatched', now(), now())`,
		"Flat Src "+suffix, "Flat Dst "+suffix, reg2, lic2)
	if err != nil {
		t.Fatalf("seed flat trip: %v", err)
	}

	adminCookie := createTestUserAndLogin(t, app, "admin.live@transitops.com", rbac.RoleAdmin)
	safetyCookie := createTestUserAndLogin(t, app, "safety.live@transitops.com", rbac.RoleSafetyOfficer)
	driverEmail := fmt.Sprintf("driver.live.%s@transitops.com", suffix)
	driverCookie := createTestUserAndLogin(t, app, driverEmail, rbac.RoleDriver)
	_, _ = db.Exec(ctx, `UPDATE users SET driver_license = $1 WHERE email = $2`, lic, driverEmail)

	get := func(cookie *http.Cookie) (int, LiveTripsResponse) {
		req := httptest.NewRequest("GET", "/api/v1/trips/live", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		var resp LiveTripsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return w.Code, resp
	}

	// Admin: only the geo trip, progress = 2h*80/800 = 0.2, license present
	code, resp := get(adminCookie)
	if code != http.StatusOK {
		t.Fatalf("admin live expected 200, got %d", code)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 mappable trip, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.EstimatedProgress < 0.19 || item.EstimatedProgress > 0.21 {
		t.Errorf("expected progress ~0.2, got %v", item.EstimatedProgress)
	}
	if item.DriverLicense == nil || *item.DriverLicense != lic {
		t.Errorf("admin should see driver license")
	}
	if resp.AssumedSpeedKmh != AssumedAverageSpeedKMH || resp.EstimatedWarning == "" {
		t.Errorf("response must carry assumption disclosure")
	}

	// Safety: license stripped
	_, sResp := get(safetyCookie)
	if len(sResp.Items) != 1 || sResp.Items[0].DriverLicense != nil {
		t.Errorf("safety payload must omit driver_license")
	}

	// Driver sees own trip (same single trip here)
	code, dResp := get(driverCookie)
	if code != http.StatusOK || len(dResp.Items) != 1 {
		t.Errorf("driver expected 1 own trip, got code=%d items=%d", code, len(dResp.Items))
	}
}
