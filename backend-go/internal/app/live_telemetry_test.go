package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func readMigrationFile(t *testing.T, name string) (string, error) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(findRepoRoot(t), "backend-go", "migrations", name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// applyMigration000006Up ensures the 000006 objects exist (idempotent).
func applyMigration000006Up(t *testing.T) {
	t.Helper()
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS vehicle_positions (
			id BIGSERIAL PRIMARY KEY,
			vehicle_reg TEXT NOT NULL REFERENCES vehicles(registration_number) ON DELETE RESTRICT,
			latitude DOUBLE PRECISION NOT NULL,
			longitude DOUBLE PRECISION NOT NULL,
			heading_deg DOUBLE PRECISION NOT NULL,
			speed_kmh DOUBLE PRECISION NOT NULL,
			recorded_at TIMESTAMPTZ NOT NULL,
			source TEXT NOT NULL CHECK (source IN ('simulator', 'device')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE INDEX IF NOT EXISTS idx_vehicle_positions_reg_recorded ON vehicle_positions(vehicle_reg, recorded_at DESC)`,
		`ALTER TABLE destinations ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION NULL`,
		`ALTER TABLE destinations ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION NULL`,
		`CREATE TABLE IF NOT EXISTS route_geometries (
			id BIGSERIAL PRIMARY KEY,
			origin TEXT NOT NULL,
			destination TEXT NOT NULL,
			polyline6 TEXT NOT NULL,
			distance_m DOUBLE PRECISION NOT NULL,
			duration_s DOUBLE PRECISION NOT NULL,
			provider TEXT NOT NULL,
			computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CONSTRAINT uq_route_geometries_pair UNIQUE (origin, destination))`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("migration 000006 up statement failed: %v", err)
		}
	}
}

func TestMigration000006_UpObjectsExist(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	for _, tbl := range []string{"vehicle_positions", "route_geometries"} {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT FROM information_schema.tables WHERE table_name = $1)`, tbl).Scan(&exists); err != nil || !exists {
			t.Fatalf("expected table %s to exist", tbl)
		}
	}
	var idxExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT FROM pg_indexes WHERE indexname = 'idx_vehicle_positions_reg_recorded')`).Scan(&idxExists); err != nil || !idxExists {
		t.Fatalf("expected index idx_vehicle_positions_reg_recorded to exist")
	}
	for _, col := range []string{"latitude", "longitude"} {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT FROM information_schema.columns WHERE table_name = 'destinations' AND column_name = $1)`, col).Scan(&exists); err != nil || !exists {
			t.Fatalf("expected destinations.%s column to exist", col)
		}
	}
	// Down must be exercised without damaging shared data: run it in a
	// transaction and roll back, verifying the drops take effect.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	down, err := readMigrationFile(t, "000006_live_tracking.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, down); err != nil {
		t.Fatalf("migration 000006 down failed: %v", err)
	}
	var gone bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT FROM information_schema.tables WHERE table_name = 'vehicle_positions')`).Scan(&gone); err != nil || gone {
		t.Fatalf("expected vehicle_positions to be dropped by down migration")
	}
}

func TestIngestPosition_ValidationTable(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	reg := fmt.Sprintf("VH-ING-%s", uuid.NewString()[:6])
	if _, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'M', 'Van', 1000, 0, 1000, 'Available', true)`, reg); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, reg)

	now := time.Now().UTC()
	cases := []struct {
		name  string
		input PositionInput
		code  string
	}{
		{"lat low", PositionInput{reg, -91, 0, 0, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"lat high", PositionInput{reg, 91, 0, 0, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"lon low", PositionInput{reg, 0, -181, 0, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"lon high", PositionInput{reg, 0, 181, 0, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"heading low", PositionInput{reg, 0, 0, -1, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"heading high", PositionInput{reg, 0, 0, 361, 10, now, "simulator", true}, "VALIDATION_FAILED"},
		{"speed high", PositionInput{reg, 0, 0, 0, 201, now, "simulator", true}, "VALIDATION_FAILED"},
		{"speed low", PositionInput{reg, 0, 0, 0, -1, now, "simulator", true}, "VALIDATION_FAILED"},
		{"too old", PositionInput{reg, 0, 0, 0, 10, now.Add(-11 * time.Minute), "simulator", true}, "VALIDATION_FAILED"},
		{"too future", PositionInput{reg, 0, 0, 0, 10, now.Add(61 * time.Second), "simulator", true}, "VALIDATION_FAILED"},
		{"bad source", PositionInput{reg, 0, 0, 0, 10, now, "gps", true}, "VALIDATION_FAILED"},
		{"device gated", PositionInput{reg, 0, 0, 0, 10, now, "device", false}, "DEVICE_INGEST_NOT_ENABLED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = IngestPosition(ctx, tx, tc.input)
			ingErr, ok := asIngestError(err)
			if !ok || ingErr.Code != tc.code {
				t.Fatalf("expected code %s, got err %v", tc.code, err)
			}
		})
	}

	// Boundaries accepted: exactly 10min old is rejected, just inside passes.
	tx, _ := pool.Begin(ctx)
	if _, err := IngestPosition(ctx, tx, PositionInput{reg, 41.8, -87.6, 0, 0, now.Add(-9 * time.Minute), "simulator", true}); err != nil {
		t.Fatalf("9-min-old point should pass: %v", err)
	}
	if _, err := IngestPosition(ctx, tx, PositionInput{reg, 41.8, -87.6, 360, 200, now.Add(59 * time.Second), "simulator", true}); err != nil {
		t.Fatalf("boundary point should pass: %v", err)
	}
	// Device source accepted with the service-level test flag.
	if _, err := IngestPosition(ctx, tx, PositionInput{reg, 41.8, -87.6, 90, 50, now, "device", true}); err != nil {
		t.Fatalf("device source with test flag should pass: %v", err)
	}
	tx.Rollback(ctx)
}

func asIngestError(err error) (*IngestError, bool) {
	if err == nil {
		return nil, false
	}
	var ingErr *IngestError
	if e, ok := err.(*IngestError); ok {
		ingErr = e
		return ingErr, true
	}
	return nil, false
}

func TestIngestPosition_DuplicateAndPurge(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	reg := fmt.Sprintf("VH-DUP-%s", uuid.NewString()[:6])
	if _, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'M', 'Van', 1000, 0, 1000, 'Available', true)`, reg); err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM vehicle_positions WHERE vehicle_reg = $1`, reg)
		pool.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, reg)
	}()

	now := time.Now().UTC().Truncate(time.Second)
	in := PositionInput{reg, 41.8, -87.6, 90, 50, now, "simulator", true}
	tx1, _ := pool.Begin(ctx)
	dup, err := IngestPosition(ctx, tx1, in)
	if err != nil || dup {
		t.Fatalf("first insert should succeed fresh: dup=%v err=%v", dup, err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx2, _ := pool.Begin(ctx)
	dup, err = IngestPosition(ctx, tx2, in)
	if err != nil || !dup {
		t.Fatalf("second insert should report duplicate: dup=%v err=%v", dup, err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vehicle_positions WHERE vehicle_reg = $1`, reg).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected exactly 1 row after duplicate, got %d (err %v)", count, err)
	}

	// Rows older than 48h are purged on insert.
	old := now.Add(-49 * time.Hour)
	if _, err := pool.Exec(ctx, `INSERT INTO vehicle_positions (vehicle_reg, latitude, longitude, heading_deg, speed_kmh, recorded_at, source)
		VALUES ($1, 0, 0, 0, 0, $2, 'simulator')`, reg, old); err != nil {
		t.Fatal(err)
	}
	fresh := PositionInput{reg, 41.8, -87.6, 90, 50, now.Add(time.Second), "simulator", true}
	tx3, _ := pool.Begin(ctx)
	if _, err := IngestPosition(ctx, tx3, fresh); err != nil {
		t.Fatal(err)
	}
	if err := tx3.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vehicle_positions WHERE vehicle_reg = $1 AND recorded_at < now() - INTERVAL '48 hours'`, reg).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expected old rows purged, %d remain (err %v)", count, err)
	}
}

func TestTelemetry_RoleMatrixAndDuplicate(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	app := setupTestApp(t, pool, true)
	ctx := context.Background()

	suffix := uuid.NewString()[:6]
	reg := fmt.Sprintf("VH-TEL-%s", suffix)
	if _, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
		VALUES ($1, 'M', 'Van', 1000, 0, 1000, 'Available', true)`, reg); err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM vehicle_positions WHERE vehicle_reg = $1`, reg)
		pool.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, reg)
	}()

	t.Setenv("SIM_ENABLED", "1")
	fmCookie := createTestUserAndLogin(t, app, fmt.Sprintf("fmtel-%s@transitops.com", suffix), rbac.RoleFleetManager)
	drvCookie := createTestUserAndLogin(t, app, fmt.Sprintf("drtel-%s@transitops.com", suffix), rbac.RoleDriver)

	post := func(cookie *http.Cookie, body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/telemetry/positions", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		return w
	}
	recAt := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	good := map[string]any{
		"vehicle_reg": reg, "latitude": 41.8, "longitude": -87.6,
		"heading_deg": 90, "speed_kmh": 50, "recorded_at": recAt, "source": "simulator",
	}

	// Driver is not in the role matrix: 403.
	if w := post(drvCookie, good); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for Driver post, got %d: %s", w.Code, w.Body.String())
	}
	// Fresh point accepted.
	if w := post(fmCookie, good); w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for fresh simulator point, got %d: %s", w.Code, w.Body.String())
	}
	// Duplicate reports 200 with duplicate=true and no new row.
	w := post(fmCookie, good)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for duplicate, got %d: %s", w.Code, w.Body.String())
	}
	var dupResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dupResp); err != nil || dupResp["duplicate"] != true {
		t.Fatalf("expected {duplicate:true}, got %s", w.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vehicle_positions WHERE vehicle_reg = $1`, reg).Scan(&count); err != nil || count != 1 {
		t.Fatalf("expected 1 row after duplicate, got %d", count)
	}

	// Device source is never accepted over HTTP.
	dev := map[string]any{
		"vehicle_reg": reg, "latitude": 41.8, "longitude": -87.6,
		"heading_deg": 90, "speed_kmh": 50,
		"recorded_at": time.Now().UTC().Format(time.RFC3339), "source": "device",
	}
	if w := post(fmCookie, dev); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for device source, got %d: %s", w.Code, w.Body.String())
	}
	var devResp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &devResp)
	_ = devResp

	// Simulator disabled: 403 SIMULATOR_DISABLED.
	t.Setenv("SIM_ENABLED", "")
	if w := post(fmCookie, good); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "SIMULATOR_DISABLED") {
		t.Fatalf("expected 403 SIMULATOR_DISABLED, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLiveVehicles_ScopeFreshnessETag(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	app := setupTestApp(t, pool, true)
	ctx := context.Background()
	suffix := uuid.NewString()[:6]

	// The test database has no seeded hubs; upsert the pair used below.
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES
		('Chicago Logistics Hub', 'Midwest', 41.8781, -87.6298),
		('Denver Intermodal Hub', 'Mountain', 39.7392, -104.9903)
		ON CONFLICT (name) DO UPDATE SET latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude`); err != nil {
		t.Fatal(err)
	}

	mkVehicle := func(reg string) {
		if _, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
			VALUES ($1, 'M', 'Van', 1000, 0, 1000, 'On Trip', true)`, reg); err != nil {
			t.Fatal(err)
		}
	}
	mkDriver := func(lic string) {
		if _, err := pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active)
			VALUES ($1, 'Live Driver', 'Van', '2029-01-01', '555-1', 90, 'On Trip', true)`, lic); err != nil {
			t.Fatal(err)
		}
	}
	mkTrip := func(src, dst, reg, lic string) int64 {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 100, 100, 100, 'Dispatched', now(), now()) RETURNING id`,
			src, dst, reg, lic).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	reg1 := fmt.Sprintf("VH-LV1-%s", suffix)
	reg2 := fmt.Sprintf("VH-LV2-%s", suffix)
	lic1 := fmt.Sprintf("DL-LV1-%s", suffix)
	lic2 := fmt.Sprintf("DL-LV2-%s", suffix)
	mkVehicle(reg1)
	mkVehicle(reg2)
	mkDriver(lic1)
	mkDriver(lic2)
	trip1 := mkTrip("Chicago Logistics Hub", "Denver Intermodal Hub", reg1, lic1)
	_ = mkTrip("Chicago Logistics Hub", "Denver Intermodal Hub", reg2, lic2)
	defer func() {
		pool.Exec(ctx, `DELETE FROM vehicle_positions WHERE vehicle_reg IN ($1, $2)`, reg1, reg2)
		pool.Exec(ctx, `DELETE FROM trips WHERE id IN ($1)`, trip1)
		pool.Exec(ctx, `DELETE FROM trips WHERE vehicle_reg IN ($1, $2)`, reg1, reg2)
		pool.Exec(ctx, `DELETE FROM vehicles WHERE registration_number IN ($1, $2)`, reg1, reg2)
		pool.Exec(ctx, `DELETE FROM drivers WHERE license_number IN ($1, $2)`, lic1, lic2)
	}()

	// reg1: fresh position; reg2: none.
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO vehicle_positions (vehicle_reg, latitude, longitude, heading_deg, speed_kmh, recorded_at, source)
		VALUES ($1, 41.8781, -87.6298, 270, 60, $2, 'simulator')`, reg1, now.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}

	finCookie := createTestUserAndLogin(t, app, fmt.Sprintf("finlive-%s@transitops.com", suffix), rbac.RoleFinancialAnalyst)
	get := func(cookie *http.Cookie, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/live/vehicles", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		return w
	}

	w := get(finCookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for live vehicles, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "driver_license") || strings.Contains(body, "driver_name") {
		t.Fatalf("live response must not contain driver fields: %s", body)
	}
	var resp struct {
		Items []liveVehicle `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 live items, got %d", len(resp.Items))
	}
	byReg := map[string]liveVehicle{}
	for _, it := range resp.Items {
		byReg[it.VehicleReg] = it
	}
	fresh := byReg[reg1]
	if fresh.Status != "fresh" {
		t.Fatalf("expected fresh status, got %s", fresh.Status)
	}
	if fresh.AgeSeconds == nil || *fresh.AgeSeconds > 60 {
		t.Fatalf("expected age_seconds < 60, got %+v", fresh.AgeSeconds)
	}
	if fresh.TripID == 0 || fresh.Origin.Name == "" || fresh.Destination.Name == "" {
		t.Fatalf("live item missing trip/origin/destination: %+v", fresh)
	}
	nopos := byReg[reg2]
	if nopos.Status != "no_position" {
		t.Fatalf("expected no_position status, got %s", nopos.Status)
	}
	if nopos.ProgressM != nil || nopos.ProgressPct != nil {
		t.Fatalf("expected null progress for no_position, got %+v", nopos)
	}

	// ETag: second request with If-None-Match -> 304.
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag header on live response")
	}
	if w2 := get(finCookie, etag); w2.Code != http.StatusNotModified {
		t.Fatalf("expected 304 for matching ETag, got %d", w2.Code)
	}

	// Driver scoping: lic1 driver sees only own trip.
	drvCookie := createDriverUserAndLogin(t, app, fmt.Sprintf("drlive-%s@transitops.com", suffix), lic1)
	w3 := get(drvCookie, "")
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for driver live view, got %d: %s", w3.Code, w3.Body.String())
	}
	var dresp struct {
		Items []liveVehicle `json:"items"`
	}
	if err := json.Unmarshal(w3.Body.Bytes(), &dresp); err != nil {
		t.Fatal(err)
	}
	if len(dresp.Items) != 1 || dresp.Items[0].VehicleReg != reg1 {
		t.Fatalf("expected driver to see only own trip, got %+v", dresp.Items)
	}
}
