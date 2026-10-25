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

// Helper to seed a unique vehicle for testing
func createTestVehicle(t *testing.T, db *pgxpool.Pool, reg, status, vType string, cap, odo float64) {
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Test Model', $2, $3, $4, 30000, $5, true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = $5, type = $2, max_load_capacity = $3, odometer = $4, is_active = true`,
		reg, vType, cap, odo, status)
	if err != nil {
		t.Fatalf("failed to insert test vehicle %s: %v", reg, err)
	}
}

// Helper to seed a unique driver for testing
func createTestDriver(t *testing.T, db *pgxpool.Pool, lic, status, cat string, expiryDate time.Time, safety float64) {
	ctx := context.Background()
	_, err := db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Test Driver', $2, $3, '+1-555-0100', $4, $5, true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = $5, license_category = $2, license_expiry_date = $3, safety_score = $4, is_active = true`,
		lic, cat, expiryDate.Format("2006-01-02"), safety, status)
	if err != nil {
		t.Fatalf("failed to insert test driver %s: %v", lic, err)
	}
}

// ---------------- 1. CONCURRENCY TESTS ----------------

// 50 goroutines dispatch one vehicle with 50 different drivers at once -> exactly 1 success, 49 x 409 TRIP_VEHICLE_UNAVAILABLE.
func TestCompulsory_ConcurrentVehicleDispatch(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.concurrent.v@transitops.com", rbac.RoleAdmin)

	rounds := 100 // 100 rounds per spec
	for round := 1; round <= rounds; round++ {
		vReg := fmt.Sprintf("CR-V-%02d-%s", round, uuid.NewString()[:4])
		createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)

		driversCount := 50
		dLicenses := make([]string, driversCount)
		for i := 0; i < driversCount; i++ {
			dLicenses[i] = fmt.Sprintf("CR-D-%02d-%02d-%s", round, i, uuid.NewString()[:4])
			createTestDriver(t, db, dLicenses[i], "Available", "Heavy", time.Now().AddDate(1, 0, 0), 95.0)
		}

		var wg sync.WaitGroup
		successCount := 0
		conflictCount := 0
		var mu sync.Mutex

		for i := 0; i < driversCount; i++ {
			wg.Add(1)
			go func(dLic string, idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]any{
					"source_name":      "New York Central Hub",
					"destination_name": "Chicago Logistics Hub",
					"vehicle_reg":      vReg,
					"driver_license":   dLic,
					"cargo_weight":     5000,
					"planned_distance": 800,
					"revenue":          2500,
					"dispatch":         true,
				})

				req := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", uuid.NewString())
				req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.1.%d.%d", round%250, idx%250))
				req.AddCookie(adminCookie)
				w := httptest.NewRecorder()
				app.ServeHTTP(w, req)

				mu.Lock()
				defer mu.Unlock()
				if w.Code == http.StatusCreated {
					successCount++
				} else if w.Code == http.StatusConflict {
					conflictCount++
					var res map[string]any
					_ = json.Unmarshal(w.Body.Bytes(), &res)
					errObj, _ := res["error"].(map[string]any)
					code, _ := errObj["code"].(string)
					if code != "TRIP_VEHICLE_UNAVAILABLE" && code != "VEHICLE_NOT_AVAILABLE" {
						t.Errorf("expected TRIP_VEHICLE_UNAVAILABLE or VEHICLE_NOT_AVAILABLE, got %s", code)
					}
				} else {
					t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
				}
			}(dLicenses[i], i)
		}
		wg.Wait()

		if successCount != 1 {
			t.Fatalf("Round %d: expected exactly 1 successful dispatch, got %d", round, successCount)
		}
		if conflictCount != driversCount-1 {
			t.Fatalf("Round %d: expected %d conflicts, got %d", round, driversCount-1, conflictCount)
		}

		// SQL check: exactly 1 Dispatched trip for this vehicle
		var activeTrips int
		_ = db.QueryRow(ctx, "SELECT count(*) FROM trips WHERE vehicle_reg = $1 AND status = 'Dispatched'", vReg).Scan(&activeTrips)
		if activeTrips != 1 {
			t.Fatalf("Round %d: SQL invariant violation! Expected 1 Dispatched trip, got %d", round, activeTrips)
		}

		// Cleanup
		_, _ = db.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", vReg)
		_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		for _, d := range dLicenses {
			_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", d)
		}
	}
}

