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

// ---------------- 2. FINANCE COMPULSORY TESTS ----------------

func TestCompulsory_Finance_ExpenseValidation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.fin.val@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("FIN-VAL-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("FIN-VAL-D-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	createTrip := func(status string) int64 {
		var id int64
		err := db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, created_at, updated_at)
			VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 200, 800, $3, 1000, 1200, now(), now()) RETURNING id`,
			vReg, dLic, status).Scan(&id)
		if err != nil {
			t.Fatalf("failed to insert trip: %v", err)
		}
		return id
	}

	draftTripID := createTrip("Draft")
	cancelledTripID := createTrip("Cancelled")
	dispatchedTripID := createTrip("Dispatched")
	completedTripID := createTrip("Completed")

	postExpense := func(body map[string]any) *httptest.ResponseRecorder {
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/expenses", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		return w
	}

	// 1. Expense on draft trip -> 422 EXPENSE_ON_INACTIVE_TRIP
	wDraft := postExpense(map[string]any{"trip_id": draftTripID, "type": "Toll", "amount": 25.0})
	if wDraft.Code != http.StatusUnprocessableEntity || !strings.Contains(wDraft.Body.String(), "EXPENSE_ON_INACTIVE_TRIP") {
		t.Fatalf("expected 422 EXPENSE_ON_INACTIVE_TRIP on draft trip, got %d: %s", wDraft.Code, wDraft.Body.String())
	}

	// 2. Expense on cancelled trip -> 422 EXPENSE_ON_INACTIVE_TRIP
	wCan := postExpense(map[string]any{"trip_id": cancelledTripID, "type": "Toll", "amount": 25.0})
	if wCan.Code != http.StatusUnprocessableEntity || !strings.Contains(wCan.Body.String(), "EXPENSE_ON_INACTIVE_TRIP") {
		t.Fatalf("expected 422 EXPENSE_ON_INACTIVE_TRIP on cancelled trip, got %d: %s", wCan.Code, wCan.Body.String())
	}

	// 3. Expense Fuel without liters -> 422
	wNoLit := postExpense(map[string]any{"trip_id": dispatchedTripID, "type": "Fuel", "amount": 100.0})
	if wNoLit.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for fuel without liters, got %d", wNoLit.Code)
	}

	// 4. Expense Toll with liters -> 422
	wTollLit := postExpense(map[string]any{"trip_id": dispatchedTripID, "type": "Toll", "amount": 30.0, "liters": 10.0})
	if wTollLit.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for toll with liters, got %d", wTollLit.Code)
	}

	// 5. Expense with negative amount -> 422
	wNeg := postExpense(map[string]any{"trip_id": dispatchedTripID, "type": "Other", "amount": -10.0})
	if wNeg.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for negative expense amount, got %d", wNeg.Code)
	}

	// 6. Expense future date (> 1 day in future) -> 422
	futureDate := time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")
	wFuture := postExpense(map[string]any{"trip_id": dispatchedTripID, "type": "Other", "amount": 50.0, "date": futureDate})
	if wFuture.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for future date, got %d", wFuture.Code)
	}

	// 7. Expense with mismatched vehicle_reg -> 422
	wMismatch := postExpense(map[string]any{"trip_id": dispatchedTripID, "vehicle_reg": "NON-EXISTENT", "type": "Other", "amount": 50.0})
	if wMismatch.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for mismatched vehicle_reg, got %d", wMismatch.Code)
	}

	// 8. Expense on dispatched trip -> 201 Created
	wDisp := postExpense(map[string]any{"trip_id": dispatchedTripID, "type": "Toll", "amount": 40.0})
	if wDisp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for toll on dispatched trip, got %d: %s", wDisp.Code, wDisp.Body.String())
	}

	// 9. Expense on completed trip with Fuel & liters -> 201 Created
	wComp := postExpense(map[string]any{"trip_id": completedTripID, "type": "Fuel", "amount": 120.0, "liters": 45.0})
	if wComp.Code != http.StatusCreated {
		t.Fatalf("expected 201 for fuel on completed trip, got %d: %s", wComp.Code, wComp.Body.String())
	}

	// 10. Standalone expense by vehicle_reg without trip -> 201 Created
	wStandalone := postExpense(map[string]any{"vehicle_reg": vReg, "type": "Maintenance", "amount": 250.0})
	if wStandalone.Code != http.StatusCreated {
		t.Fatalf("expected 201 for standalone expense on vehicle, got %d: %s", wStandalone.Code, wStandalone.Body.String())
	}
}

func TestCompulsory_Finance_SummaryMath(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.fin.math@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("FIN-MTH-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("FIN-MTH-D-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Semi", 20000, 1000, 80000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	// Trip 1: Completed, odo 1000 -> 1300 = 300 km
	var t1ID int64
	err := db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 5000, 300, 1500, 'Completed', 1000, 1300, now(), now(), now()) RETURNING id`,
		vReg, dLic).Scan(&t1ID)
	if err != nil {
		t.Fatalf("failed to insert trip 1: %v", err)
	}

	// Trip 2: Completed, odo 1300 -> 1500 = 200 km (total distance = 500 km)
	var t2ID int64
	err = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		VALUES ('Chicago Logistics Hub', 'Dallas Gateway Hub', $1, $2, 6000, 200, 1200, 'Completed', 1300, 1500, now(), now(), now()) RETURNING id`,
		vReg, dLic).Scan(&t2ID)
	if err != nil {
		t.Fatalf("failed to insert trip 2: %v", err)
	}

	// Expense 1: Fuel = 150.00
	_, _ = db.Exec(ctx, `INSERT INTO expenses (trip_id, vehicle_reg, type, liters, cost, date, created_at, updated_at)
		VALUES ($1, $2, 'Fuel', 50.0, 150.00, CURRENT_DATE, now(), now())`, t1ID, vReg)

	// Expense 2: Toll (Other) = 50.00
	_, _ = db.Exec(ctx, `INSERT INTO expenses (trip_id, vehicle_reg, type, cost, date, created_at, updated_at)
		VALUES ($1, $2, 'Toll', 50.00, CURRENT_DATE, now(), now())`, t1ID, vReg)

	// Expense 3: Maintenance Expense = 100.00
	_, _ = db.Exec(ctx, `INSERT INTO expenses (vehicle_reg, type, cost, date, created_at, updated_at)
		VALUES ($1, 'Maintenance', 100.00, CURRENT_DATE, now(), now())`, vReg)

	// Maintenance Log: Closed = 200.00
	_, _ = db.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, 'Brake overhaul', 200.00, CURRENT_DATE, CURRENT_DATE, 'Closed', now(), now())`, vReg)

	// Query GET /api/v1/finance/summary?vehicle_reg=vReg
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/finance/summary?vehicle_reg=%s", vReg), nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for finance summary, got %d: %s", w.Code, w.Body.String())
	}

	var resp FinanceSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Vehicles) != 1 {
		t.Fatalf("expected 1 vehicle in summary, got %d", len(resp.Vehicles))
	}

	v := resp.Vehicles[0]
	// Fuel: 150.00
	if v.FuelExpenses != 150.00 {
		t.Errorf("expected FuelExpenses 150.00, got %f", v.FuelExpenses)
	}
	// Maintenance expenses: 100.00
	if v.MaintenanceExpenses != 100.00 {
		t.Errorf("expected MaintenanceExpenses 100.00, got %f", v.MaintenanceExpenses)
	}
	// Other expenses: 50.00
	if v.OtherExpenses != 50.00 {
		t.Errorf("expected OtherExpenses 50.00, got %f", v.OtherExpenses)
	}
	// Closed maintenance logs: 200.00
	if v.ClosedMaintenanceCost != 200.00 {
		t.Errorf("expected ClosedMaintenanceCost 200.00, got %f", v.ClosedMaintenanceCost)
	}
	// Total distance: 500.00
	if v.TotalDistance != 500.00 {
		t.Errorf("expected TotalDistance 500.00, got %f", v.TotalDistance)
	}
	// Total cost = 150 + 100 + 50 + 200 = 500.00
	// Cost per km = 500 / 500 = 1.00
	if v.CostPerKm == nil || *v.CostPerKm != 1.00 {
		t.Errorf("expected CostPerKm 1.00, got %v", v.CostPerKm)
	}
}

