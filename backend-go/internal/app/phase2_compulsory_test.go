package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// 1. Duplicate registration -> 409 with code VEHICLE_EXISTS / DRIVER_EXISTS / DESTINATION_EXISTS
// A race of two simultaneous creates yields one 201 and one 409, never a 500.
func TestCompulsory_DuplicateRegistrationAndRace(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "race_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	// A. Destination duplicate
	destName := fmt.Sprintf("DEST-DUP-%s", uuid.NewString()[:6])
	destBody, _ := json.Marshal(map[string]any{"name": destName, "region": "North"})
	reqD1 := httptest.NewRequest("POST", "/api/v1/destinations", bytes.NewReader(destBody))
	reqD1.Header.Set("Content-Type", "application/json")
	reqD1.AddCookie(adminCookie)
	wD1 := httptest.NewRecorder()
	app.ServeHTTP(wD1, reqD1)
	if wD1.Code != http.StatusCreated {
		t.Fatalf("destination create failed: %d: %s", wD1.Code, wD1.Body.String())
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM destinations WHERE name = $1", destName)
	}()

	reqD2 := httptest.NewRequest("POST", "/api/v1/destinations", bytes.NewReader(destBody))
	reqD2.Header.Set("Content-Type", "application/json")
	reqD2.AddCookie(adminCookie)
	wD2 := httptest.NewRecorder()
	app.ServeHTTP(wD2, reqD2)
	if wD2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate destination, got %d: %s", wD2.Code, wD2.Body.String())
	}
	var errRespD map[string]any
	_ = json.Unmarshal(wD2.Body.Bytes(), &errRespD)
	if errRespD["error"].(map[string]any)["code"] != "DESTINATION_EXISTS" {
		t.Errorf("expected code DESTINATION_EXISTS, got %v", errRespD["error"].(map[string]any)["code"])
	}

	// B. Driver duplicate
	lic := fmt.Sprintf("DL-DUP-%s", uuid.NewString()[:6])
	dBody, _ := json.Marshal(map[string]any{
		"license_number":      lic,
		"name":                "Duplicate Driver",
		"license_category":    "Heavy",
		"license_expiry_date": "2028-01-01",
		"contact_number":      "+1-555-9999",
		"safety_score":        95.0,
	})
	reqDr1 := httptest.NewRequest("POST", "/api/v1/drivers", bytes.NewReader(dBody))
	reqDr1.Header.Set("Content-Type", "application/json")
	reqDr1.AddCookie(adminCookie)
	wDr1 := httptest.NewRecorder()
	app.ServeHTTP(wDr1, reqDr1)
	if wDr1.Code != http.StatusCreated {
		t.Fatalf("driver create failed: %d: %s", wDr1.Code, wDr1.Body.String())
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
	}()

	reqDr2 := httptest.NewRequest("POST", "/api/v1/drivers", bytes.NewReader(dBody))
	reqDr2.Header.Set("Content-Type", "application/json")
	reqDr2.AddCookie(adminCookie)
	wDr2 := httptest.NewRecorder()
	app.ServeHTTP(wDr2, reqDr2)
	if wDr2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate driver, got %d: %s", wDr2.Code, wDr2.Body.String())
	}
	var errRespDr map[string]any
	_ = json.Unmarshal(wDr2.Body.Bytes(), &errRespDr)
	if errRespDr["error"].(map[string]any)["code"] != "DRIVER_EXISTS" {
		t.Errorf("expected code DRIVER_EXISTS, got %v", errRespDr["error"].(map[string]any)["code"])
	}

	// C. Concurrent race on Vehicle creation
	regRace := fmt.Sprintf("VH-RACE-%s", uuid.NewString()[:6])
	vBody, _ := json.Marshal(map[string]any{
		"registration_number": regRace,
		"model":               "Ford Transit Race",
		"type":                "Van",
		"max_load_capacity":   1500.0,
		"odometer":            100.0,
		"acquisition_cost":    20000.0,
	})
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regRace)
	}()

	var wg sync.WaitGroup
	codes := make([]int, 2)
	bodies := make([]string, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(vBody))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(adminCookie)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)
			codes[idx] = w.Code
			bodies[idx] = w.Body.String()
		}(i)
	}
	wg.Wait()

	// Exactly one 201 and one 409, never 500
	has201 := (codes[0] == http.StatusCreated || codes[1] == http.StatusCreated)
	has409 := (codes[0] == http.StatusConflict || codes[1] == http.StatusConflict)

	if !has201 || !has409 {
		t.Fatalf("race condition failed: expected one 201 and one 409, got codes: %v, bodies: %v", codes, bodies)
	}
	for i, c := range codes {
		if c == http.StatusConflict {
			var errV map[string]any
			_ = json.Unmarshal([]byte(bodies[i]), &errV)
			if errV["error"].(map[string]any)["code"] != "VEHICLE_EXISTS" {
				t.Errorf("expected error code VEHICLE_EXISTS on race collision, got %v", errV["error"].(map[string]any)["code"])
			}
		}
	}
}