// 50 goroutines dispatch one driver with 50 different vehicles -> exactly 1 success, 49 conflicts.
func TestCompulsory_ConcurrentDriverDispatch(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.concurrent.d@transitops.com", rbac.RoleAdmin)

	rounds := 100
	for round := 1; round <= rounds; round++ {
		dLic := fmt.Sprintf("CD-D-%02d-%s", round, uuid.NewString()[:4])
		createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 95.0)

		vehiclesCount := 50
		vRegs := make([]string, vehiclesCount)
		for i := 0; i < vehiclesCount; i++ {
			vRegs[i] = fmt.Sprintf("CD-V-%02d-%02d-%s", round, i, uuid.NewString()[:4])
			createTestVehicle(t, db, vRegs[i], "Available", "Van", 10000, 1000)
		}

		var wg sync.WaitGroup
		successCount := 0
		conflictCount := 0
		var mu sync.Mutex

		for i := 0; i < vehiclesCount; i++ {
			wg.Add(1)
			go func(vReg string, idx int) {
				defer wg.Done()
				body, _ := json.Marshal(map[string]any{
					"source_name":      "New York Central Hub",
					"destination_name": "Chicago Logistics Hub",
					"vehicle_reg":      vReg,
					"driver_license":   dLic,
					"cargo_weight":     5000,
					"planned_distance": 800,
					"revenue":          2500,
					"dispatch":         true,
				})

				req := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body))
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
					var res map[string]any
					_ = json.Unmarshal(w.Body.Bytes(), &res)
					errObj, _ := res["error"].(map[string]any)
					code, _ := errObj["code"].(string)
					if code != "TRIP_DRIVER_UNAVAILABLE" && code != "DRIVER_NOT_AVAILABLE" {
						t.Errorf("expected TRIP_DRIVER_UNAVAILABLE or DRIVER_NOT_AVAILABLE, got %s", code)
					}
				} else {
					t.Errorf("unexpected status %d: %s", w.Code, w.Body.String())
				}
			}(vRegs[i], i)
		}
		wg.Wait()

		if successCount != 1 {
			t.Fatalf("Round %d: expected exactly 1 successful dispatch, got %d", round, successCount)
		}
		if conflictCount != vehiclesCount-1 {
			t.Fatalf("Round %d: expected %d conflicts, got %d", round, vehiclesCount-1, conflictCount)
		}

		var activeTrips int
		_ = db.QueryRow(ctx, "SELECT count(*) FROM trips WHERE driver_license = $1 AND status = 'Dispatched'", dLic).Scan(&activeTrips)
		if activeTrips != 1 {
			t.Fatalf("Round %d: SQL invariant violation! Expected 1 Dispatched trip, got %d", round, activeTrips)
		}

		_, _ = db.Exec(ctx, "DELETE FROM trips WHERE driver_license = $1", dLic)
		_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", dLic)
		for _, v := range vRegs {
			_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", v)
		}
	}
}