func TestCompulsory_Finance_RoleMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.fin.rm@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.fin.rm@transitops.com", rbac.RoleFleetManager)
	finCookie := createTestUserAndLogin(t, app, "fin.fin.rm@transitops.com", rbac.RoleFinancialAnalyst)
	safetyCookie := createTestUserAndLogin(t, app, "safety.fin.rm@transitops.com", rbac.RoleSafetyOfficer)
	driverCookie := createTestUserAndLogin(t, app, "driver.fin.rm@transitops.com", rbac.RoleDriver)

	// GET /api/v1/expenses: Admin=200, Fleet=200, Fin=200, Safety=403, Driver=403
	checkExpensesGet := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/expenses", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /expenses expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkExpensesGet(adminCookie, http.StatusOK)
	checkExpensesGet(fleetCookie, http.StatusOK)
	checkExpensesGet(finCookie, http.StatusOK)
	checkExpensesGet(safetyCookie, http.StatusForbidden)
	checkExpensesGet(driverCookie, http.StatusForbidden)

	// GET /api/v1/finance/summary: Admin=200, Fleet=200, Fin=200, Safety=403, Driver=403
	checkSummaryGet := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/finance/summary", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /finance/summary expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkSummaryGet(adminCookie, http.StatusOK)
	checkSummaryGet(fleetCookie, http.StatusOK)
	checkSummaryGet(finCookie, http.StatusOK)
	checkSummaryGet(safetyCookie, http.StatusForbidden)
	checkSummaryGet(driverCookie, http.StatusForbidden)
}