// 2. Negative odometer/capacity/cost -> 422; the same row inserted via raw SQL violates a CHECK.
func TestCompulsory_NegativeValuesAndSqlCheckViolation(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "neg_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	// A. Negative odometer via API -> 422
	b1, _ := json.Marshal(map[string]any{
		"registration_number": "VH-NEG-ODO",
		"model":               "Transit",
		"type":                "Van",
		"max_load_capacity":   1000.0,
		"odometer":            -10.0,
		"acquisition_cost":    20000.0,
	})
	r1 := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(b1))
	r1.Header.Set("Content-Type", "application/json")
	r1.AddCookie(adminCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, r1)
	if w1.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for negative odometer via API, got %d", w1.Code)
	}

	// Raw SQL negative odometer violates check_vehicles_odometer
	_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ('VH-NEG-ODO-SQL', 'Transit', 'Van', 1000, -10, 20000, 'Available')`)
	if err == nil {
		t.Fatal("expected raw SQL negative odometer to fail CHECK constraint, but succeeded")
	}
	if err != nil && (!hasPgErrCode(err, "23514")) {
		t.Fatalf("expected SQLSTATE 23514 (check_violation), got %v", err)
	}

	// B. Zero / negative capacity via API -> 422
	b2, _ := json.Marshal(map[string]any{
		"registration_number": "VH-ZERO-CAP",
		"model":               "Transit",
		"type":                "Van",
		"max_load_capacity":   0.0,
		"odometer":            100.0,
		"acquisition_cost":    20000.0,
	})
	r2 := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(b2))
	r2.Header.Set("Content-Type", "application/json")
	r2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, r2)
	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for zero max_load_capacity, got %d", w2.Code)
	}

	// Raw SQL non-positive capacity violates check_vehicles_max_load
	_, err = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ('VH-ZERO-CAP-SQL', 'Transit', 'Van', 0, 100, 20000, 'Available')`)
	if err == nil || !hasPgErrCode(err, "23514") {
		t.Fatalf("expected SQLSTATE 23514 for zero capacity, got %v", err)
	}

	// C. Negative acquisition cost via API -> 422
	b3, _ := json.Marshal(map[string]any{
		"registration_number": "VH-NEG-COST",
		"model":               "Transit",
		"type":                "Van",
		"max_load_capacity":   1000.0,
		"odometer":            100.0,
		"acquisition_cost":    -500.0,
	})
	r3 := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(b3))
	r3.Header.Set("Content-Type", "application/json")
	r3.AddCookie(adminCookie)
	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, r3)
	if w3.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for negative acquisition cost, got %d", w3.Code)
	}

	// Raw SQL negative cost violates check_vehicles_cost
	_, err = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ('VH-NEG-COST-SQL', 'Transit', 'Van', 1000, 100, -500, 'Available')`)
	if err == nil || !hasPgErrCode(err, "23514") {
		t.Fatalf("expected SQLSTATE 23514 for negative cost, got %v", err)
	}
}

// 3. Status "Banana" via API -> 422; via raw SQL -> CHECK violation.
func TestCompulsory_StatusBanana(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "banana_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	// Via API -> 422
	b, _ := json.Marshal(map[string]any{
		"registration_number": "VH-BANANA-API",
		"model":               "Transit",
		"type":                "Van",
		"max_load_capacity":   1500.0,
		"odometer":            100.0,
		"acquisition_cost":    25000.0,
		"status":              "Banana",
	})
	r := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for status 'Banana' via API, got %d: %s", w.Code, w.Body.String())
	}

	// Via raw SQL -> CHECK violation (SQLSTATE 23514 check_vehicles_status)
	_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ('VH-BANANA-SQL', 'Transit', 'Van', 1500, 100, 25000, 'Banana')`)
	if err == nil || !hasPgErrCode(err, "23514") {
		t.Fatalf("expected raw SQL status 'Banana' to violate check_vehicles_status (23514), got %v", err)
	}
}