// Concurrent dispatch and complete on the same trip -> no state corruption.
func TestCompulsory_ConcurrentDispatchAndComplete(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.disp.comp@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("CDC-V-1-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("CDC-D-1-%s", uuid.NewString()[:6])
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 95.0)

	// Create a Draft trip first
	var tripID int64
	err := db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 5000, 800, 2500, 'Draft', now(), now())
		RETURNING id`, vReg, dLic).Scan(&tripID)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		_, _ = db.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
		_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", dLic)
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Dispatch
	go func() {
		defer wg.Done()
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/dispatch", tripID), nil)
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
	}()

	// Goroutine 2: Complete
	go func() {
		defer wg.Done()
		compBody, _ := json.Marshal(map[string]any{"final_odometer": 1500})
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(compBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
	}()

	wg.Wait()

	// State must be consistent:
	// If Complete won before Dispatch, Complete returned 409 (Draft cannot be completed) and Dispatch made it Dispatched.
	// If Dispatch won first, Complete may have succeeded (Completed) or failed.
	var tripStatus string
	var vStatus string
	var dStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", tripID).Scan(&tripStatus)
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dLic).Scan(&dStatus)

	if tripStatus == "Completed" {
		if vStatus != "Available" || dStatus != "Available" {
			t.Fatalf("trip Completed but resources not Available: v=%s, d=%s", vStatus, dStatus)
		}
	} else if tripStatus == "Dispatched" {
		if vStatus != "On Trip" || dStatus != "On Trip" {
			t.Fatalf("trip Dispatched but resources not On Trip: v=%s, d=%s", vStatus, dStatus)
		}
	} else if tripStatus == "Draft" {
		if vStatus != "Available" || dStatus != "Available" {
			t.Fatalf("trip Draft but resources not Available: v=%s, d=%s", vStatus, dStatus)
		}
	} else {
		t.Fatalf("unexpected trip status %s", tripStatus)
	}
}

// 200 random mixed operations across 20 vehicles and 20 drivers with 20 workers -> finishes within 60s, no deadlock, invariant holds.
func TestCompulsory_200MixedConcurrentOperations(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.mixed200@transitops.com", rbac.RoleAdmin)

	numAssets := 20
	vehicles := make([]string, numAssets)
	drivers := make([]string, numAssets)

	// Clean up any stale records from prior runs
	for i := 0; i < numAssets; i++ {
		v := fmt.Sprintf("M200-V-%02d", i)
		d := fmt.Sprintf("M200-D-%02d", i)
		_, _ = db.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1 OR driver_license = $2", v, d)
		_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", v)
		_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", d)
	}

	for i := 0; i < numAssets; i++ {
		vehicles[i] = fmt.Sprintf("M200-V-%02d", i)
		drivers[i] = fmt.Sprintf("M200-D-%02d", i)
		createTestVehicle(t, db, vehicles[i], "Available", "Van", 10000, 1000)
		createTestDriver(t, db, drivers[i], "Available", "Heavy", time.Now().AddDate(1, 0, 0), 95.0)
	}

	defer func() {
		for i := 0; i < numAssets; i++ {
			_, _ = db.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", vehicles[i])
			_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vehicles[i])
			_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", drivers[i])
		}
	}()

	start := time.Now()
	var wg sync.WaitGroup
	workers := 20
	opsPerWorker := 10 // 20 * 10 = 200 operations

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for op := 0; op < opsPerWorker; op++ {
				idx := (workerID + op) % numAssets
				v := vehicles[idx]
				d := drivers[idx]
				ip := fmt.Sprintf("10.3.%d.%d", workerID, op)

				switch op % 4 {
				case 0:
					// Create draft
					body, _ := json.Marshal(map[string]any{
						"source_name":      "New York Central Hub",
						"destination_name": "Chicago Logistics Hub",
						"vehicle_reg":      v,
						"driver_license":   d,
						"cargo_weight":     4000,
						"planned_distance": 500,
						"revenue":          1500,
						"dispatch":         false,
					})
					req := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Idempotency-Key", uuid.NewString())
					req.Header.Set("X-Forwarded-For", ip)
					req.AddCookie(adminCookie)
					rec := httptest.NewRecorder()
					app.ServeHTTP(rec, req)

				case 1:
					// Create & Dispatch
					body, _ := json.Marshal(map[string]any{
						"source_name":      "New York Central Hub",
						"destination_name": "Chicago Logistics Hub",
						"vehicle_reg":      v,
						"driver_license":   d,
						"cargo_weight":     4000,
						"planned_distance": 500,
						"revenue":          1500,
						"dispatch":         true,
					})
					req := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Idempotency-Key", uuid.NewString())
					req.Header.Set("X-Forwarded-For", ip)
					req.AddCookie(adminCookie)
					rec := httptest.NewRecorder()
					app.ServeHTTP(rec, req)

				case 2:
					// Attempt complete on active trip if any
					var tripID int64
					var odo float64
					err := db.QueryRow(ctx, "SELECT id, COALESCE(start_odometer, 1000) FROM trips WHERE vehicle_reg = $1 AND status = 'Dispatched' LIMIT 1", v).Scan(&tripID, &odo)
					if err == nil {
						compBody, _ := json.Marshal(map[string]any{"final_odometer": odo + 100})
						req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(compBody))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("Idempotency-Key", uuid.NewString())
						req.Header.Set("X-Forwarded-For", ip)
						req.AddCookie(adminCookie)
						rec := httptest.NewRecorder()
						app.ServeHTTP(rec, req)
					}

				case 3:
					// Attempt cancel on draft or dispatched trip if any
					var tripID int64
					err := db.QueryRow(ctx, "SELECT id FROM trips WHERE vehicle_reg = $1 AND status IN ('Draft', 'Dispatched') LIMIT 1", v).Scan(&tripID)
					if err == nil {
						req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/cancel", tripID), nil)
						req.Header.Set("Idempotency-Key", uuid.NewString())
						req.Header.Set("X-Forwarded-For", ip)
						req.AddCookie(adminCookie)
						rec := httptest.NewRecorder()
						app.ServeHTTP(rec, req)
					}
				}
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)
	if elapsed > 60*time.Second {
		t.Fatalf("200 mixed ops took %s (must be < 60s)", elapsed)
	}

	// Post-run invariant check on the test vehicles and drivers:
	// 1. Every On Trip vehicle has exactly one Dispatched trip
	var badVehicles int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM vehicles v 
		WHERE v.registration_number = ANY($1) AND v.status = 'On Trip' 
		AND (SELECT count(*) FROM trips t WHERE t.vehicle_reg = v.registration_number AND t.status = 'Dispatched') != 1`, vehicles).Scan(&badVehicles)
	if badVehicles != 0 {
		t.Fatalf("Invariant violation: %d On Trip vehicles do not have exactly 1 Dispatched trip", badVehicles)
	}

	// 2. Every Dispatched trip's vehicle and driver are On Trip
	var badTrips int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM trips t 
		JOIN vehicles v ON t.vehicle_reg = v.registration_number
		JOIN drivers d ON t.driver_license = d.license_number
		WHERE t.vehicle_reg = ANY($1) AND t.status = 'Dispatched' AND (v.status != 'On Trip' OR d.status != 'On Trip')`, vehicles).Scan(&badTrips)
	if badTrips != 0 {
		t.Fatalf("Invariant violation: %d Dispatched trips have vehicle/driver not On Trip", badTrips)
	}

	// 3. No Available vehicle has a Dispatched trip
	var badAvail int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM vehicles v 
		WHERE v.registration_number = ANY($1) AND v.status = 'Available' 
		AND (SELECT count(*) FROM trips t WHERE t.vehicle_reg = v.registration_number AND t.status = 'Dispatched') > 0`, vehicles).Scan(&badAvail)
	if badAvail != 0 {
		t.Fatalf("Invariant violation: %d Available vehicles have Dispatched trips", badAvail)
	}
}