func TestCompulsory_Finance_FuelMigration(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Verify columns fuel_liters and fuel_cost are absent from trips table
	var colCount int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns 
		WHERE table_name = 'trips' AND column_name IN ('fuel_liters', 'fuel_cost')`).Scan(&colCount)
	if err != nil {
		t.Fatalf("failed to query columns: %v", err)
	}
	if colCount != 0 {
		t.Fatalf("expected trips.fuel_liters and trips.fuel_cost to be dropped in migration 000004, found %d", colCount)
	}

	// Verify completed_at exists on trips
	var completedAtExists bool
	err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'trips' AND column_name = 'completed_at')`).Scan(&completedAtExists)
	if err != nil || !completedAtExists {
		t.Fatalf("expected completed_at column to exist on trips table")
	}

	// Verify expenses table has vehicle_reg
	var vehicleRegExists bool
	err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'expenses' AND column_name = 'vehicle_reg')`).Scan(&vehicleRegExists)
	if err != nil || !vehicleRegExists {
		t.Fatalf("expected vehicle_reg column to exist on expenses table")
	}
}

// ---------------- 3. DASHBOARD COMPULSORY TESTS ----------------

func TestCompulsory_Dashboard_KPICorrectnessFixture(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.fix@transitops.com", rbac.RoleAdmin)

	v1 := fmt.Sprintf("KPI-V1-%s", uuid.NewString()[:6])
	v2 := fmt.Sprintf("KPI-V2-%s", uuid.NewString()[:6])
	v3 := fmt.Sprintf("KPI-V3-%s", uuid.NewString()[:6])
	v4 := fmt.Sprintf("KPI-V4-%s", uuid.NewString()[:6])

	d1 := fmt.Sprintf("KPI-D1-%s", uuid.NewString()[:6])
	d2 := fmt.Sprintf("KPI-D2-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, v1, "Available", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, v2, "On Trip", "Box Truck", 8000, 2000, 40000, true)
	createP4Vehicle(t, db, v3, "In Shop", "Flatbed", 12000, 3000, 60000, true)
	createP4Vehicle(t, db, v4, "Retired", "Semi", 20000, 4000, 80000, true)

	createP4Driver(t, db, d1, "Available", "Light", time.Now().AddDate(1, 0, 0))
	createP4Driver(t, db, d2, "On Trip", "Heavy", time.Now().AddDate(1, 0, 0))

	// T1: Completed on V2, D2, odo 1000->1150 = 150 km, rev 1000.00
	var t1ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 2000, 100, 1000, 'Completed', 1000, 1150, now(), now(), now()) RETURNING id`, v2, d2).Scan(&t1ID)

	// T2: Completed on V1, D1, odo 1000->1250 = 250 km, rev 2000.00
	var t2ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		VALUES ('Chicago Logistics Hub', 'Dallas Gateway Hub', $1, $2, 3000, 250, 2000, 'Completed', 1000, 1250, now(), now(), now()) RETURNING id`, v1, d1).Scan(&t2ID)

	// T3: Cancelled on V1, D1, rev 5000.00 -> MUST NOT COUNT in revenue or distance
	var t3ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Atlanta Distribution Hub', $1, $2, 1000, 300, 5000, 'Cancelled', now(), now()) RETURNING id`, v1, d1).Scan(&t3ID)

	// T4: Dispatched on V2, D2, rev 800.00 -> Active trip
	var t4ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, created_at, updated_at)
		VALUES ('Atlanta Distribution Hub', 'Miami Port Hub', $1, $2, 1500, 120, 800, 'Dispatched', 2000, now(), now()) RETURNING id`, v2, d2).Scan(&t4ID)

	// T5: Draft on V1, D1, rev 400.00 -> Pending trip
	var t5ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('Miami Port Hub', 'Denver Intermodal Hub', $1, $2, 800, 80, 400, 'Draft', now(), now()) RETURNING id`, v1, d1).Scan(&t5ID)

	// T6: Another Draft on V3, rev 300.00 -> Pending trip
	var t6ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('Denver Intermodal Hub', 'Chicago Logistics Hub', $1, 500, 50, 300, 'Draft', now(), now()) RETURNING id`, v3).Scan(&t6ID)

	// Expenses
	_, _ = db.Exec(ctx, `INSERT INTO expenses (trip_id, vehicle_reg, type, liters, cost, date, created_at, updated_at) VALUES ($1, $2, 'Fuel', 50.0, 100.00, CURRENT_DATE, now(), now())`, t1ID, v2)
	_, _ = db.Exec(ctx, `INSERT INTO expenses (trip_id, vehicle_reg, type, liters, cost, date, created_at, updated_at) VALUES ($1, $2, 'Fuel', 50.0, 150.00, CURRENT_DATE, now(), now())`, t2ID, v1)
	_, _ = db.Exec(ctx, `INSERT INTO expenses (vehicle_reg, type, cost, date, created_at, updated_at) VALUES ($1, 'Toll', 50.00, CURRENT_DATE, now(), now())`, v1)
	_, _ = db.Exec(ctx, `INSERT INTO expenses (vehicle_reg, type, cost, date, created_at, updated_at) VALUES ($1, 'Maintenance', 300.00, CURRENT_DATE, now(), now())`, v3)

	// Maintenance logs
	_, _ = db.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, end_date, status, created_at, updated_at) VALUES ($1, 'Closed service', 500.00, CURRENT_DATE, CURRENT_DATE, 'Closed', now(), now())`, v3)
	_, _ = db.Exec(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status, created_at, updated_at) VALUES ($1, 'Open service', 700.00, CURRENT_DATE, 'Open', now(), now())`, v3)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dashboard KPIs, got %d: %s", w.Code, w.Body.String())
	}

	var res DashboardKPIsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 1. Assert fleet counts
	if res.ActiveVehicles < 1 {
		t.Errorf("expected at least 1 active vehicle, got %d", res.ActiveVehicles)
	}
	if res.InShopVehicles < 1 {
		t.Errorf("expected at least 1 in-shop vehicle, got %d", res.InShopVehicles)
	}
	if res.RetiredVehicles < 1 {
		t.Errorf("expected at least 1 retired vehicle, got %d", res.RetiredVehicles)
	}

	// 2. Assert trips
	if res.ActiveTrips < 1 {
		t.Errorf("expected at least 1 active trip, got %d", res.ActiveTrips)
	}
	if res.PendingTrips < 2 {
		t.Errorf("expected at least 2 pending trips, got %d", res.PendingTrips)
	}
	if res.CompletedTrips < 2 {
		t.Errorf("expected at least 2 completed trips, got %d", res.CompletedTrips)
	}

	// 3. Find our specific vehicles in the array
	var kpiV1, kpiV2, kpiV3 *VehicleKPI
	for i := range res.Vehicles {
		if res.Vehicles[i].RegistrationNumber == v1 {
			kpiV1 = &res.Vehicles[i]
		}
		if res.Vehicles[i].RegistrationNumber == v2 {
			kpiV2 = &res.Vehicles[i]
		}
		if res.Vehicles[i].RegistrationNumber == v3 {
			kpiV3 = &res.Vehicles[i]
		}
	}

	if kpiV1 == nil || kpiV2 == nil || kpiV3 == nil {
		t.Fatalf("failed to find created vehicles in response: v1=%v, v2=%v, v3=%v", kpiV1, kpiV2, kpiV3)
	}

	// V1: revenue=2000.00, distance=250.00, fuel_cost=150.00, other=50.00, op=200.00
	if kpiV1.Revenue != 2000.00 {
		t.Errorf("V1 revenue: expected 2000.00, got %f", kpiV1.Revenue)
	}
	if kpiV1.Distance != 250.00 {
		t.Errorf("V1 distance: expected 250.00, got %f", kpiV1.Distance)
	}
	if kpiV1.OperationalCost != 200.00 {
		t.Errorf("V1 operational_cost: expected 200.00, got %f", kpiV1.OperationalCost)
	}

	// V2: revenue=1000.00, distance=150.00, fuel_cost=100.00, op=100.00
	if kpiV2.Revenue != 1000.00 {
		t.Errorf("V2 revenue: expected 1000.00, got %f", kpiV2.Revenue)
	}
	if kpiV2.Distance != 150.00 {
		t.Errorf("V2 distance: expected 150.00, got %f", kpiV2.Distance)
	}
	if kpiV2.OperationalCost != 100.00 {
		t.Errorf("V2 operational_cost: expected 100.00, got %f", kpiV2.OperationalCost)
	}

	// V3: maintenance=300 (expense) + 500 (closed log) = 800.00, op=800.00
	if kpiV3.MaintenanceCost != 800.00 {
		t.Errorf("V3 maintenance_cost: expected 800.00, got %f", kpiV3.MaintenanceCost)
	}
	if kpiV3.OperationalCost != 800.00 {
		t.Errorf("V3 operational_cost: expected 800.00, got %f", kpiV3.OperationalCost)
	}
}

func TestCompulsory_Dashboard_Benchmark10kTrips(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.bench@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("BENCH-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("BENCH-D-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Van", 10000, 1000, 30000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(2, 0, 0))

	// Fast bulk insert of 10,000 completed trips
	_, err := db.Exec(ctx, `
		INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		SELECT
			'New York Central Hub',
			'Chicago Logistics Hub',
			$1,
			$2,
			1000 + (g % 500),
			100 + (g % 100),
			500 + (g % 200),
			'Completed',
			1000,
			1150,
			now(),
			now(),
			now()
		FROM generate_series(1, 10000) AS g`, vReg, dLic)
	if err != nil {
		t.Fatalf("failed bulk insert of 10,000 trips: %v", err)
	}

	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, vReg)
		_, _ = db.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, dLic)
	}()

	// Measure GET /api/v1/dashboard/kpis duration
	start := time.Now()
	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	t.Logf("10k trips GET /dashboard/kpis completed in %v", elapsed)
	if elapsed > 100*time.Millisecond {
		t.Errorf("benchmark exceeded target: %v > 100ms", elapsed)
	}
}

func TestCompulsory_Dashboard_RoleMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.dash.rm@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.dash.rm@transitops.com", rbac.RoleFleetManager)
	finCookie := createTestUserAndLogin(t, app, "fin.dash.rm@transitops.com", rbac.RoleFinancialAnalyst)
	safetyCookie := createTestUserAndLogin(t, app, "safety.dash.rm@transitops.com", rbac.RoleSafetyOfficer)

	// Driver user with driver license
	driverLic := fmt.Sprintf("RM-DRV-%s", uuid.NewString()[:6])
	createP4Driver(t, db, driverLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))
	driverEmail := fmt.Sprintf("driver.dash.rm.%s@transitops.com", uuid.NewString()[:6])
	driverCookie := createTestUserAndLogin(t, app, driverEmail, rbac.RoleDriver)
	_, _ = db.Exec(ctx, `UPDATE users SET driver_license = $1 WHERE email = $2`, driverLic, driverEmail)

	// GET /dashboard/kpis: Driver=403, Admin=200, Fleet=200, Fin=200, Safety=200
	checkKpis := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /dashboard/kpis expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkKpis(driverCookie, http.StatusForbidden)
	checkKpis(adminCookie, http.StatusOK)
	checkKpis(fleetCookie, http.StatusOK)
	checkKpis(finCookie, http.StatusOK)
	checkKpis(safetyCookie, http.StatusOK)

	// GET /dashboard/me: Driver=200, Admin=403, Fleet=403, Fin=403, Safety=403
	checkMe := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/dashboard/me", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /dashboard/me expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkMe(driverCookie, http.StatusOK)
	checkMe(adminCookie, http.StatusForbidden)
	checkMe(fleetCookie, http.StatusForbidden)
	checkMe(finCookie, http.StatusForbidden)
	checkMe(safetyCookie, http.StatusForbidden)
}

func TestCompulsory_Dashboard_DriverPIIScrubbed(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.pii.dash@transitops.com", rbac.RoleAdmin)

	secretDriverName := "SuperSecretDriverNameXYZ"
	secretLicense := fmt.Sprintf("LIC-PII-%s", uuid.NewString()[:6])
	vReg := fmt.Sprintf("PII-V-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
	_, _ = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Heavy', CURRENT_DATE + INTERVAL '1 year', '+1-555-8888', 95.0, 'Available', true, now(), now())`,
		secretLicense, secretDriverName)

	_, _ = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, completed_at, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 200, 500, 'Completed', now(), now(), now())`,
		vReg, secretLicense)

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, secretDriverName) {
		t.Fatalf("driver name leaked in /dashboard/kpis response: %s", secretDriverName)
	}
	if strings.Contains(bodyStr, secretLicense) {
		t.Fatalf("driver license leaked in /dashboard/kpis response: %s", secretLicense)
	}
}

// ---------------- 4. CSV EXPORT COMPULSORY TESTS ----------------

func TestCompulsory_Dashboard_ExportCSV_StreamAndInjection(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.export.csv@transitops.com", rbac.RoleAdmin)
	driverCookie := createTestUserAndLogin(t, app, "driver.export.csv@transitops.com", rbac.RoleDriver)
	safetyCookie := createTestUserAndLogin(t, app, "safety.export.csv@transitops.com", rbac.RoleSafetyOfficer)

	// Driver and safety officer get 403
	reqDriver := httptest.NewRequest("GET", "/api/v1/dashboard/export.csv", nil)
	reqDriver.AddCookie(driverCookie)
	wDriver := httptest.NewRecorder()
	app.ServeHTTP(wDriver, reqDriver)
	if wDriver.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for driver on export.csv, got %d", wDriver.Code)
	}

	reqSafety := httptest.NewRequest("GET", "/api/v1/dashboard/export.csv", nil)
	reqSafety.AddCookie(safetyCookie)
	wSafety := httptest.NewRecorder()
	app.ServeHTTP(wSafety, reqSafety)
	if wSafety.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for safety officer on export.csv, got %d", wSafety.Code)
	}

	// Insert vehicles with formula injection attack payloads
	injReg1 := "=cmd|' /C calc'!A0"
	injReg2 := "+SUM(A1:A10)"
	injReg3 := "-100"
	injReg4 := "@ATTACK"

	for _, reg := range []string{injReg1, injReg2, injReg3, injReg4} {
		_, _ = db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
			VALUES ($1, 'AttackModel', 'Van', 5000, 1000, 20000, 'Available', true, now(), now())
			ON CONFLICT (registration_number) DO NOTHING`, reg)
	}

	defer func() {
		for _, reg := range []string{injReg1, injReg2, injReg3, injReg4} {
			_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, reg)
		}
	}()

	req := httptest.NewRequest("GET", "/api/v1/dashboard/export.csv", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for export.csv, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/csv") {
		t.Fatalf("expected text/csv Content-Type, got %s", contentType)
	}

	disp := w.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment; filename=\"dashboard-") {
		t.Fatalf("expected attachment Content-Disposition, got %s", disp)
	}

	csvBody := w.Body.String()

	// Assert each malicious payload is sanitized with leading single quote "'"
	if !strings.Contains(csvBody, "'=cmd|' /C calc'!A0") {
		t.Errorf("injection not mitigated for '=cmd': %s", csvBody)
	}
	if !strings.Contains(csvBody, "'+SUM(A1:A10)") {
		t.Errorf("injection not mitigated for '+SUM': %s", csvBody)
	}
	if !strings.Contains(csvBody, "'-100") {
		t.Errorf("injection not mitigated for '-100': %s", csvBody)
	}
	if !strings.Contains(csvBody, "'@ATTACK") {
		t.Errorf("injection not mitigated for '@ATTACK': %s", csvBody)
	}
}