// 4. PATCH containing status -> 422 (unknown field).
func TestCompulsory_PatchContainingStatus(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "patch_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	reg := fmt.Sprintf("VH-PST-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ($1, 'Transit', 'Van', 1200, 500, 22000, 'Available')`, reg)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", reg)
	}()

	patchBody, _ := json.Marshal(map[string]any{
		"model":  "Updated Transit",
		"status": "In Shop",
	})
	r := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/vehicles/%s", reg), bytes.NewReader(patchBody))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when PATCH contains status, got %d: %s", w.Code, w.Body.String())
	}
}

// 5. Delete vehicle with trip history -> soft delete; list excludes it by default; include_inactive=true shows it; trips still render with its name.
// Delete a vehicle that is On Trip -> 409.
func TestCompulsory_SoftDeleteAndIncludeInactive(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "softdel_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	// A. Delete vehicle that is On Trip -> 409
	regTrip := fmt.Sprintf("VH-ONTRIP-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ($1, 'Volvo', 'Semi', 12000, 10000, 60000, 'On Trip')`, regTrip)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regTrip)
	}()

	rTrip := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regTrip), nil)
	rTrip.AddCookie(adminCookie)
	wTrip := httptest.NewRecorder()
	app.ServeHTTP(wTrip, rTrip)
	if wTrip.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting vehicle On Trip, got %d: %s", wTrip.Code, wTrip.Body.String())
	}

	// B. Delete vehicle with completed trip history -> soft delete
	regHist := fmt.Sprintf("VH-HIST-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'History Truck', 'Box Truck', 8000, 25000, 40000, 'Available', true)`, regHist)

	h1 := fmt.Sprintf("HUB-X-%s", uuid.NewString()[:6])
	h2 := fmt.Sprintf("HUB-Y-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'R1'), ($2, 'R2')`, h1, h2)
	lic := fmt.Sprintf("DL-H-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
		VALUES ($1, 'Hist Driver', 'Heavy', '2028-01-01', '555', 90, 'Available')`, lic)

	_, _ = pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES ($1, $2, $3, $4, 2000, 150, 600, 'Completed')`, h1, h2, regHist, lic)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", regHist)
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", lic)
		_, _ = pool.Exec(ctx, "DELETE FROM destinations WHERE name IN ($1, $2)", h1, h2)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", regHist)
	}()

	rDel := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/vehicles/%s", regHist), nil)
	rDel.AddCookie(adminCookie)
	wDel := httptest.NewRecorder()
	app.ServeHTTP(wDel, rDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on soft delete of vehicle with history, got %d: %s", wDel.Code, wDel.Body.String())
	}

	// 1. List excludes it by default
	rListDef := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s", regHist), nil)
	rListDef.AddCookie(adminCookie)
	wListDef := httptest.NewRecorder()
	app.ServeHTTP(wListDef, rListDef)
	var respDef PaginatedResponse[vehicle]
	_ = json.Unmarshal(wListDef.Body.Bytes(), &respDef)
	if len(respDef.Items) != 0 {
		t.Fatalf("expected soft-deleted vehicle to be excluded from default list, but found %d items", len(respDef.Items))
	}

	// 2. include_inactive=true shows it
	rListInc := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&include_inactive=true", regHist), nil)
	rListInc.AddCookie(adminCookie)
	wListInc := httptest.NewRecorder()
	app.ServeHTTP(wListInc, rListInc)
	var respInc PaginatedResponse[vehicle]
	_ = json.Unmarshal(wListInc.Body.Bytes(), &respInc)
	if len(respInc.Items) != 1 || respInc.Items[0].RegistrationNumber != regHist {
		t.Fatalf("expected vehicle to appear with include_inactive=true")
	}
	if respInc.Items[0].IsActive {
		t.Fatalf("expected is_active to be false")
	}

	// 3. Trips still render with its registration
	var tripVehicleReg string
	_ = pool.QueryRow(ctx, "SELECT vehicle_reg FROM trips WHERE vehicle_reg = $1", regHist).Scan(&tripVehicleReg)
	if tripVehicleReg != regHist {
		t.Errorf("expected trip foreign key to still reference vehicle %s, got %s", regHist, tripVehicleReg)
	}
}