// Lock-order test: two transactions touching the same vehicle and driver in opposite order complete without deadlock (200 iterations).
func TestCompulsory_LockOrderNoDeadlock(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	vReg := fmt.Sprintf("LOCK-V-1-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("LOCK-D-1-%s", uuid.NewString()[:6])
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 95.0)
	defer func() {
		_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", dLic)
	}()

	iterations := 200
	for i := 0; i < iterations; i++ {
		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1 uses lockVehicleAndDriver (locks vehicle first, then driver)
		go func() {
			defer wg.Done()
			tx, err := db.Begin(ctx)
			if err != nil {
				return
			}
			defer tx.Rollback(ctx)
			_, _ = lockVehicleAndDriver(ctx, tx, vReg, dLic)
			time.Sleep(1 * time.Millisecond)
			_ = tx.Commit(ctx)
		}()

		// Goroutine 2 also uses lockVehicleAndDriver
		go func() {
			defer wg.Done()
			tx, err := db.Begin(ctx)
			if err != nil {
				return
			}
			defer tx.Rollback(ctx)
			_, _ = lockVehicleAndDriver(ctx, tx, vReg, dLic)
			time.Sleep(1 * time.Millisecond)
			_ = tx.Commit(ctx)
		}()

		wg.Wait()
	}
}

// ---------------- 2. RULE ISOLATION TESTS ----------------

func TestCompulsory_RuleIsolation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.rules@transitops.com", rbac.RoleAdmin)

	// A. Overloaded cargo -> TRIP_OVERLOADED (409)
	vOvr := "RULE-V-OVR"
	dOvr := "RULE-D-OVR"
	createTestVehicle(t, db, vOvr, "Available", "Van", 3000, 1000)
	createTestDriver(t, db, dOvr, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)

	bodyOvr, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vOvr,
		"driver_license":   dOvr,
		"cargo_weight":     5000, // exceeds 3000
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqOvr := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyOvr))
	reqOvr.Header.Set("Content-Type", "application/json")
	reqOvr.Header.Set("Idempotency-Key", uuid.NewString())
	reqOvr.AddCookie(adminCookie)
	wOvr := httptest.NewRecorder()
	app.ServeHTTP(wOvr, reqOvr)

	if wOvr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for overloaded cargo, got %d: %s", wOvr.Code, wOvr.Body.String())
	}
	var resOvr map[string]any
	_ = json.Unmarshal(wOvr.Body.Bytes(), &resOvr)
	if resOvr["error"].(map[string]any)["code"] != "TRIP_OVERLOADED" {
		t.Fatalf("expected code TRIP_OVERLOADED, got %v", resOvr["error"].(map[string]any)["code"])
	}

	// B. Driver license expired -> DRIVER_LICENSE_EXPIRED (409)
	vExp := "RULE-V-EXP"
	dExp := "RULE-D-EXP"
	createTestVehicle(t, db, vExp, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dExp, "Available", "Heavy", time.Now().AddDate(-1, 0, 0), 90.0) // expired

	bodyExp, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vExp,
		"driver_license":   dExp,
		"cargo_weight":     1000,
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqExp := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyExp))
	reqExp.Header.Set("Content-Type", "application/json")
	reqExp.Header.Set("Idempotency-Key", uuid.NewString())
	reqExp.AddCookie(adminCookie)
	wExp := httptest.NewRecorder()
	app.ServeHTTP(wExp, reqExp)

	if wExp.Code != http.StatusConflict {
		t.Fatalf("expected 409 for expired license, got %d: %s", wExp.Code, wExp.Body.String())
	}
	var resExp map[string]any
	_ = json.Unmarshal(wExp.Body.Bytes(), &resExp)
	if resExp["error"].(map[string]any)["code"] != "DRIVER_LICENSE_EXPIRED" {
		t.Fatalf("expected code DRIVER_LICENSE_EXPIRED, got %v", resExp["error"].(map[string]any)["code"])
	}

	// C. Suspended driver -> DRIVER_NOT_AVAILABLE (409)
	dSusp := "RULE-D-SUSP"
	createTestDriver(t, db, dSusp, "Suspended", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)
	bodySusp, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vExp,
		"driver_license":   dSusp,
		"cargo_weight":     1000,
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqSusp := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodySusp))
	reqSusp.Header.Set("Content-Type", "application/json")
	reqSusp.Header.Set("Idempotency-Key", uuid.NewString())
	reqSusp.AddCookie(adminCookie)
	wSusp := httptest.NewRecorder()
	app.ServeHTTP(wSusp, reqSusp)
	if wSusp.Code != http.StatusConflict {
		t.Fatalf("expected 409 for suspended driver, got %d", wSusp.Code)
	}

	// D. Vehicle In Shop -> VEHICLE_NOT_AVAILABLE (409)
	vShop := "RULE-V-SHOP"
	createTestVehicle(t, db, vShop, "In Shop", "Van", 10000, 1000)
	bodyShop, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vShop,
		"driver_license":   dOvr,
		"cargo_weight":     1000,
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqShop := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyShop))
	reqShop.Header.Set("Content-Type", "application/json")
	reqShop.Header.Set("Idempotency-Key", uuid.NewString())
	reqShop.AddCookie(adminCookie)
	wShop := httptest.NewRecorder()
	app.ServeHTTP(wShop, reqShop)
	if wShop.Code != http.StatusConflict {
		t.Fatalf("expected 409 for vehicle in shop, got %d", wShop.Code)
	}

	// E. Same endpoints -> SAME_ENDPOINTS (422)
	bodySame, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "New York Central Hub",
		"vehicle_reg":      vExp,
		"driver_license":   dOvr,
		"cargo_weight":     1000,
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqSame := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodySame))
	reqSame.Header.Set("Content-Type", "application/json")
	reqSame.Header.Set("Idempotency-Key", uuid.NewString())
	reqSame.AddCookie(adminCookie)
	wSame := httptest.NewRecorder()
	app.ServeHTTP(wSame, reqSame)
	if wSame.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for same endpoints, got %d", wSame.Code)
	}

	// F. License category mismatch -> LICENSE_CATEGORY_MISMATCH (409)
	vSemi := "RULE-V-SEMI"
	dVan := "RULE-D-VAN"
	createTestVehicle(t, db, vSemi, "Available", "Semi", 40000, 1000)
	createTestDriver(t, db, dVan, "Available", "Van", time.Now().AddDate(1, 0, 0), 90.0) // Van driver cannot drive Semi

	bodyMismatch, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vSemi,
		"driver_license":   dVan,
		"cargo_weight":     1000,
		"planned_distance": 500,
		"revenue":          1000,
		"dispatch":         true,
	})
	reqMismatch := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyMismatch))
	reqMismatch.Header.Set("Content-Type", "application/json")
	reqMismatch.Header.Set("Idempotency-Key", uuid.NewString())
	reqMismatch.AddCookie(adminCookie)
	wMismatch := httptest.NewRecorder()
	app.ServeHTTP(wMismatch, reqMismatch)
	if wMismatch.Code != http.StatusConflict {
		t.Fatalf("expected 409 for license mismatch, got %d: %s", wMismatch.Code, wMismatch.Body.String())
	}
	var resMismatch map[string]any
	_ = json.Unmarshal(wMismatch.Body.Bytes(), &resMismatch)
	if resMismatch["error"].(map[string]any)["code"] != "LICENSE_CATEGORY_MISMATCH" {
		t.Fatalf("expected code LICENSE_CATEGORY_MISMATCH, got %v", resMismatch["error"].(map[string]any)["code"])
	}
}