// ---------------- 5. DATA MINIMIZATION & ROLE MATRIX TESTS ----------------

func TestCompulsory_DataMinimization_ComprehensiveMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.min.matrix@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.min.matrix@transitops.com", rbac.RoleFleetManager)
	finCookie := createTestUserAndLogin(t, app, "fin.min.matrix@transitops.com", rbac.RoleFinancialAnalyst)
	safetyCookie := createTestUserAndLogin(t, app, "safety.min.matrix@transitops.com", rbac.RoleSafetyOfficer)

	driverLic := fmt.Sprintf("MIN-DRV-%s", uuid.NewString()[:6])
	createP4Driver(t, db, driverLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))
	driverEmail := fmt.Sprintf("driver.min.matrix.%s@transitops.com", uuid.NewString()[:6])
	driverCookie := createTestUserAndLogin(t, app, driverEmail, rbac.RoleDriver)
	_, _ = db.Exec(ctx, `UPDATE users SET driver_license = $1 WHERE email = $2`, driverLic, driverEmail)

	type endpointCheck struct {
		method string
		path   string
		body   string
		expect map[string]int // role -> expected HTTP status
	}

	checks := []endpointCheck{
		{
			method: "GET",
			path:   "/api/v1/maintenance",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusOK,
				rbac.RoleFleetManager:     http.StatusOK,
				rbac.RoleSafetyOfficer:    http.StatusOK,
				rbac.RoleFinancialAnalyst: http.StatusForbidden,
				rbac.RoleDriver:           http.StatusForbidden,
			},
		},
		{
			method: "GET",
			path:   "/api/v1/expenses",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusOK,
				rbac.RoleFleetManager:     http.StatusOK,
				rbac.RoleFinancialAnalyst: http.StatusOK,
				rbac.RoleSafetyOfficer:    http.StatusForbidden,
				rbac.RoleDriver:           http.StatusForbidden,
			},
		},
		{
			method: "GET",
			path:   "/api/v1/finance/summary",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusOK,
				rbac.RoleFleetManager:     http.StatusOK,
				rbac.RoleFinancialAnalyst: http.StatusOK,
				rbac.RoleSafetyOfficer:    http.StatusForbidden,
				rbac.RoleDriver:           http.StatusForbidden,
			},
		},
		{
			method: "GET",
			path:   "/api/v1/dashboard/kpis",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusOK,
				rbac.RoleFleetManager:     http.StatusOK,
				rbac.RoleFinancialAnalyst: http.StatusOK,
				rbac.RoleSafetyOfficer:    http.StatusOK,
				rbac.RoleDriver:           http.StatusForbidden,
			},
		},
		{
			method: "GET",
			path:   "/api/v1/dashboard/me",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusForbidden,
				rbac.RoleFleetManager:     http.StatusForbidden,
				rbac.RoleFinancialAnalyst: http.StatusForbidden,
				rbac.RoleSafetyOfficer:    http.StatusForbidden,
				rbac.RoleDriver:           http.StatusOK,
			},
		},
		{
			method: "GET",
			path:   "/api/v1/dashboard/export.csv",
			expect: map[string]int{
				rbac.RoleAdmin:            http.StatusOK,
				rbac.RoleFleetManager:     http.StatusOK,
				rbac.RoleFinancialAnalyst: http.StatusOK,
				rbac.RoleSafetyOfficer:    http.StatusForbidden,
				rbac.RoleDriver:           http.StatusForbidden,
			},
		},
	}

	cookieMap := map[string]*http.Cookie{
		rbac.RoleAdmin:            adminCookie,
		rbac.RoleFleetManager:     fleetCookie,
		rbac.RoleFinancialAnalyst: finCookie,
		rbac.RoleSafetyOfficer:    safetyCookie,
		rbac.RoleDriver:           driverCookie,
	}

	for _, check := range checks {
		for role, expectedCode := range check.expect {
			cookie := cookieMap[role]
			var req *http.Request
			if check.body != "" {
				req = httptest.NewRequest(check.method, check.path, strings.NewReader(check.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", uuid.NewString())
			} else {
				req = httptest.NewRequest(check.method, check.path, nil)
			}
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)

			if w.Code != expectedCode {
				t.Errorf("%s %s for role %s: expected %d, got %d (%s)", check.method, check.path, role, expectedCode, w.Code, w.Body.String())
			}
		}
	}

	// Verify Data Minimization in Driver Self View
	reqDriverMe := httptest.NewRequest("GET", "/api/v1/dashboard/me", nil)
	reqDriverMe.AddCookie(driverCookie)
	wDriverMe := httptest.NewRecorder()
	app.ServeHTTP(wDriverMe, reqDriverMe)

	if wDriverMe.Code != http.StatusOK {
		t.Fatalf("expected 200 for driver on /dashboard/me, got %d", wDriverMe.Code)
	}

	driverBody := wDriverMe.Body.String()
	forbiddenFields := []string{"revenue", "operational_cost", "roi", "fuel_cost", "maintenance_cost"}
	for _, field := range forbiddenFields {
		if strings.Contains(strings.ToLower(driverBody), `"`+field+`"`) {
			t.Errorf("driver /dashboard/me leaked fleet financial field: %s", field)
		}
	}
}