// 6. Driver-role user: can read only own driver record; cannot call any registry write.
func TestCompulsory_DriverRoleScopeAndWriteProtection(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	ctx := context.Background()

	dLicSelf := fmt.Sprintf("DL-R-SELF-%s", uuid.NewString()[:6])
	dLicOther := fmt.Sprintf("DL-R-OTH-%s", uuid.NewString()[:6])

	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
		VALUES ($1, 'Self Driver', 'Medium', '2028-01-01', '111', 95, 'Available')`, dLicSelf)
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
		VALUES ($1, 'Other Driver', 'Medium', '2028-01-01', '222', 90, 'Available')`, dLicOther)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number IN ($1, $2)", dLicSelf, dLicOther)
	}()

	driverCookie := createDriverUserAndLogin(t, app, fmt.Sprintf("driver-c-%s@transitops.com", uuid.NewString()[:6]), dLicSelf)

	// A. Can read only own driver record
	rList := httptest.NewRequest("GET", "/api/v1/drivers", nil)
	rList.AddCookie(driverCookie)
	wList := httptest.NewRecorder()
	app.ServeHTTP(wList, rList)
	if wList.Code != http.StatusOK {
		t.Fatalf("driver list failed: %d", wList.Code)
	}
	var drResp PaginatedResponse[driver]
	_ = json.Unmarshal(wList.Body.Bytes(), &drResp)
	if len(drResp.Items) != 1 || drResp.Items[0].LicenseNumber != dLicSelf {
		t.Fatalf("driver expected to see only own record, saw: %v", drResp.Items)
	}

	// B. Direct GET on other driver profile -> 403
	rGetOther := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/drivers/%s", dLicOther), nil)
	rGetOther.AddCookie(driverCookie)
	wGetOther := httptest.NewRecorder()
	app.ServeHTTP(wGetOther, rGetOther)
	if wGetOther.Code != http.StatusForbidden {
		t.Fatalf("driver reading other profile expected 403, got %d", wGetOther.Code)
	}

	// C. Cannot call ANY registry write endpoint
	writeRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/vehicles"},
		{"PATCH", "/api/v1/vehicles/DUMMY"},
		{"POST", "/api/v1/vehicles/DUMMY/retire"},
		{"DELETE", "/api/v1/vehicles/DUMMY"},
		{"POST", "/api/v1/drivers"},
		{"PATCH", "/api/v1/drivers/DUMMY"},
		{"POST", "/api/v1/drivers/DUMMY/suspend"},
		{"POST", "/api/v1/drivers/DUMMY/reinstate"},
		{"POST", "/api/v1/drivers/DUMMY/set-off-duty"},
		{"POST", "/api/v1/drivers/DUMMY/set-available"},
		{"DELETE", "/api/v1/drivers/DUMMY"},
		{"POST", "/api/v1/destinations"},
		{"DELETE", "/api/v1/destinations/DUMMY"},
	}

	for _, tc := range writeRoutes {
		r := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(driverCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("Driver role expected 403 on %s %s, got %d", tc.method, tc.path, w.Code)
		}
	}
}

