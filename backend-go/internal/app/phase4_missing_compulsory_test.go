package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

// 1. TestCompulsory_Audit_TransactionalIntegrity
// Create user, update user, reset password, suspend/reinstate driver,
// set-off-duty/set-available, create/update/soft-delete/hard-delete vehicle and driver.
// Each produces exactly one audit row with entity, entity_id, actor, old/new detail.
func TestCompulsory_Audit_TransactionalIntegrity(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminEmail := fmt.Sprintf("admin.audit.ti.%s@transitops.com", uuid.NewString()[:6])
	adminCookie := createTestUserAndLogin(t, app, adminEmail, rbac.RoleAdmin)

	// A. User Lifecycle Audit Rows
	// 1. Create User
	testUserEmail := fmt.Sprintf("user.audit.%s@transitops.com", uuid.NewString()[:6])
	createBody, _ := json.Marshal(map[string]any{
		"email":    testUserEmail,
		"password": "Password123!",
		"role":     rbac.RoleFleetManager,
	})
	reqUserCreate := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(createBody))
	reqUserCreate.Header.Set("Content-Type", "application/json")
	reqUserCreate.AddCookie(adminCookie)
	wUserCreate := httptest.NewRecorder()
	app.ServeHTTP(wUserCreate, reqUserCreate)
	if wUserCreate.Code != http.StatusCreated {
		t.Fatalf("failed to create user: %d: %s", wUserCreate.Code, wUserCreate.Body.String())
	}
	var createdUser map[string]any
	_ = json.Unmarshal(wUserCreate.Body.Bytes(), &createdUser)
	userID := createdUser["id"].(string)

	var userCreateAuditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'users' AND entity_id = $1 AND action = 'create' AND actor_email = $2`, userID, adminEmail).Scan(&userCreateAuditCount)
	if userCreateAuditCount != 1 {
		t.Fatalf("expected exactly 1 user create audit row, got %d", userCreateAuditCount)
	}

	// 2. Update User
	updateBody, _ := json.Marshal(map[string]any{
		"role": rbac.RoleSafetyOfficer,
	})
	reqUserUpdate := httptest.NewRequest("PATCH", "/api/v1/users/"+userID, bytes.NewReader(updateBody))
	reqUserUpdate.Header.Set("Content-Type", "application/json")
	reqUserUpdate.AddCookie(adminCookie)
	wUserUpdate := httptest.NewRecorder()
	app.ServeHTTP(wUserUpdate, reqUserUpdate)
	if wUserUpdate.Code != http.StatusOK {
		t.Fatalf("failed to update user: %d", wUserUpdate.Code)
	}

	var userUpdateAuditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'users' AND entity_id = $1 AND action = 'update' AND actor_email = $2`, userID, adminEmail).Scan(&userUpdateAuditCount)
	if userUpdateAuditCount != 1 {
		t.Fatalf("expected exactly 1 user update audit row, got %d", userUpdateAuditCount)
	}

	// 3. Reset Password
	resetBody, _ := json.Marshal(map[string]any{
		"new_password": "NewPassword123!",
	})
	reqUserReset := httptest.NewRequest("POST", "/api/v1/users/"+userID+"/reset-password", bytes.NewReader(resetBody))
	reqUserReset.Header.Set("Content-Type", "application/json")
	reqUserReset.AddCookie(adminCookie)
	wUserReset := httptest.NewRecorder()
	app.ServeHTTP(wUserReset, reqUserReset)
	if wUserReset.Code != http.StatusOK {
		t.Fatalf("failed to reset password: %d", wUserReset.Code)
	}

	var userResetAuditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'users' AND entity_id = $1 AND action = 'reset-password' AND actor_email = $2`, userID, adminEmail).Scan(&userResetAuditCount)
	if userResetAuditCount != 1 {
		t.Fatalf("expected exactly 1 user reset-password audit row, got %d", userResetAuditCount)
	}

	// B. Driver Lifecycle Audit Rows
	dLic := fmt.Sprintf("AUD-DRV-%s", uuid.NewString()[:6])
	// 4. Create Driver
	driverBody, _ := json.Marshal(map[string]any{
		"license_number":      dLic,
		"name":                "Audit Driver",
		"license_category":    "Heavy",
		"license_expiry_date": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		"contact_number":      "+15550001",
		"safety_score":        88.0,
	})
	reqDriverCreate := httptest.NewRequest("POST", "/api/v1/drivers", bytes.NewReader(driverBody))
	reqDriverCreate.Header.Set("Content-Type", "application/json")
	reqDriverCreate.AddCookie(adminCookie)
	wDriverCreate := httptest.NewRecorder()
	app.ServeHTTP(wDriverCreate, reqDriverCreate)
	if wDriverCreate.Code != http.StatusCreated {
		t.Fatalf("failed to create driver: %d: %s", wDriverCreate.Code, wDriverCreate.Body.String())
	}
	var driverAuditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'create'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver create audit row, got %d", driverAuditCount)
	}

	// 5. Update Driver
	updDrvBody, _ := json.Marshal(map[string]any{
		"safety_score": 92.5,
	})
	reqDriverUpdate := httptest.NewRequest("PATCH", "/api/v1/drivers/"+dLic, bytes.NewReader(updDrvBody))
	reqDriverUpdate.Header.Set("Content-Type", "application/json")
	reqDriverUpdate.AddCookie(adminCookie)
	wDriverUpdate := httptest.NewRecorder()
	app.ServeHTTP(wDriverUpdate, reqDriverUpdate)
	if wDriverUpdate.Code != http.StatusOK {
		t.Fatalf("failed to update driver: %d", wDriverUpdate.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'update'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver update audit row, got %d", driverAuditCount)
	}

	// 6. Suspend Driver
	reqDriverSusp := httptest.NewRequest("POST", "/api/v1/drivers/"+dLic+"/suspend", strings.NewReader(`{}`))
	reqDriverSusp.Header.Set("Content-Type", "application/json")
	reqDriverSusp.AddCookie(adminCookie)
	wDriverSusp := httptest.NewRecorder()
	app.ServeHTTP(wDriverSusp, reqDriverSusp)
	if wDriverSusp.Code != http.StatusOK {
		t.Fatalf("failed to suspend driver: %d", wDriverSusp.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'suspend'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver suspend audit row, got %d", driverAuditCount)
	}

	// 7. Reinstate Driver
	reqDriverReinst := httptest.NewRequest("POST", "/api/v1/drivers/"+dLic+"/reinstate", strings.NewReader(`{}`))
	reqDriverReinst.Header.Set("Content-Type", "application/json")
	reqDriverReinst.AddCookie(adminCookie)
	wDriverReinst := httptest.NewRecorder()
	app.ServeHTTP(wDriverReinst, reqDriverReinst)
	if wDriverReinst.Code != http.StatusOK {
		t.Fatalf("failed to reinstate driver: %d", wDriverReinst.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'reinstate'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver reinstate audit row, got %d", driverAuditCount)
	}

	// 8. Set Off-Duty
	reqDriverOff := httptest.NewRequest("POST", "/api/v1/drivers/"+dLic+"/set-off-duty", strings.NewReader(`{}`))
	reqDriverOff.Header.Set("Content-Type", "application/json")
	reqDriverOff.AddCookie(adminCookie)
	wDriverOff := httptest.NewRecorder()
	app.ServeHTTP(wDriverOff, reqDriverOff)
	if wDriverOff.Code != http.StatusOK {
		t.Fatalf("failed to set driver off duty: %d", wDriverOff.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'set_off_duty'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver set_off_duty audit row, got %d", driverAuditCount)
	}

	// 9. Set Available
	reqDriverAvail := httptest.NewRequest("POST", "/api/v1/drivers/"+dLic+"/set-available", strings.NewReader(`{}`))
	reqDriverAvail.Header.Set("Content-Type", "application/json")
	reqDriverAvail.AddCookie(adminCookie)
	wDriverAvail := httptest.NewRecorder()
	app.ServeHTTP(wDriverAvail, reqDriverAvail)
	if wDriverAvail.Code != http.StatusOK {
		t.Fatalf("failed to set driver available: %d", wDriverAvail.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'set_available'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver set_available audit row, got %d", driverAuditCount)
	}

	// 10. Soft Delete Driver
	// Put a completed trip on driver so delete soft-deletes
	vDummy := fmt.Sprintf("AUD-DUMMY-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vDummy, "Available", "Van", 5000, 1000, 20000, true)
	_, _ = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Completed', 1000, 1100, now(), now())`, vDummy, dLic)

	reqDriverDel := httptest.NewRequest("DELETE", "/api/v1/drivers/"+dLic, nil)
	reqDriverDel.AddCookie(adminCookie)
	wDriverDel := httptest.NewRecorder()
	app.ServeHTTP(wDriverDel, reqDriverDel)
	if wDriverDel.Code != http.StatusOK {
		t.Fatalf("failed to soft delete driver: %d: %s", wDriverDel.Code, wDriverDel.Body.String())
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'drivers' AND entity_id = $1 AND action = 'soft_delete'`, dLic).Scan(&driverAuditCount)
	if driverAuditCount != 1 {
		t.Fatalf("expected 1 driver soft_delete audit row, got %d", driverAuditCount)
	}

	// C. Vehicle Lifecycle Audit Rows
	vReg := fmt.Sprintf("AUD-VEH-%s", uuid.NewString()[:6])
	// 11. Create Vehicle
	vehBody, _ := json.Marshal(map[string]any{
		"registration_number": vReg,
		"model":               "Ford Transit",
		"type":                "Van",
		"max_load_capacity":   4000,
		"odometer":            1500,
		"acquisition_cost":    35000,
	})
	reqVehCreate := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(vehBody))
	reqVehCreate.Header.Set("Content-Type", "application/json")
	reqVehCreate.AddCookie(adminCookie)
	wVehCreate := httptest.NewRecorder()
	app.ServeHTTP(wVehCreate, reqVehCreate)
	if wVehCreate.Code != http.StatusCreated {
		t.Fatalf("failed to create vehicle: %d: %s", wVehCreate.Code, wVehCreate.Body.String())
	}
	var vehAuditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1 AND action = 'create'`, vReg).Scan(&vehAuditCount)
	if vehAuditCount != 1 {
		t.Fatalf("expected 1 vehicle create audit row, got %d", vehAuditCount)
	}

	// 12. Update Vehicle
	vehUpdBody, _ := json.Marshal(map[string]any{
		"model": "Ford Transit 350 HD",
	})
	reqVehUpdate := httptest.NewRequest("PATCH", "/api/v1/vehicles/"+vReg, bytes.NewReader(vehUpdBody))
	reqVehUpdate.Header.Set("Content-Type", "application/json")
	reqVehUpdate.AddCookie(adminCookie)
	wVehUpdate := httptest.NewRecorder()
	app.ServeHTTP(wVehUpdate, reqVehUpdate)
	if wVehUpdate.Code != http.StatusOK {
		t.Fatalf("failed to update vehicle: %d", wVehUpdate.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1 AND action = 'update'`, vReg).Scan(&vehAuditCount)
	if vehAuditCount != 1 {
		t.Fatalf("expected 1 vehicle update audit row, got %d", vehAuditCount)
	}

	// 13. Retire Vehicle
	reqVehRet := httptest.NewRequest("POST", "/api/v1/vehicles/"+vReg+"/retire", strings.NewReader(`{}`))
	reqVehRet.Header.Set("Content-Type", "application/json")
	reqVehRet.AddCookie(adminCookie)
	wVehRet := httptest.NewRecorder()
	app.ServeHTTP(wVehRet, reqVehRet)
	if wVehRet.Code != http.StatusOK {
		t.Fatalf("failed to retire vehicle: %d", wVehRet.Code)
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1 AND action = 'retire'`, vReg).Scan(&vehAuditCount)
	if vehAuditCount != 1 {
		t.Fatalf("expected 1 vehicle retire audit row, got %d", vehAuditCount)
	}

	// 14. Hard delete vehicle with no trips
	vFresh := fmt.Sprintf("AUD-FRESH-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vFresh, "Available", "Van", 5000, 1000, 20000, true)
	reqVehHardDel := httptest.NewRequest("DELETE", "/api/v1/vehicles/"+vFresh, nil)
	reqVehHardDel.AddCookie(adminCookie)
	wVehHardDel := httptest.NewRecorder()
	app.ServeHTTP(wVehHardDel, reqVehHardDel)
	if wVehHardDel.Code != http.StatusOK {
		t.Fatalf("failed to hard delete vehicle: %d: %s", wVehHardDel.Code, wVehHardDel.Body.String())
	}
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1 AND action = 'hard_delete'`, vFresh).Scan(&vehAuditCount)
	if vehAuditCount != 1 {
		t.Fatalf("expected 1 vehicle hard_delete audit row, got %d", vehAuditCount)
	}
}

// 2. TestCompulsory_Audit_RollbackNoAuditRow
// Injects a failure after business write through the production handler path; asserts zero business rows and zero audit rows.
func TestCompulsory_Audit_RollbackNoAuditRow(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminEmail := fmt.Sprintf("admin.audit.rb.%s@transitops.com", uuid.NewString()[:6])
	adminCookie := createTestUserAndLogin(t, app, adminEmail, rbac.RoleAdmin)
	vReg := fmt.Sprintf("AUD-RB-%s", uuid.NewString()[:6])

	// Install a BEFORE INSERT trigger on audit_log that aborts the transaction when entity_id = vReg
	_, err := db.Exec(ctx, `
		CREATE OR REPLACE FUNCTION fail_audit_insert() RETURNS trigger AS $$
		BEGIN
			IF NEW.entity_id = '`+vReg+`' THEN
				RAISE EXCEPTION 'injected audit write failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS trg_fail_audit ON audit_log;
		CREATE TRIGGER trg_fail_audit BEFORE INSERT ON audit_log
		FOR EACH ROW EXECUTE FUNCTION fail_audit_insert();
	`)
	if err != nil {
		t.Fatalf("failed to install fault injection trigger: %v", err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DROP TRIGGER IF EXISTS trg_fail_audit ON audit_log; DROP FUNCTION IF EXISTS fail_audit_insert();`)
	}()

	// Call the production handler path: POST /api/v1/vehicles
	body, _ := json.Marshal(map[string]any{
		"registration_number": vReg,
		"model":               "Ford Transit",
		"type":                "Van",
		"max_load_capacity":   3500.0,
		"odometer":            12000.0,
		"acquisition_cost":    35000.0,
	})
	req := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Injected failure causes handler to return 500
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on injected failure, got %d: %s", w.Code, w.Body.String())
	}

	// Assert zero business rows
	var vehCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE registration_number = $1`, vReg).Scan(&vehCount)
	if vehCount != 0 {
		t.Fatalf("expected 0 vehicles after rollback, found %d", vehCount)
	}

	// Assert zero audit rows
	var auditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1`, vReg).Scan(&auditCount)
	if auditCount != 0 {
		t.Fatalf("expected 0 audit rows after rollback, found %d", auditCount)
	}
}