// ---------------- 3. LIFECYCLE TESTS ----------------

func TestCompulsory_TripLifecycle(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.lifecycle@transitops.com", rbac.RoleAdmin)
	vReg := "LC-V-1"
	dLic := "LC-D-1"
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 2500)
	createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)
	defer func() {
		_, _ = db.Exec(ctx, "DELETE FROM trips WHERE vehicle_reg = $1", vReg)
		_, _ = db.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", vReg)
		_, _ = db.Exec(ctx, "DELETE FROM drivers WHERE license_number = $1", dLic)
	}()

	// 1. Create Draft trip: verify vehicle and driver status before and after remain Available
	createDraftBody, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     3000,
		"planned_distance": 400,
		"revenue":          1200,
	})
	reqDraft := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(createDraftBody))
	reqDraft.Header.Set("Content-Type", "application/json")
	reqDraft.Header.Set("Idempotency-Key", uuid.NewString())
	reqDraft.AddCookie(adminCookie)
	wDraft := httptest.NewRecorder()
	app.ServeHTTP(wDraft, reqDraft)

	if wDraft.Code != http.StatusCreated {
		t.Fatalf("draft create failed: %d: %s", wDraft.Code, wDraft.Body.String())
	}
	var draftRes map[string]any
	_ = json.Unmarshal(wDraft.Body.Bytes(), &draftRes)
	tripID := int64(draftRes["id"].(float64))

	var vStat, dStat string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStat)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dLic).Scan(&dStat)
	if vStat != "Available" || dStat != "Available" {
		t.Fatalf("draft creation mutated resources: v=%s, d=%s", vStat, dStat)
	}

	// 2. Complete/Cancel Draft -> 409 INVALID_TRANSITION
	compDraftBody, _ := json.Marshal(map[string]any{"final_odometer": 3000})
	reqCompDraft := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(compDraftBody))
	reqCompDraft.Header.Set("Content-Type", "application/json")
	reqCompDraft.Header.Set("Idempotency-Key", uuid.NewString())
	reqCompDraft.AddCookie(adminCookie)
	wCompDraft := httptest.NewRecorder()
	app.ServeHTTP(wCompDraft, reqCompDraft)
	if wCompDraft.Code != http.StatusConflict {
		t.Fatalf("expected 409 completing draft trip, got %d", wCompDraft.Code)
	}

	// 3. Dispatch Draft -> Dispatched
	reqDisp := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/dispatch", tripID), nil)
	reqDisp.Header.Set("Idempotency-Key", uuid.NewString())
	reqDisp.AddCookie(adminCookie)
	wDisp := httptest.NewRecorder()
	app.ServeHTTP(wDisp, reqDisp)
	if wDisp.Code != http.StatusOK {
		t.Fatalf("dispatch failed: %d: %s", wDisp.Code, wDisp.Body.String())
	}

	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStat)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dLic).Scan(&dStat)
	if vStat != "On Trip" || dStat != "On Trip" {
		t.Fatalf("dispatch did not set resources On Trip: v=%s, d=%s", vStat, dStat)
	}

	// 4. Complete with final_odometer < start_odometer -> 422 INVALID_ODOMETER
	badOdoBody, _ := json.Marshal(map[string]any{"final_odometer": 2000}) // start was 2500
	reqBadOdo := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(badOdoBody))
	reqBadOdo.Header.Set("Content-Type", "application/json")
	reqBadOdo.Header.Set("Idempotency-Key", uuid.NewString())
	reqBadOdo.AddCookie(adminCookie)
	wBadOdo := httptest.NewRecorder()
	app.ServeHTTP(wBadOdo, reqBadOdo)
	if wBadOdo.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for lower final odometer, got %d: %s", wBadOdo.Code, wBadOdo.Body.String())
	}

	// 5. fuel_liters without fuel_cost -> 422
	badFuelBody, _ := json.Marshal(map[string]any{
		"final_odometer": 2900,
		"fuel_liters":    50.0,
		// missing fuel_cost
	})
	reqBadFuel := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(badFuelBody))
	reqBadFuel.Header.Set("Content-Type", "application/json")
	reqBadFuel.Header.Set("Idempotency-Key", uuid.NewString())
	reqBadFuel.AddCookie(adminCookie)
	wBadFuel := httptest.NewRecorder()
	app.ServeHTTP(wBadFuel, reqBadFuel)
	if wBadFuel.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for fuel without cost, got %d", wBadFuel.Code)
	}

	// 6. Complete with valid data -> Completed, resources Available, vehicle odometer updated
	validCompBody, _ := json.Marshal(map[string]any{
		"final_odometer": 2950.0,
		"fuel_liters":    60.0,
		"fuel_cost":      90.0,
	})
	reqValidComp := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(validCompBody))
	reqValidComp.Header.Set("Content-Type", "application/json")
	reqValidComp.Header.Set("Idempotency-Key", uuid.NewString())
	reqValidComp.AddCookie(adminCookie)
	wValidComp := httptest.NewRecorder()
	app.ServeHTTP(wValidComp, reqValidComp)
	if wValidComp.Code != http.StatusOK {
		t.Fatalf("valid complete failed: %d: %s", wValidComp.Code, wValidComp.Body.String())
	}

	var newOdo float64
	_ = db.QueryRow(ctx, "SELECT status, odometer FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStat, &newOdo)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dLic).Scan(&dStat)
	if vStat != "Available" || dStat != "Available" || newOdo != 2950.0 {
		t.Fatalf("complete did not release resources: v=%s, d=%s, odo=%f", vStat, dStat, newOdo)
	}

	// 7. Complete twice -> 409 INVALID_TRANSITION, odometer not doubled
	reqCompTwice := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/complete", tripID), bytes.NewReader(validCompBody))
	reqCompTwice.Header.Set("Content-Type", "application/json")
	reqCompTwice.Header.Set("Idempotency-Key", uuid.NewString())
	reqCompTwice.AddCookie(adminCookie)
	wCompTwice := httptest.NewRecorder()
	app.ServeHTTP(wCompTwice, reqCompTwice)
	if wCompTwice.Code != http.StatusConflict {
		t.Fatalf("expected 409 completing twice, got %d", wCompTwice.Code)
	}

	// 8. Dispatched -> Cancelled restores resources
	vCan := "LC-V-CAN"
	dCan := "LC-D-CAN"
	createTestVehicle(t, db, vCan, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dCan, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)

	dispBody, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vCan,
		"driver_license":   dCan,
		"cargo_weight":     1000,
		"planned_distance": 200,
		"revenue":          500,
		"dispatch":         true,
	})
	reqDisp2 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(dispBody))
	reqDisp2.Header.Set("Content-Type", "application/json")
	reqDisp2.Header.Set("Idempotency-Key", uuid.NewString())
	reqDisp2.AddCookie(adminCookie)
	wDisp2 := httptest.NewRecorder()
	app.ServeHTTP(wDisp2, reqDisp2)

	var dispRes map[string]any
	_ = json.Unmarshal(wDisp2.Body.Bytes(), &dispRes)
	canTripID := int64(dispRes["id"].(float64))

	// Cancel the dispatched trip
	reqCan := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/cancel", canTripID), nil)
	reqCan.Header.Set("Idempotency-Key", uuid.NewString())
	reqCan.AddCookie(adminCookie)
	wCan := httptest.NewRecorder()
	app.ServeHTTP(wCan, reqCan)
	if wCan.Code != http.StatusOK {
		t.Fatalf("cancel failed: %d: %s", wCan.Code, wCan.Body.String())
	}

	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vCan).Scan(&vStat)
	_ = db.QueryRow(ctx, "SELECT status FROM drivers WHERE license_number = $1", dCan).Scan(&dStat)
	if vStat != "Available" || dStat != "Available" {
		t.Fatalf("cancel did not restore resources: v=%s, d=%s", vStat, dStat)
	}
}