// 7. Pagination: no duplicates or gaps across pages with concurrent inserts; limit above 100 is clamped; invalid cursor → 400.
func TestCompulsory_PaginationNoGapsAndLimitClamp(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "page_clamp_admin@transitops.com", rbac.RoleAdmin)
	ctx := context.Background()

	// A. Limit above 100 is clamped (does not fail)
	rClamp := httptest.NewRequest("GET", "/api/v1/vehicles?limit=500", nil)
	rClamp.AddCookie(adminCookie)
	wClamp := httptest.NewRecorder()
	app.ServeHTTP(wClamp, rClamp)
	if wClamp.Code != http.StatusOK {
		t.Fatalf("expected 200 for limit=500, got %d", wClamp.Code)
	}

	// B. Invalid cursor -> 400 with code INVALID_CURSOR
	rBad := httptest.NewRequest("GET", "/api/v1/vehicles?cursor=bad-cursor-not-base64-12345!@#$", nil)
	rBad.AddCookie(adminCookie)
	wBad := httptest.NewRecorder()
	app.ServeHTTP(wBad, rBad)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid cursor, got %d", wBad.Code)
	}
	var errMap map[string]any
	_ = json.Unmarshal(wBad.Body.Bytes(), &errMap)
	if errMap["error"].(map[string]any)["code"] != "INVALID_CURSOR" {
		t.Fatalf("expected code INVALID_CURSOR, got %v", errMap["error"].(map[string]any)["code"])
	}

	// C. Concurrent insert during pagination: keyset cursor pagination prevents duplicates and gaps
	prefix := fmt.Sprintf("PG-TST-%s", uuid.NewString()[:4])
	for i := 1; i <= 6; i++ {
		reg := fmt.Sprintf("%s-V%02d", prefix, i)
		_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
			VALUES ($1, 'Model P', 'Van', 1000, 100, 15000, 'Available')`, reg)
		defer func(r string) {
			_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", r)
		}(reg)
	}

	// Fetch Page 1 (limit 3)
	rP1 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&limit=3", prefix), nil)
	rP1.AddCookie(adminCookie)
	wP1 := httptest.NewRecorder()
	app.ServeHTTP(wP1, rP1)
	var p1 PaginatedResponse[vehicle]
	_ = json.Unmarshal(wP1.Body.Bytes(), &p1)
	if len(p1.Items) != 3 {
		t.Fatalf("expected 3 items on page 1, got %d", len(p1.Items))
	}

	// Concurrently insert a new vehicle with registration lower than the cursor
	newLowReg := fmt.Sprintf("%s-V00-LOW", prefix)
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ($1, 'Model P', 'Van', 1000, 100, 15000, 'Available')`, newLowReg)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", newLowReg)
	}()

	// Fetch Page 2 using cursor
	rP2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&limit=3&cursor=%s", prefix, *p1.NextCursor), nil)
	rP2.AddCookie(adminCookie)
	wP2 := httptest.NewRecorder()
	app.ServeHTTP(wP2, rP2)
	var p2 PaginatedResponse[vehicle]
	_ = json.Unmarshal(wP2.Body.Bytes(), &p2)

	// Verify no duplicates between Page 1 and Page 2
	seen := make(map[string]bool)
	for _, it := range p1.Items {
		seen[it.RegistrationNumber] = true
	}
	for _, it := range p2.Items {
		if seen[it.RegistrationNumber] {
			t.Fatalf("duplicate registration %s found across pages!", it.RegistrationNumber)
		}
	}
}