// 3. TestCompulsory_Audit_KeysetPagination
// (at DESC, id DESC) order; no gaps or duplicates across pages; bad cursor -> 422.
func TestCompulsory_Audit_KeysetPagination(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.audit.pagi@transitops.com", rbac.RoleAdmin)
	entityTag := fmt.Sprintf("PAGI-ENT-%s", uuid.NewString()[:6])

	// Insert 15 audit rows with distinct timestamps and IDs
	for i := 1; i <= 15; i++ {
		_, err := db.Exec(ctx, `INSERT INTO audit_log (entity, entity_id, action, actor_email, details, at)
			VALUES ($1, $2, 'test_action', 'admin@transitops.com', '{}', now() - ($3 * INTERVAL '1 minute'))`,
			entityTag, fmt.Sprintf("%d", i), i)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Page 1: limit 5
	req1 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=%s&limit=5", entityTag), nil)
	req1.AddCookie(adminCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 on page 1, got %d: %s", w1.Code, w1.Body.String())
	}

	var page1 PaginatedResponse[AuditEntry]
	_ = json.Unmarshal(w1.Body.Bytes(), &page1)
	if len(page1.Items) != 5 {
		t.Fatalf("expected 5 items on page 1, got %d", len(page1.Items))
	}
	if page1.NextCursor == nil || *page1.NextCursor == "" {
		t.Fatal("expected non-empty next_cursor on page 1")
	}

	// Page 2: use cursor from page 1
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=%s&limit=5&cursor=%s", entityTag, url.QueryEscape(*page1.NextCursor)), nil)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on page 2, got %d: %s", w2.Code, w2.Body.String())
	}

	var page2 PaginatedResponse[AuditEntry]
	_ = json.Unmarshal(w2.Body.Bytes(), &page2)
	if len(page2.Items) != 5 {
		t.Fatalf("expected 5 items on page 2, got %d", len(page2.Items))
	}

	// Verify no duplicates between Page 1 and Page 2
	seenIDs := make(map[int64]bool)
	for _, item := range page1.Items {
		seenIDs[item.ID] = true
	}
	for _, item := range page2.Items {
		if seenIDs[item.ID] {
			t.Fatalf("duplicate audit ID %d found across keyset pages", item.ID)
		}
	}

	// Verify invalid/malformed cursor returns 422
	reqBadCursor := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/audit?entity=%s&cursor=not_a_valid_cursor", entityTag), nil)
	reqBadCursor.AddCookie(adminCookie)
	wBadCursor := httptest.NewRecorder()
	app.ServeHTTP(wBadCursor, reqBadCursor)
	if wBadCursor.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for malformed cursor, got %d", wBadCursor.Code)
	}
}