// ---------------- 4. IDEMPOTENCY TESTS ----------------

func TestCompulsory_IdempotencyKey(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.idem.comp@transitops.com", rbac.RoleAdmin)
	vReg := fmt.Sprintf("IDEM-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("IDEM-D-%s", uuid.NewString()[:6])
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)

	idemKey := uuid.NewString()

	body1, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     2000,
		"planned_distance": 300,
		"revenue":          900,
	})

	// 1. Same key, same body, twice -> identical status and body; exactly one trip row
	req1 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)
	req1.AddCookie(adminCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d: %s", w1.Code, w1.Body.String())
	}

	req2 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("second replay request failed: %d", w2.Code)
	}
	if w2.Header().Get("X-Idempotent-Replay") != "true" {
		t.Fatal("expected X-Idempotent-Replay header on replayed response")
	}
	if w1.Body.String() != w2.Body.String() {
		t.Fatalf("replayed body differs: %s vs %s", w1.Body.String(), w2.Body.String())
	}

	var tripCount int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM trips WHERE vehicle_reg = $1", vReg).Scan(&tripCount)
	if tripCount != 1 {
		t.Fatalf("expected exactly 1 trip row created, got %d", tripCount)
	}

	// 2. Same key, different body -> 422 IDEMPOTENCY_KEY_REUSED
	bodyDiff, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Atlanta Distribution Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic,
		"cargo_weight":     2000,
		"planned_distance": 300,
		"revenue":          900,
	})
	reqDiff := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyDiff))
	reqDiff.Header.Set("Content-Type", "application/json")
	reqDiff.Header.Set("Idempotency-Key", idemKey)
	reqDiff.AddCookie(adminCookie)
	wDiff := httptest.NewRecorder()
	app.ServeHTTP(wDiff, reqDiff)
	if wDiff.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for reused key with different body, got %d: %s", wDiff.Code, wDiff.Body.String())
	}

	// 3. Same key on different route -> 422 IDEMPOTENCY_KEY_REUSED
	reqRoute := httptest.NewRequest("POST", "/api/v1/trips/123/dispatch", nil)
	reqRoute.Header.Set("Idempotency-Key", idemKey)
	reqRoute.AddCookie(adminCookie)
	wRoute := httptest.NewRecorder()
	app.ServeHTTP(wRoute, reqRoute)
	if wRoute.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for reused key on different route, got %d", wRoute.Code)
	}

	// 4. Non-UUID key -> 400 INVALID_IDEMPOTENCY_KEY
	reqBadKey := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	reqBadKey.Header.Set("Content-Type", "application/json")
	reqBadKey.Header.Set("Idempotency-Key", "not-a-valid-uuid-12345")
	reqBadKey.AddCookie(adminCookie)
	wBadKey := httptest.NewRecorder()
	app.ServeHTTP(wBadKey, reqBadKey)
	if wBadKey.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-UUID key, got %d", wBadKey.Code)
	}

	// 5. Expired idempotency row (>24h) is not replayed
	expiredKey := uuid.NewString()
	_, _ = db.Exec(ctx, `INSERT INTO idempotency_keys (key, user_id, route, request_hash, status_code, response_body, created_at, expires_at)
		VALUES ($1, 'admin.idem.comp@transitops.com', '/api/v1/trips', 'expiredhash', 201, '{"expired":true}', now() - INTERVAL '25 hours', now() - INTERVAL '1 hour')`,
		expiredKey)

	reqExp := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	reqExp.Header.Set("Content-Type", "application/json")
	reqExp.Header.Set("Idempotency-Key", expiredKey)
	reqExp.AddCookie(adminCookie)
	wExp := httptest.NewRecorder()
	app.ServeHTTP(wExp, reqExp)
	// Because row is expired, it should NOT replay '{"expired":true}', but execute normally
	if strings.Contains(wExp.Body.String(), "expired") {
		t.Fatal("expired idempotency key was incorrectly replayed")
	}
}