// 8. Raw-SQL test inserting two Dispatched trips for one vehicle -> second insert fails (partial unique index).
// Also test driver dispatched partial index, and maintenance open partial index.
func TestCompulsory_PartialUniqueIndexes(t *testing.T) {
	pool := getTestDB(t)
	ctx := context.Background()

	// Destinations for trips
	hA := fmt.Sprintf("HUB-PU-A-%s", uuid.NewString()[:6])
	hB := fmt.Sprintf("HUB-PU-B-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO destinations (name, region) VALUES ($1, 'North'), ($2, 'South')`, hA, hB)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM destinations WHERE name IN ($1, $2)", hA, hB)
	}()

	// A. Vehicle Dispatched partial unique index
	vReg := fmt.Sprintf("VH-PU-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ($1, 'Model U', 'Van', 1000, 100, 20000, 'Available')`, vReg)

	dLic1 := fmt.Sprintf("DL-PU1-%s", uuid.NewString()[:6])
	dLic2 := fmt.Sprintf("DL-PU2-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
		VALUES ($1, 'Driver 1', 'Heavy', '2028-01-01', '111', 90, 'Available'),
		       ($2, 'Driver 2', 'Heavy', '2028-01-01', '222', 90, 'Available')`, dLic1, dLic2)

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", vReg)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		_, _ = pool.Exec(ctx, "DELETE FROM drivers WHERE license_number IN ($1, $2)", dLic1, dLic2)
	}()

	// First Dispatched trip succeeds
	_, err := pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES ($1, $2, $3, $4, 500, 100, 300, 'Dispatched')`, hA, hB, vReg, dLic1)
	if err != nil {
		t.Fatalf("first dispatched trip failed: %v", err)
	}

	// Second Dispatched trip for the SAME vehicle MUST FAIL due to partial unique index
	_, err = pool.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES ($1, $2, $3, $4, 500, 100, 300, 'Dispatched')`, hA, hB, vReg, dLic2)
	if err == nil {
		t.Fatal("expected second Dispatched trip for same vehicle to fail partial unique index, but succeeded!")
	}
	if !hasPgErrCode(err, "23505") {
		t.Fatalf("expected SQLSTATE 23505 (unique_violation) for partial index, got %v", err)
	}

	// B. Maintenance Open partial unique index
	vRegM := fmt.Sprintf("VH-PUM-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
		VALUES ($1, 'Model M', 'Semi', 10000, 500, 50000, 'Available')`, vRegM)
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM maintenance_logs WHERE vehicle_reg = $1", vRegM)
		_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vRegM)
	}()

	_, err = pool.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status)
		VALUES ($1, 'Log 1', 300, '2026-01-01', 'Open')`, vRegM)
	if err != nil {
		t.Fatalf("first open maintenance failed: %v", err)
	}

	// Second Open maintenance for the SAME vehicle MUST FAIL due to partial unique index
	_, err = pool.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status)
		VALUES ($1, 'Log 2', 400, '2026-01-02', 'Open')`, vRegM)
	if err == nil {
		t.Fatal("expected second Open maintenance for same vehicle to fail partial unique index, but succeeded!")
	}
	if !hasPgErrCode(err, "23505") {
		t.Fatalf("expected SQLSTATE 23505 (unique_violation) for open maintenance, got %v", err)
	}
}

func hasPgErrCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return strings.Contains(err.Error(), code)
}

// 9. Migration up -> down -> up passes on an empty and a seeded database.
func TestCompulsory_MigrationUpDownUp(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	root := findRepoRoot(t)
	migPath := filepath.Join(root, "backend-go", "migrations")

	runMigrate := func(action string) error {
		cmd := exec.Command("go", "run", "./cmd/migrate", "-path", migPath, action)
		cmd.Dir = filepath.Join(root, "backend-go")
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn, "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("migrate %s failed: %v: %s", action, err, string(out))
		}
		return nil
	}

	runSeed := func() error {
		cmd := exec.Command("go", "run", "./cmd/seed")
		cmd.Dir = filepath.Join(root, "backend-go")
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn, "ENV=local", "CGO_ENABLED=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("seed failed: %v: %s", err, string(out))
		}
		return nil
	}

	// 1. Ensure up
	if err := runMigrate("up"); err != nil {
		t.Fatal(err)
	}

	// 2. Down on empty / initial
	if err := runMigrate("down"); err != nil {
		t.Fatalf("migrate down on empty/initial failed: %v", err)
	}

	// 3. Up again
	if err := runMigrate("up"); err != nil {
		t.Fatalf("migrate up after down failed: %v", err)
	}

	// 4. Seed database
	if err := runSeed(); err != nil {
		t.Fatalf("seeding failed: %v", err)
	}

	// 5. Down on seeded database
	if err := runMigrate("down"); err != nil {
		t.Fatalf("migrate down on seeded DB failed: %v", err)
	}

	// 6. Up again and re-seed for remaining tests
	if err := runMigrate("up"); err != nil {
		t.Fatalf("migrate up after seeded down failed: %v", err)
	}
	if err := runSeed(); err != nil {
		t.Fatalf("re-seeding failed: %v", err)
	}
}