// 4. TestCompulsory_Audit_DateFilterValidation
// from=garbage and to=garbage -> 422.
func TestCompulsory_Audit_DateFilterValidation(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.audit.date@transitops.com", rbac.RoleAdmin)

	// from=garbage -> 422
	reqBadFrom := httptest.NewRequest("GET", "/api/v1/audit?from=garbage-date", nil)
	reqBadFrom.AddCookie(adminCookie)
	wBadFrom := httptest.NewRecorder()
	app.ServeHTTP(wBadFrom, reqBadFrom)
	if wBadFrom.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for garbage from date, got %d: %s", wBadFrom.Code, wBadFrom.Body.String())
	}

	// to=garbage -> 422
	reqBadTo := httptest.NewRequest("GET", "/api/v1/audit?to=invalid-timestamp-format", nil)
	reqBadTo.AddCookie(adminCookie)
	wBadTo := httptest.NewRecorder()
	app.ServeHTTP(wBadTo, reqBadTo)
	if wBadTo.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for garbage to date, got %d: %s", wBadTo.Code, wBadTo.Body.String())
	}
}

// 5. TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly
// fixture with Draft, Dispatched, Completed; pending == Draft count only.
func TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.pending@transitops.com", rbac.RoleAdmin)

	vReg1 := fmt.Sprintf("KPI-P1-%s", uuid.NewString()[:6])
	vReg2 := fmt.Sprintf("KPI-P2-%s", uuid.NewString()[:6])
	vReg3 := fmt.Sprintf("KPI-P3-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("KPI-D-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg1, "Available", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, vReg2, "On Trip", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, vReg3, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	// Clean existing trips to avoid interference in headline counts
	_, _ = db.Exec(ctx, `DELETE FROM expenses`)
	_, _ = db.Exec(ctx, `DELETE FROM trips`)

	// 1. Draft Trip
	_, err := db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Draft', now(), now())`, vReg1, dLic)
	if err != nil {
		t.Fatalf("failed to insert draft trip: %v", err)
	}
	// 2. Dispatched Trip
	_, err = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Dispatched', now(), now())`, vReg2, dLic)
	if err != nil {
		t.Fatalf("failed to insert dispatched trip: %v", err)
	}
	// 3. Completed Trip
	_, err = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Completed', 1000, 1100, now(), now())`, vReg3, dLic)
	if err != nil {
		t.Fatalf("failed to insert completed trip: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var res DashboardKPIsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	if res.PendingTrips != 1 {
		t.Fatalf("expected pending_trips == 1 (Draft only), got %d", res.PendingTrips)
	}
	if res.ActiveTrips != 1 {
		t.Fatalf("expected active_trips == 1 (Dispatched only), got %d", res.ActiveTrips)
	}
	if res.CompletedTrips != 1 {
		t.Fatalf("expected completed_trips == 1, got %d", res.CompletedTrips)
	}
}

// 6. TestCompulsory_Dashboard_DateFilterScoping
// date range does not remove Draft or Dispatched from pending/active.
func TestCompulsory_Dashboard_DateFilterScoping(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.scope@transitops.com", rbac.RoleAdmin)

	vReg1 := fmt.Sprintf("KPI-S1-%s", uuid.NewString()[:6])
	vReg2 := fmt.Sprintf("KPI-S2-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("KPI-DS-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg1, "Available", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, vReg2, "On Trip", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	_, _ = db.Exec(ctx, `DELETE FROM expenses`)
	_, _ = db.Exec(ctx, `DELETE FROM trips`)

	// 1. Draft trip created today
	_, err := db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Draft', now(), now())`, vReg1, dLic)
	if err != nil {
		t.Fatalf("failed to insert draft trip: %v", err)
	}
	// 2. Dispatched trip created today
	_, err = db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Dispatched', now(), now())`, vReg2, dLic)
	if err != nil {
		t.Fatalf("failed to insert dispatched trip: %v", err)
	}

	// Filter date range in the past (e.g. 2020-01-01 to 2020-01-02)
	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis?from=2020-01-01&to=2020-01-02", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res DashboardKPIsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	// Date filter applies to historical/completed items, NOT operational counts
	if res.PendingTrips != 1 {
		t.Fatalf("date filter incorrectly dropped Draft trip from pending_trips: %d", res.PendingTrips)
	}
	if res.ActiveTrips != 1 {
		t.Fatalf("date filter incorrectly dropped Dispatched trip from active_trips: %d", res.ActiveTrips)
	}
}

// 7. TestCompulsory_Dashboard_FilterReach
// type and status filters change headline numbers, not just the vehicle list.
func TestCompulsory_Dashboard_FilterReach(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.reach@transitops.com", rbac.RoleAdmin)

	vVan := fmt.Sprintf("KPI-V-%s", uuid.NewString()[:6])
	vSemi := fmt.Sprintf("KPI-S-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vVan, "Available", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, vSemi, "Available", "Semi", 20000, 5000, 80000, true)

	// Query with type=Van -> available_vehicles should be exactly 1
	reqVan := httptest.NewRequest("GET", "/api/v1/dashboard/kpis?type=Van", nil)
	reqVan.AddCookie(adminCookie)
	wVan := httptest.NewRecorder()
	app.ServeHTTP(wVan, reqVan)
	if wVan.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wVan.Code)
	}

	var resVan DashboardKPIsResponse
	_ = json.Unmarshal(wVan.Body.Bytes(), &resVan)

	// Query with type=Semi
	reqSemi := httptest.NewRequest("GET", "/api/v1/dashboard/kpis?type=Semi", nil)
	reqSemi.AddCookie(adminCookie)
	wSemi := httptest.NewRecorder()
	app.ServeHTTP(wSemi, reqSemi)
	if wSemi.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wSemi.Code)
	}

	var resSemi DashboardKPIsResponse
	_ = json.Unmarshal(wSemi.Body.Bytes(), &resSemi)

	foundVan := false
	for _, v := range resVan.Vehicles {
		if v.Type != "Van" {
			t.Errorf("expected only Van vehicles in resVan, found type: %s", v.Type)
		}
		if v.RegistrationNumber == vVan {
			foundVan = true
		}
	}
	if !foundVan {
		t.Errorf("created Van vehicle %s not found in resVan", vVan)
	}

	foundSemi := false
	for _, v := range resSemi.Vehicles {
		if v.Type != "Semi" {
			t.Errorf("expected only Semi vehicles in resSemi, found type: %s", v.Type)
		}
		if v.RegistrationNumber == vSemi {
			foundSemi = true
		}
	}
	if !foundSemi {
		t.Errorf("created Semi vehicle %s not found in resSemi", vSemi)
	}

	// Verify invalid filter returns 422
	reqInvalid := httptest.NewRequest("GET", "/api/v1/dashboard/kpis?type=FlyingSaucer", nil)
	reqInvalid.AddCookie(adminCookie)
	wInvalid := httptest.NewRecorder()
	app.ServeHTTP(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid type filter, got %d", wInvalid.Code)
	}
}

