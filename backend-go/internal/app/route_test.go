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

func TestOSRMClientFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "osrm_route.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	// Chicago -> Denver: assert lon,lat order in the request path.
	res, err := FetchRoute(context.Background(), srv.URL, 41.8781, -87.6298, 39.7392, -104.9903)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "-87.629800,41.878100;-104.990300,39.739200") {
		t.Fatalf("expected lon,lat order in OSRM path, got %s", gotPath)
	}
	if !strings.Contains(gotPath, "geometries=polyline6") {
		t.Fatalf("expected polyline6 geometries param, got %s", gotPath)
	}
	if res.Provider != "osrm" {
		t.Fatalf("expected provider osrm, got %s", res.Provider)
	}
	if res.DistanceM <= 0 || res.DurationS <= 0 || res.Polyline6 == "" {
		t.Fatalf("expected populated route result: %+v", res)
	}
	if _, err := DecodePolyline6(res.Polyline6); err != nil {
		t.Fatalf("fixture geometry must decode: %v", err)
	}

	// Non-200 from OSRM surfaces an error.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()
	if _, err := FetchRoute(context.Background(), bad.URL, 41.8781, -87.6298, 39.7392, -104.9903); err == nil {
		t.Fatal("expected error on OSRM 503")
	}
}

func TestEnsureRouteCacheHit(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	suffix := uuid.NewString()[:6]
	hubA := fmt.Sprintf("Test Hub A %s", suffix)
	hubB := fmt.Sprintf("Test Hub B %s", suffix)
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 41.8781, -87.6298)`, hubA); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 39.7392, -104.9903)`, hubB); err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB)
		pool.Exec(ctx, `DELETE FROM destinations WHERE name IN ($1, $2)`, hubA, hubB)
	}()

	raw, err := os.ReadFile(filepath.Join("testdata", "osrm_route.json"))
	if err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	t.Setenv("OSRM_URL", srv.URL)

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := EnsureRoute(ctx, tx1, hubA, hubB)
	if err != nil {
		tx1.Rollback(ctx)
		t.Fatal(err)
	}
	if cached {
		tx1.Rollback(ctx)
		t.Fatal("first dispatch of pair must be a cache miss")
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cached, err = EnsureRoute(ctx, tx2, hubA, hubB)
	if err != nil {
		tx2.Rollback(ctx)
		t.Fatal(err)
	}
	if !cached {
		tx2.Rollback(ctx)
		t.Fatal("second dispatch of pair must be a cache hit")
	}
	tx2.Rollback(ctx)

	if hits != 1 {
		t.Fatalf("expected exactly 1 OSRM call across two dispatches, got %d", hits)
	}
	var provider string
	if err := pool.QueryRow(ctx, `SELECT provider FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB).Scan(&provider); err != nil || provider != "osrm" {
		t.Fatalf("expected cached provider osrm, got %q (err %v)", provider, err)
	}
}

func TestEnsureRouteApproximateFallback(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	suffix := uuid.NewString()[:6]
	hubA := fmt.Sprintf("Test Hub C %s", suffix)
	hubB := fmt.Sprintf("Test Hub D %s", suffix)
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 41.8781, -87.6298)`, hubA); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 39.7392, -104.9903)`, hubB); err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB)
		pool.Exec(ctx, `DELETE FROM destinations WHERE name IN ($1, $2)`, hubA, hubB)
	}()

	// Unreachable OSRM: closed port fails fast.
	t.Setenv("OSRM_URL", "http://127.0.0.1:1")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cached, err := EnsureRoute(ctx, tx, hubA, hubB)
	if err != nil {
		t.Fatal(err)
	}
	if cached {
		t.Fatal("fallback path must report a miss on first compute")
	}
	var provider, poly string
	if err := tx.QueryRow(ctx, `SELECT provider, polyline6 FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB).Scan(&provider, &poly); err != nil {
		t.Fatal(err)
	}
	if provider != "approximate" {
		t.Fatalf("expected provider approximate, got %q", provider)
	}
	pts, err := DecodePolyline6(poly)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("approximate route must be a straight 2-point line for dashed UI, got %d points", len(pts))
	}
}

func TestDispatchCachesRoutePair(t *testing.T) {
	applyMigration000006Up(t)
	pool := getTestDB(t)
	defer pool.Close()
	app := setupTestApp(t, pool, true)
	ctx := context.Background()
	suffix := uuid.NewString()[:6]

	hubA := fmt.Sprintf("Test Hub E %s", suffix)
	hubB := fmt.Sprintf("Test Hub F %s", suffix)
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 41.8781, -87.6298)`, hubA); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, 'Test', 39.7392, -104.9903)`, hubB); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "osrm_route.json"))
	if err != nil {
		t.Fatal(err)
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	t.Setenv("OSRM_URL", srv.URL)

	fmCookie := createTestUserAndLogin(t, app, fmt.Sprintf("fmosrm-%s@transitops.com", suffix), rbac.RoleFleetManager)
	expiry := time.Now().AddDate(2, 0, 0)
	var tripIDs []int64
	for i := 0; i < 2; i++ {
		reg := fmt.Sprintf("VH-OSRM-%s-%d", suffix, i)
		lic := fmt.Sprintf("DL-OSRM-%s-%d", suffix, i)
		createP4Vehicle(t, pool, reg, "Available", "Van", 5000, 1000, 20000, true)
		createP4Driver(t, pool, lic, "Available", "Heavy", expiry)
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 100, 100, 100, 'Draft', now(), now()) RETURNING id`,
			hubA, hubB, reg, lic).Scan(&id); err != nil {
			t.Fatal(err)
		}
		tripIDs = append(tripIDs, id)
		defer pool.Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
		defer pool.Exec(ctx, `DELETE FROM vehicles WHERE registration_number = $1`, reg)
		defer pool.Exec(ctx, `DELETE FROM drivers WHERE license_number = $1`, lic)
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB)
		pool.Exec(ctx, `DELETE FROM destinations WHERE name IN ($1, $2)`, hubA, hubB)
	}()

	for _, id := range tripIDs {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/dispatch", id), bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.AddCookie(fmCookie)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("dispatch %d failed: %d %s", id, w.Code, w.Body.String())
		}
		var resp trip
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Status != "Dispatched" {
			t.Fatalf("expected Dispatched, got %s", resp.Status)
		}
	}

	if hits != 1 {
		t.Fatalf("expected exactly 1 OSRM call for two dispatches of the same pair, got %d", hits)
	}
	var provider string
	var count int
	if err := pool.QueryRow(ctx, `SELECT provider, count(*) OVER () FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, hubA, hubB).Scan(&provider, &count); err != nil {
		t.Fatal(err)
	}
	if provider != "osrm" || count != 1 {
		t.Fatalf("expected one cached osrm row, got provider=%q count=%d", provider, count)
	}
}