// ---------------- 6. AUDIT LOG COMPULSORY TESTS ----------------

func TestCompulsory_Audit_RoleMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.audit.rm@transitops.com", rbac.RoleAdmin)
	fleetCookie := createTestUserAndLogin(t, app, "fleet.audit.rm@transitops.com", rbac.RoleFleetManager)
	finCookie := createTestUserAndLogin(t, app, "fin.audit.rm@transitops.com", rbac.RoleFinancialAnalyst)
	safetyCookie := createTestUserAndLogin(t, app, "safety.audit.rm@transitops.com", rbac.RoleSafetyOfficer)
	driverCookie := createTestUserAndLogin(t, app, "driver.audit.rm@transitops.com", rbac.RoleDriver)

	checkAudit := func(cookie *http.Cookie, expectedCode int) {
		req := httptest.NewRequest("GET", "/api/v1/audit", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != expectedCode {
			t.Errorf("GET /audit expected %d, got %d", expectedCode, w.Code)
		}
	}

	checkAudit(adminCookie, http.StatusOK)
	checkAudit(fleetCookie, http.StatusForbidden)
	checkAudit(finCookie, http.StatusForbidden)
	checkAudit(safetyCookie, http.StatusForbidden)
	checkAudit(driverCookie, http.StatusForbidden)
}