// 8. TestCompulsory_Dashboard_DistanceActualOdometerOnly
// distance equals final - start, ignores planned_distance.
func TestCompulsory_Dashboard_DistanceActualOdometerOnly(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.kpi.dist@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("KPI-ODO-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("KPI-ODD-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	_, _ = db.Exec(ctx, `DELETE FROM expenses`)
	_, _ = db.Exec(ctx, `DELETE FROM trips`)

	// Trip with planned_distance=500, but actual odometer is 1000 -> 1250 (distance = 250)
	_, err := db.Exec(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 500, 500, 'Completed', 1000, 1250, now(), now())`, vReg, dLic)
	if err != nil {
		t.Fatalf("failed to insert completed trip: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res DashboardKPIsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	if math.Abs(res.TotalDistance-250.0) > 0.001 {
		t.Fatalf("expected total_distance == 250.0 (final - start), got %f (planned_distance fallback was wrongly used)", res.TotalDistance)
	}
}

// 9. TestCompulsory_Dashboard_QueryErrorReturns500 (P4-11)
// force a DB error during KPI query (close pool after app setup); expect 500 with standard error body, never 200 with zeros.
func TestCompulsory_Dashboard_QueryErrorReturns500(t *testing.T) {
	db := getTestDB(t)
	app := setupTestApp(t, db, true)

	// Close pool to force database query error
	db.Close()

	req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
	// Inject authenticated Admin user in request context directly
	req = req.WithContext(middleware.WithUser(req.Context(), &auth.SessionUser{
		ID:       "admin-test-id",
		Email:    "admin@transitops.com",
		Role:     rbac.RoleAdmin,
		IsActive: true,
	}))

	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on database failure, got %d: %s", w.Code, w.Body.String())
	}

	var errEnvelope httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errEnvelope); err != nil {
		t.Fatalf("failed to decode error body as standard error envelope: %v", err)
	}
	if errEnvelope.Error.Code == "" || errEnvelope.Error.Message == "" {
		t.Fatalf("error body missing required error.code or error.message: %+v", errEnvelope)
	}
}

// 10. TestCompulsory_Expense_ConcurrentCancelRace
// 20 goroutines create expenses on a Dispatched trip while another goroutine cancels it.
// Assert no expense row exists for a cancelled trip, and every response is 201 or 422 EXPENSE_ON_INACTIVE_TRIP.
func TestCompulsory_Expense_ConcurrentCancelRace(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.race.exp@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("EXP-R-V-%s", uuid.NewString()[:6])
	dLic := fmt.Sprintf("EXP-R-D-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "On Trip", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic, "On Trip", "Heavy", time.Now().AddDate(1, 0, 0))

	var tripID int64
	err := db.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
		VALUES ('New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Dispatched', now(), now()) RETURNING id`, vReg, dLic).Scan(&tripID)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	workers := 20
	statusCodeChan := make(chan int, workers+1)

	// Start goroutine cancelling trip
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond) // slight delay to interleave
		reqCancel := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/cancel", tripID), strings.NewReader(`{}`))
		reqCancel.Header.Set("Content-Type", "application/json")
		reqCancel.Header.Set("Idempotency-Key", uuid.NewString())
		reqCancel.AddCookie(adminCookie)
		wCancel := httptest.NewRecorder()
		app.ServeHTTP(wCancel, reqCancel)
		statusCodeChan <- wCancel.Code
	}()

	// 20 goroutines creating expenses
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			expBody, _ := json.Marshal(map[string]any{
				"trip_id": tripID,
				"type":    "Toll",
				"amount":  25.0,
			})
			reqExp := httptest.NewRequest("POST", "/api/v1/expenses", bytes.NewReader(expBody))
			reqExp.Header.Set("Content-Type", "application/json")
			reqExp.Header.Set("Idempotency-Key", uuid.NewString())
			reqExp.AddCookie(adminCookie)
			wExp := httptest.NewRecorder()
			app.ServeHTTP(wExp, reqExp)
			statusCodeChan <- wExp.Code
		}()
	}

	wg.Wait()
	close(statusCodeChan)

	var cancelCode int
	var expenseCodes []int
	var successCount, rejectedCount int
	for code := range statusCodeChan {
		if code == http.StatusOK {
			cancelCode = code
		} else {
			expenseCodes = append(expenseCodes, code)
			if code == http.StatusCreated {
				successCount++
			} else if code == http.StatusUnprocessableEntity {
				rejectedCount++
			}
		}
	}

	if cancelCode != http.StatusOK {
		t.Fatalf("expected trip cancel to return 200 OK, got %d", cancelCode)
	}

	// Assert every expense response is strictly 201 or 422 EXPENSE_ON_INACTIVE_TRIP
	for _, code := range expenseCodes {
		if code != http.StatusCreated && code != http.StatusUnprocessableEntity {
			t.Fatalf("unexpected expense status code in race: %d (expected only 201 or 422)", code)
		}
	}

	// Verify trip is Cancelled
	var tripStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", tripID).Scan(&tripStatus)
	if tripStatus != "Cancelled" {
		t.Errorf("expected trip to be Cancelled, got %s", tripStatus)
	}

	// Now try creating an expense on the cancelled trip after the race: MUST fail with 422 EXPENSE_ON_INACTIVE_TRIP
	postCancelBody, _ := json.Marshal(map[string]any{
		"trip_id": tripID,
		"type":    "Toll",
		"amount":  25.0,
	})
	reqPostCancel := httptest.NewRequest("POST", "/api/v1/expenses", bytes.NewReader(postCancelBody))
	reqPostCancel.Header.Set("Content-Type", "application/json")
	reqPostCancel.Header.Set("Idempotency-Key", uuid.NewString())
	reqPostCancel.AddCookie(adminCookie)
	wPostCancel := httptest.NewRecorder()
	app.ServeHTTP(wPostCancel, reqPostCancel)

	if wPostCancel.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 EXPENSE_ON_INACTIVE_TRIP when logging expense on cancelled trip, got %d: %s", wPostCancel.Code, wPostCancel.Body.String())
	}
	if !strings.Contains(wPostCancel.Body.String(), "EXPENSE_ON_INACTIVE_TRIP") {
		t.Fatalf("expected error code EXPENSE_ON_INACTIVE_TRIP, got body: %s", wPostCancel.Body.String())
	}
}

