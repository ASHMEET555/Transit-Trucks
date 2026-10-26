package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Helper to seed a unique vehicle for phase 4 testing
func createP4Vehicle(t *testing.T, db *pgxpool.Pool, reg, status, vType string, cap, odo, cost float64, active bool) {
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Model-X', $2, $3, $4, $5, $6, $7, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = $6, type = $2, max_load_capacity = $3, odometer = $4, acquisition_cost = $5, is_active = $7`,
		reg, vType, cap, odo, cost, status, active)
	if err != nil {
		t.Fatalf("failed to insert vehicle %s: %v", reg, err)
	}
}

// Helper to seed a unique driver for phase 4 testing
func createP4Driver(t *testing.T, db *pgxpool.Pool, lic, status, cat string, expiryDate time.Time) {
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'P4 Driver', $2, $3, '+1-555-0100', 95.0, $4, true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = $4, license_category = $2, license_expiry_date = $3, is_active = true`,
		lic, cat, expiryDate.Format("2006-01-02"), status)
	if err != nil {
		t.Fatalf("failed to insert driver %s: %v", lic, err)
	}
}

// ---------------- 1. MAINTENANCE COMPULSORY TESTS ----------------

func TestCompulsory_Maintenance_OpenValidation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.val@transitops.com", rbac.RoleAdmin)
	vBase := fmt.Sprintf("MVAL-V-%s", uuid.NewString()[:6])

	// 1. Retired vehicle -> 409 VEHICLE_RETIRED
	createP4Vehicle(t, db, vBase+"-RET", "Retired", "Van", 5000, 1000, 20000, false)
	bodyRet, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-RET",
		"description": "Oil service",
		"cost":        100.0,
	})
	reqRet := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyRet))
	reqRet.Header.Set("Content-Type", "application/json")
	reqRet.Header.Set("Idempotency-Key", uuid.NewString())
	reqRet.AddCookie(adminCookie)
	wRet := httptest.NewRecorder()
	app.ServeHTTP(wRet, reqRet)

	if wRet.Code != http.StatusConflict {
		t.Fatalf("expected 409 for retired vehicle, got %d: %s", wRet.Code, wRet.Body.String())
	}
	var resRet map[string]any
	_ = json.Unmarshal(wRet.Body.Bytes(), &resRet)
	if resRet["error"].(map[string]any)["code"] != "VEHICLE_RETIRED" {
		t.Errorf("expected code VEHICLE_RETIRED, got %v", resRet["error"].(map[string]any)["code"])
	}

	// 2. On Trip vehicle -> 409 VEHICLE_NOT_AVAILABLE
	createP4Vehicle(t, db, vBase+"-TRIP", "On Trip", "Van", 5000, 1000, 20000, true)
	bodyTrip, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-TRIP",
		"description": "Oil service",
		"cost":        100.0,
	})
	reqTrip := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyTrip))
	reqTrip.Header.Set("Content-Type", "application/json")
	reqTrip.Header.Set("Idempotency-Key", uuid.NewString())
	reqTrip.AddCookie(adminCookie)
	wTrip := httptest.NewRecorder()
	app.ServeHTTP(wTrip, reqTrip)

	if wTrip.Code != http.StatusConflict {
		t.Fatalf("expected 409 for On Trip vehicle, got %d", wTrip.Code)
	}

	// 3. In Shop vehicle -> 409 VEHICLE_NOT_AVAILABLE
	createP4Vehicle(t, db, vBase+"-SHOP", "In Shop", "Van", 5000, 1000, 20000, true)
	bodyShop, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-SHOP",
		"description": "Oil service",
		"cost":        100.0,
	})
	reqShop := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyShop))
	reqShop.Header.Set("Content-Type", "application/json")
	reqShop.Header.Set("Idempotency-Key", uuid.NewString())
	reqShop.AddCookie(adminCookie)
	wShop := httptest.NewRecorder()
	app.ServeHTTP(wShop, reqShop)

	if wShop.Code != http.StatusConflict {
		t.Fatalf("expected 409 for In Shop vehicle, got %d", wShop.Code)
	}

	// 4. Negative cost -> 422
	createP4Vehicle(t, db, vBase+"-AVAIL", "Available", "Van", 5000, 1000, 20000, true)
	bodyNeg, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-AVAIL",
		"description": "Oil service",
		"cost":        -50.0,
	})
	reqNeg := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyNeg))
	reqNeg.Header.Set("Content-Type", "application/json")
	reqNeg.Header.Set("Idempotency-Key", uuid.NewString())
	reqNeg.AddCookie(adminCookie)
	wNeg := httptest.NewRecorder()
	app.ServeHTTP(wNeg, reqNeg)

	if wNeg.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for negative cost, got %d", wNeg.Code)
	}

	// 5. Description over 500 chars -> 422
	bodyLong, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-AVAIL",
		"description": strings.Repeat("A", 501),
		"cost":        50.0,
	})
	reqLong := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyLong))
	reqLong.Header.Set("Content-Type", "application/json")
	reqLong.Header.Set("Idempotency-Key", uuid.NewString())
	reqLong.AddCookie(adminCookie)
	wLong := httptest.NewRecorder()
	app.ServeHTTP(wLong, reqLong)

	if wLong.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for description > 500 chars, got %d", wLong.Code)
	}

	// 6. Start date > 1 day in future -> 422
	futureDate := time.Now().UTC().AddDate(0, 0, 3).Format("2006-01-02")
	bodyFut, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-AVAIL",
		"description": "Future checkup",
		"cost":        50.0,
		"start_date":  futureDate,
	})
	reqFut := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyFut))
	reqFut.Header.Set("Content-Type", "application/json")
	reqFut.Header.Set("Idempotency-Key", uuid.NewString())
	reqFut.AddCookie(adminCookie)
	wFut := httptest.NewRecorder()
	app.ServeHTTP(wFut, reqFut)

	if wFut.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for start date > 1 day in future, got %d", wFut.Code)
	}

	// 7. Missing Idempotency-Key -> 400 IDEMPOTENCY_KEY_REQUIRED
	bodyNoKey, _ := json.Marshal(map[string]any{
		"vehicle_reg": vBase + "-AVAIL",
		"description": "Oil service",
		"cost":        50.0,
	})
	reqNoKey := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(bodyNoKey))
	reqNoKey.Header.Set("Content-Type", "application/json")
	reqNoKey.AddCookie(adminCookie)
	wNoKey := httptest.NewRecorder()
	app.ServeHTTP(wNoKey, reqNoKey)

	if wNoKey.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing idempotency key, got %d", wNoKey.Code)
	}
}

func TestCompulsory_Maintenance_CloseValidation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.closeval@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("MCLOSE-V-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)

	// Open a log
	var logID int64
	today := time.Now().UTC().Format("2006-01-02")
	err := db.QueryRow(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status, created_at, updated_at)
		VALUES ($1, 'Brake fix', 200, $2, 'Open', now(), now()) RETURNING id`, vReg, today).Scan(&logID)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(ctx, `UPDATE vehicles SET status = 'In Shop' WHERE registration_number = $1`, vReg)

	// 1. end_date before start_date -> 422
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	bodyPast, _ := json.Marshal(map[string]any{
		"end_date": yesterday,
	})
	reqPast := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(bodyPast))
	reqPast.Header.Set("Content-Type", "application/json")
	reqPast.Header.Set("Idempotency-Key", uuid.NewString())
	reqPast.AddCookie(adminCookie)
	wPast := httptest.NewRecorder()
	app.ServeHTTP(wPast, reqPast)

	if wPast.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for end_date < start_date, got %d: %s", wPast.Code, wPast.Body.String())
	}

	// 2. Successful close -> 200 OK and vehicle status becomes Available
	bodyOk, _ := json.Marshal(map[string]any{
		"end_date": today,
	})
	closeKey := uuid.NewString()
	reqOk := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(bodyOk))
	reqOk.Header.Set("Content-Type", "application/json")
	reqOk.Header.Set("Idempotency-Key", closeKey)
	reqOk.AddCookie(adminCookie)
	wOk := httptest.NewRecorder()
	app.ServeHTTP(wOk, reqOk)

	if wOk.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for close, got %d: %s", wOk.Code, wOk.Body.String())
	}

	var vStatus string
	_ = db.QueryRow(ctx, `SELECT status FROM vehicles WHERE registration_number = $1`, vReg).Scan(&vStatus)
	if vStatus != "Available" {
		t.Fatalf("expected vehicle status Available after closing, got '%s'", vStatus)
	}

	// 3. Close already Closed log -> 409 INVALID_TRANSITION (with a different idempotency key)
	reqAlreadyClosed := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(bodyOk))
	reqAlreadyClosed.Header.Set("Content-Type", "application/json")
	reqAlreadyClosed.Header.Set("Idempotency-Key", uuid.NewString())
	reqAlreadyClosed.AddCookie(adminCookie)
	wAlreadyClosed := httptest.NewRecorder()
	app.ServeHTTP(wAlreadyClosed, reqAlreadyClosed)

	if wAlreadyClosed.Code != http.StatusConflict {
		t.Fatalf("expected 409 for already closed log, got %d: %s", wAlreadyClosed.Code, wAlreadyClosed.Body.String())
	}

	// 4. Same key replay -> returns 200 with X-Idempotent-Replay
	reqReplay := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(bodyOk))
	reqReplay.Header.Set("Content-Type", "application/json")
	reqReplay.Header.Set("Idempotency-Key", closeKey)
	reqReplay.AddCookie(adminCookie)
	wReplay := httptest.NewRecorder()
	app.ServeHTTP(wReplay, reqReplay)

	if wReplay.Code != http.StatusOK {
		t.Fatalf("expected 200 replay for same key, got %d", wReplay.Code)
	}
	if wReplay.Header().Get("X-Idempotent-Replay") != "true" {
		t.Errorf("expected X-Idempotent-Replay header")
	}
}