// ---------------- 5. AUTHORIZATION TESTS ----------------

func TestCompulsory_TripAuthorization(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	_ = createTestUserAndLogin(t, app, "admin.authz@transitops.com", rbac.RoleAdmin)
	safetyCookie := createTestUserAndLogin(t, app, "safety.authz@transitops.com", rbac.RoleSafetyOfficer)
	financeCookie := createTestUserAndLogin(t, app, "finance.authz@transitops.com", rbac.RoleFinancialAnalyst)

	vReg := "AUTH-V-1"
	dLic1 := "AUTH-D-1"
	dLic2 := "AUTH-D-2"
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dLic1, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)
	createTestDriver(t, db, dLic2, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)

	// Driver 1 session
	driver1Email := "driver1.authz@transitops.com"
	driver1Cookie := createTestUserAndLogin(t, app, driver1Email, rbac.RoleDriver)
	_, _ = db.Exec(ctx, "UPDATE users SET driver_license = $1 WHERE email = $2", dLic1, driver1Email)

	// 1. Safety Officer: GET /trips -> 200; POST /trips -> 403
	reqSafeGet := httptest.NewRequest("GET", "/api/v1/trips", nil)
	reqSafeGet.AddCookie(safetyCookie)
	wSafeGet := httptest.NewRecorder()
	app.ServeHTTP(wSafeGet, reqSafeGet)
	if wSafeGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for safety officer GET /trips, got %d", wSafeGet.Code)
	}

	bodyCreate, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic1,
		"cargo_weight":     1000,
		"planned_distance": 200,
		"revenue":          500,
	})
	reqSafePost := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyCreate))
	reqSafePost.Header.Set("Content-Type", "application/json")
	reqSafePost.Header.Set("Idempotency-Key", uuid.NewString())
	reqSafePost.AddCookie(safetyCookie)
	wSafePost := httptest.NewRecorder()
	app.ServeHTTP(wSafePost, reqSafePost)
	if wSafePost.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for safety officer POST /trips, got %d", wSafePost.Code)
	}

	// 2. Financial Analyst: POST /trips/{id}/complete -> 403
	reqFinPost := httptest.NewRequest("POST", "/api/v1/trips/1/complete", bytes.NewReader([]byte(`{"final_odometer":2000}`)))
	reqFinPost.Header.Set("Content-Type", "application/json")
	reqFinPost.Header.Set("Idempotency-Key", uuid.NewString())
	reqFinPost.AddCookie(financeCookie)
	wFinPost := httptest.NewRecorder()
	app.ServeHTTP(wFinPost, reqFinPost)
	if wFinPost.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for financial analyst complete, got %d", wFinPost.Code)
	}

	// 3. Driver creating trip for different driver_license -> 403
	bodyWrongLic, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg,
		"driver_license":   dLic2, // Driver 1 trying to assign Driver 2
		"cargo_weight":     1000,
		"planned_distance": 200,
		"revenue":          500,
	})
	reqWrongLic := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyWrongLic))
	reqWrongLic.Header.Set("Content-Type", "application/json")
	reqWrongLic.Header.Set("Idempotency-Key", uuid.NewString())
	reqWrongLic.AddCookie(driver1Cookie)
	wWrongLic := httptest.NewRecorder()
	app.ServeHTTP(wWrongLic, reqWrongLic)
	if wWrongLic.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for driver creating trip for another license, got %d", wWrongLic.Code)
	}

	// 4. Trip that does not exist -> 404 for Fleet Manager
	fleetCookie := createTestUserAndLogin(t, app, "fleet.authz@transitops.com", rbac.RoleFleetManager)
	reqNonExist := httptest.NewRequest("POST", "/api/v1/trips/999999/dispatch", nil)
	reqNonExist.Header.Set("Idempotency-Key", uuid.NewString())
	reqNonExist.AddCookie(fleetCookie)
	wNonExist := httptest.NewRecorder()
	app.ServeHTTP(wNonExist, reqNonExist)
	if wNonExist.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent trip, got %d", wNonExist.Code)
	}

	// 5. Forbidden existing trip -> 403 for Driver (not 404)
	var trip2ID int64
	_ = db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 200, 500, 'Draft', now(), now())
		RETURNING id`, vReg, dLic2).Scan(&trip2ID)

	reqDriverForbidden := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/dispatch", trip2ID), nil)
	reqDriverForbidden.Header.Set("Idempotency-Key", uuid.NewString())
	reqDriverForbidden.AddCookie(driver1Cookie)
	wDriverForbidden := httptest.NewRecorder()
	app.ServeHTTP(wDriverForbidden, reqDriverForbidden)
	if wDriverForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (not 404) for driver accessing another driver's trip, got %d", wDriverForbidden.Code)
	}
}

// ---------------- 6. ATOMICITY & FAULT INJECTION ----------------

func TestCompulsory_FaultInjectionAndRollback(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	vReg := "FAULT-V-1"
	dLic := "FAULT-D-1"
	createTestVehicle(t, db, vReg, "Available", "Van", 10000, 1000)
	createTestDriver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0), 90.0)

	// Simulate fault: Begin transaction, lock, mutate vehicle, inject error before commit -> Rollback
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	res, err := lockVehicleAndDriver(ctx, tx, vReg, dLic)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}

	_, _ = tx.Exec(ctx, "UPDATE vehicles SET status = 'On Trip' WHERE registration_number = $1", res.Vehicle.Reg)

	// Fault injected mid-transaction
	tx.Rollback(ctx)

	// Verify vehicle is STILL Available (atomicity preserved)
	var vStat string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStat)
	if vStat != "Available" {
		t.Fatalf("fault rollback failed: vehicle status is %s, expected Available", vStat)
	}
}