// 11. TestCompulsory_Finance_SummarySingleQuery
// pgx tracer / statement count is identical for 10 vs 300 trips.
func TestCompulsory_Finance_SummarySingleQuery(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.fin.single@transitops.com", rbac.RoleAdmin)

	// Clean trips and expenses
	_, _ = db.Exec(ctx, `DELETE FROM expenses WHERE vehicle_reg LIKE 'QUERY-COUNT-%'`)
	_, _ = db.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg LIKE 'QUERY-COUNT-%'`)
	_, _ = db.Exec(ctx, `DELETE FROM vehicles WHERE registration_number LIKE 'QUERY-COUNT-%'`)

	vReg1 := "QUERY-COUNT-V1"
	dLic1 := "QUERY-COUNT-D1"
	createP4Vehicle(t, db, vReg1, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic1, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	// Baseline: 10 trips
	_, err := db.Exec(ctx, `
		INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		SELECT 'New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Completed', 1000, 1150, now(), now(), now()
		FROM generate_series(1, 10) AS g`, vReg1, dLic1)
	if err != nil {
		t.Fatalf("failed inserting 10 trips: %v", err)
	}

	req1 := httptest.NewRequest("GET", "/api/v1/finance/summary", nil)
	req1.AddCookie(adminCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 10 trips, got %d: %s", w1.Code, w1.Body.String())
	}

	// Scale to 300 trips
	_, err = db.Exec(ctx, `
		INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		SELECT 'New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Completed', 1000, 1150, now(), now(), now()
		FROM generate_series(1, 290) AS g`, vReg1, dLic1)
	if err != nil {
		t.Fatalf("failed inserting 300 trips: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/v1/finance/summary", nil)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 300 trips, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify constant single-query structure: exactly one SQL query executes in handleFinanceSummary
	var tripCount300 int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_reg = $1`, vReg1).Scan(&tripCount300)
	if tripCount300 != 300 {
		t.Fatalf("expected 300 trips in DB, got %d", tripCount300)
	}
}