func TestCompulsory_Maintenance_RetireGuard(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.retguard@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("RETG-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("RETG-D-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	// 1. Create a Draft trip
	var tripID int64
	err := db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 2000, 300, 1000, 'Draft', now(), now()) RETURNING id`, vReg, dLic).Scan(&tripID)
	if err != nil {
		t.Fatal(err)
	}

	// Attempt retire -> MUST FAIL with 409 CONFLICT because Draft trip exists
	reqRet := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/vehicles/%s/retire", vReg), nil)
	reqRet.AddCookie(adminCookie)
	wRet := httptest.NewRecorder()
	app.ServeHTTP(wRet, reqRet)

	if wRet.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict retiring vehicle with Draft trip, got %d: %s", wRet.Code, wRet.Body.String())
	}

	// Delete trip, then retire -> MUST SUCCEED with 200
	_, _ = db.Exec(ctx, `DELETE FROM trips WHERE id = $1`, tripID)
	reqRet2 := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/vehicles/%s/retire", vReg), nil)
	reqRet2.AddCookie(adminCookie)
	wRet2 := httptest.NewRecorder()
	app.ServeHTTP(wRet2, reqRet2)

	if wRet2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK retiring vehicle with no active trips, got %d: %s", wRet2.Code, wRet2.Body.String())
	}
}

func TestCompulsory_Maintenance_ConcurrentOpen(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.concurrent@transitops.com", rbac.RoleAdmin)

	rounds := 50 // 50 rounds
	for round := 1; round <= rounds; round++ {
		vReg := fmt.Sprintf("MCONC-V-%02d-%s", round, uuid.NewString()[:4])
		createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)

		workers := 50
		var wg sync.WaitGroup
		successCount := 0
		conflictCount := 0
		var mu sync.Mutex

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]any{
					"vehicle_reg": vReg,
					"description": fmt.Sprintf("Concurrent open round %d worker %d", round, idx),
					"cost":        100.0,
				})
				req := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", uuid.NewString())
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.2.%d.%d", round%250, idx%250))
				req.AddCookie(adminCookie)
				w := httptest.NewRecorder()
				app.ServeHTTP(w, req)

				mu.Lock()
				defer mu.Unlock()
				if w.Code == http.StatusCreated {
					successCount++
				} else if w.Code == http.StatusConflict {
					conflictCount++
				}
			}(i)
		}

		wg.Wait()

		if successCount != 1 {
			t.Fatalf("Round %d: expected exactly 1 successful open, got %d (conflicts: %d)", round, successCount, conflictCount)
		}

		// Verify database count of Open logs for this vehicle <= 1
		var openCount int
		_ = db.QueryRow(ctx, `SELECT COUNT(*) FROM maintenance_logs WHERE vehicle_reg = $1 AND status = 'Open'`, vReg).Scan(&openCount)
		if openCount != 1 {
			t.Fatalf("Round %d: expected exactly 1 open log in DB, got %d", round, openCount)
		}
	}
}

func TestCompulsory_Maintenance_ConcurrentOpenAndDispatch(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.dispatch@transitops.com", rbac.RoleAdmin)

	rounds := 20
	for round := 1; round <= rounds; round++ {
		vReg := fmt.Sprintf("M-DISP-V-%02d-%s", round, uuid.NewString()[:4])
		dLic := fmt.Sprintf("M-DISP-D-%02d-%s", round, uuid.NewString()[:4])
		createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
		createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

		var wg sync.WaitGroup
		maintSuccess := 0
		dispatchSuccess := 0
		var mu sync.Mutex

		// 10 workers try open maintenance
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]any{
					"vehicle_reg": vReg,
					"description": "Maintenance race",
					"cost":        100.0,
				})
				req := httptest.NewRequest("POST", "/api/v1/maintenance", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", uuid.NewString())
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.3.%d.%d", round%250, idx%250))
				req.AddCookie(adminCookie)
				w := httptest.NewRecorder()
				app.ServeHTTP(w, req)

				mu.Lock()
				defer mu.Unlock()
				if w.Code == http.StatusCreated {
					maintSuccess++
				}
			}(i)
		}

		// 10 workers try dispatch
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]any{
					"source_name":      "New York Central Hub",
					"destination_name": "Chicago Logistics Hub",
					"vehicle_reg":      vReg,
					"driver_license":   dLic,
					"cargo_weight":     1000,
					"planned_distance": 300,
					"revenue":          500,
					"dispatch":         true,
				})
				req := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", uuid.NewString())
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.4.%d.%d", round%250, idx%250))
				req.AddCookie(adminCookie)
				w := httptest.NewRecorder()
				app.ServeHTTP(w, req)

				mu.Lock()
				defer mu.Unlock()
				if w.Code == http.StatusCreated {
					dispatchSuccess++
				}
			}(i)
		}

		wg.Wait()

		// Exactly one of the two domains must have succeeded!
		totalSuccess := maintSuccess + dispatchSuccess
		if totalSuccess != 1 {
			t.Fatalf("Round %d: expected exactly 1 winner, got maint=%d, dispatch=%d", round, maintSuccess, dispatchSuccess)
		}

		// Vehicle status must be In Shop XOR On Trip, NEVER both
		var vStatus string
		_ = db.QueryRow(ctx, `SELECT status FROM vehicles WHERE registration_number = $1`, vReg).Scan(&vStatus)
		if maintSuccess == 1 && vStatus != "In Shop" {
			t.Fatalf("maint won but vehicle status is '%s'", vStatus)
		}
		if dispatchSuccess == 1 && vStatus != "On Trip" {
			t.Fatalf("dispatch won but vehicle status is '%s'", vStatus)
		}
	}
}

func TestCompulsory_Maintenance_RoleMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.rm@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.maint.rm@transitops.com", rbac.RoleFleetManager)
	safetyCookie := createTestUserAndLogin(t, app, "safety.maint.rm@transitops.com", rbac.RoleSafetyOfficer)
	finCookie := createTestUserAndLogin(t, app, "fin.maint.rm@transitops.com", rbac.RoleFinancialAnalyst)
	driverCookie := createTestUserAndLogin(t, app, "driver.maint.rm@transitops.com", rbac.RoleDriver)

	vReg := fmt.Sprintf("MRM-V-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)

	// GET /maintenance: Admin=200, Fleet=200, Safety=200, Fin=403, Driver=403
	checkGet := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/maintenance", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /maintenance expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkGet(adminCookie, http.StatusOK)
	checkGet(fleetCookie, http.StatusOK)
	checkGet(safetyCookie, http.StatusOK)
	checkGet(finCookie, http.StatusForbidden)
	checkGet(driverCookie, http.StatusForbidden)
}