func TestCompulsory_Audit_MutatingActionLoggingAndFiltering(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminEmail := fmt.Sprintf("admin.audit.act.%s@transitops.com", uuid.NewString()[:6])
	adminCookie := createTestUserAndLogin(t, app, adminEmail, rbac.RoleAdmin)

	vReg := fmt.Sprintf("AUD-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("AUD-D-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	// 1. Create a trip via API
	tripBody, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     1500,
	})
	reqCreate := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(tripBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Idempotency-Key", uuid.NewString())
	reqCreate.AddCookie(adminCookie)
	wCreate := httptest.NewRecorder()
	app.ServeHTTP(wCreate, reqCreate)

	if wCreate.Code != http.StatusCreated && wCreate.Code != http.StatusOK {
		t.Fatalf("failed to create trip: %d: %s", wCreate.Code, wCreate.Body.String())
	}
	var createdTrip map[string]any
	_ = json.Unmarshal(wCreate.Body.Bytes(), &createdTrip)
	tripIDVal := createdTrip["id"]
	tripIDStr := fmt.Sprintf("%.0f", tripIDVal.(float64))

	// 2. Dispatch the trip
	reqDisp := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%s/dispatch", tripIDStr), strings.NewReader(`{}`))
	reqDisp.Header.Set("Content-Type", "application/json")
	reqDisp.Header.Set("Idempotency-Key", uuid.NewString())
	reqDisp.AddCookie(adminCookie)
	wDisp := httptest.NewRecorder()
	app.ServeHTTP(wDisp, reqDisp)

	if wDisp.Code != http.StatusOK {
		t.Fatalf("failed to dispatch trip: %d: %s", wDisp.Code, wDisp.Body.String())
	}

	// 3. Query audit log with entity=trips and entity_id=tripIDStr
	reqAudit := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=trips&entity_id=%s", tripIDStr), nil)
	reqAudit.AddCookie(adminCookie)
	wAudit := httptest.NewRecorder()
	app.ServeHTTP(wAudit, reqAudit)

	if wAudit.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /audit query, got %d: %s", wAudit.Code, wAudit.Body.String())
	}

	var auditResp PaginatedResponse[AuditEntry]
	if err := json.Unmarshal(wAudit.Body.Bytes(), &auditResp); err != nil {
		t.Fatalf("failed to parse audit response: %v", err)
	}

	if len(auditResp.Items) < 1 {
		t.Fatalf("expected at least 1 audit entry for trip %s, got %d", tripIDStr, len(auditResp.Items))
	}

	// Verify actor and action
	foundDispatch := false
	for _, entry := range auditResp.Items {
		if entry.Action == "dispatch" {
			foundDispatch = true
			if entry.ActorEmail == nil || *entry.ActorEmail != adminEmail {
				t.Errorf("expected actor_email %s, got %v", adminEmail, entry.ActorEmail)
			}
		}
	}
	if !foundDispatch {
		t.Errorf("dispatch action not recorded in audit log")
	}

	// 4. Test date range filtering
	today := time.Now().UTC().Format("2006-01-02")
	reqDateAudit := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=trips&from=%s&to=%s", today, today), nil)
	reqDateAudit.AddCookie(adminCookie)
	wDateAudit := httptest.NewRecorder()
	app.ServeHTTP(wDateAudit, reqDateAudit)

	if wDateAudit.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for date filtered audit, got %d", wDateAudit.Code)
	}

	// Past date range that shouldn't match our newly created trip
	pastDate := "2020-01-01"
	reqPastAudit := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=trips&entity_id=%s&from=%s&to=%s", tripIDStr, pastDate, pastDate), nil)
	reqPastAudit.AddCookie(adminCookie)
	wPastAudit := httptest.NewRecorder()
	app.ServeHTTP(wPastAudit, reqPastAudit)

	var pastResp PaginatedResponse[AuditEntry]
	_ = json.Unmarshal(wPastAudit.Body.Bytes(), &pastResp)
	if len(pastResp.Items) != 0 {
		t.Errorf("expected 0 entries for past date filter, got %d", len(pastResp.Items))
	}
}