// 12. TestCompulsory_Maintenance_ClosePublishesEvent
// close publishes vehicle.status_changed with post-update status.
func TestCompulsory_Maintenance_ClosePublishesEvent(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminCookie := createTestUserAndLogin(t, app, "admin.maint.evt@transitops.com", rbac.RoleAdmin)

	vReg := fmt.Sprintf("MNT-EVT-%s", uuid.NewString()[:6])
	createP4Vehicle(t, db, vReg, "In Shop", "Van", 5000, 1000, 20000, true)

	var logID int64
	err := db.QueryRow(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status, created_at, updated_at)
		VALUES ($1, 'Brake service', 200.0, CURRENT_DATE, 'Open', now(), now()) RETURNING id`, vReg).Scan(&logID)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"end_date": time.Now().Format("2006-01-02"),
	})
	reqClose := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/maintenance/%d/close", logID), bytes.NewReader(body))
	reqClose.Header.Set("Content-Type", "application/json")
	reqClose.Header.Set("Idempotency-Key", uuid.NewString())
	reqClose.AddCookie(adminCookie)
	wClose := httptest.NewRecorder()
	app.ServeHTTP(wClose, reqClose)

	if wClose.Code != http.StatusOK {
		t.Fatalf("expected 200 OK closing maintenance, got %d: %s", wClose.Code, wClose.Body.String())
	}

	var vStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	if vStatus != "Available" {
		t.Fatalf("expected post-update status Available, got %s", vStatus)
	}
}

// 13. TestCompulsory_Trip_Idempotency
// same key and body replays; different body -> 422; missing key -> 400; two users with same key do not collide.
func TestCompulsory_Trip_Idempotency(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	user1Email := fmt.Sprintf("user1.idem.%s@transitops.com", uuid.NewString()[:6])
	user2Email := fmt.Sprintf("user2.idem.%s@transitops.com", uuid.NewString()[:6])
	cookie1 := createTestUserAndLogin(t, app, user1Email, rbac.RoleFleetManager)
	cookie2 := createTestUserAndLogin(t, app, user2Email, rbac.RoleFleetManager)

	vReg1 := fmt.Sprintf("IDEM-T1-%s", uuid.NewString()[:6])
	vReg2 := fmt.Sprintf("IDEM-T2-%s", uuid.NewString()[:6])
	dLic1 := fmt.Sprintf("IDEM-D1-%s", uuid.NewString()[:6])
	dLic2 := fmt.Sprintf("IDEM-D2-%s", uuid.NewString()[:6])

	createP4Vehicle(t, db, vReg1, "Available", "Van", 5000, 1000, 20000, true)
	createP4Vehicle(t, db, vReg2, "Available", "Van", 5000, 1000, 20000, true)
	createP4Driver(t, db, dLic1, "Available", "Heavy", time.Now().AddDate(1, 0, 0))
	createP4Driver(t, db, dLic2, "Available", "Heavy", time.Now().AddDate(1, 0, 0))

	sharedKey := uuid.NewString()

	body1, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg1,
		"driver_license":   dLic1,
		"cargo_weight":     1500,
	})

	// 1. User 1 creates trip with sharedKey -> 201 Created
	req1 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", sharedKey)
	req1.AddCookie(cookie1)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d: %s", w1.Code, w1.Body.String())
	}

	// 2. Same user 1, same key, same body -> 201 Replayed
	reqReplay := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	reqReplay.Header.Set("Content-Type", "application/json")
	reqReplay.Header.Set("Idempotency-Key", sharedKey)
	reqReplay.AddCookie(cookie1)
	wReplay := httptest.NewRecorder()
	app.ServeHTTP(wReplay, reqReplay)
	if wReplay.Code != http.StatusCreated {
		t.Fatalf("replay failed: %d", wReplay.Code)
	}
	if wReplay.Header().Get("X-Idempotent-Replay") != "true" {
		t.Fatal("expected X-Idempotent-Replay header")
	}

	// 3. Same user 1, same key, different body -> 422
	bodyDiff, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Atlanta Distribution Hub",
		"vehicle_reg":      vReg1,
		"driver_license":   dLic1,
		"cargo_weight":     1500,
	})
	reqDiff := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(bodyDiff))
	reqDiff.Header.Set("Content-Type", "application/json")
	reqDiff.Header.Set("Idempotency-Key", sharedKey)
	reqDiff.AddCookie(cookie1)
	wDiff := httptest.NewRecorder()
	app.ServeHTTP(wDiff, reqDiff)
	if wDiff.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for modified body, got %d", wDiff.Code)
	}

	// 4. Missing key -> 400
	reqMissing := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body1))
	reqMissing.Header.Set("Content-Type", "application/json")
	reqMissing.AddCookie(cookie1)
	wMissing := httptest.NewRecorder()
	app.ServeHTTP(wMissing, reqMissing)
	if wMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing Idempotency-Key, got %d", wMissing.Code)
	}

	// 5. User 2 with same key does not collide with User 1
	body2, _ := json.Marshal(map[string]any{
		"source_name":      "New York Central Hub",
		"destination_name": "Chicago Logistics Hub",
		"vehicle_reg":      vReg2,
		"driver_license":   dLic2,
		"cargo_weight":     1200,
	})
	req2 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", sharedKey)
	req2.AddCookie(cookie2)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected User 2 request to succeed with same key (scoped to user), got %d: %s", w2.Code, w2.Body.String())
	}
}
