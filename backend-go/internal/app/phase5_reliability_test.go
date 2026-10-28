package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

// 1. TestCompulsory_RequestTimeout_503
// Uses the real router with middleware chain, hits slow route, asserts 503 REQUEST_TIMEOUT within 16s.
func TestCompulsory_RequestTimeout_503(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
	}
	app := New(db, cfg)

	// In App.buildHandler, default timeout is 15s.
	// For fast test execution, override timeout via helper or hit test endpoint
	// Note: /api/v1/test/slow sleeps 16s which exceeds 15s RequestTimeout
	req := httptest.NewRequest("GET", "/api/v1/test/slow", nil)
	req = req.WithContext(middleware.WithUser(req.Context(), &auth.SessionUser{
		ID:       "test-admin",
		Email:    "admin@transitops.com",
		Role:     "Admin",
		IsActive: true,
	}))
	w := httptest.NewRecorder()

	start := time.Now()
	app.ServeHTTP(w, req)
	duration := time.Since(start)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
	if duration > 16500*time.Millisecond {
		t.Fatalf("timeout took too long: %v", duration)
	}

	var errResp httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error envelope: %v", err)
	}
	if errResp.Error.Code != "REQUEST_TIMEOUT" {
		t.Errorf("expected error code REQUEST_TIMEOUT, got %s", errResp.Error.Code)
	}

	// Confirm the slow test route is NOT registered in the production router (ENV=production)
	prodCfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "production",
		CookieDomain:   "transitops.com",
	}
	prodApp := New(db, prodCfg)
	reqProd := httptest.NewRequest("GET", "/api/v1/test/slow", nil)
	reqProd = reqProd.WithContext(middleware.WithUser(reqProd.Context(), &auth.SessionUser{
		ID:       "test-admin",
		Email:    "admin@transitops.com",
		Role:     "Admin",
		IsActive: true,
	}))
	wProd := httptest.NewRecorder()
	prodApp.ServeHTTP(wProd, reqProd)
	if wProd.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for /api/v1/test/slow in production environment, got %d", wProd.Code)
	}
}

// 2. Test that a pg_sleep route returns 504 DB_TIMEOUT over HTTP
func TestCompulsory_StatementTimeout_504(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
	}
	app := New(db, cfg)

	req := httptest.NewRequest("GET", "/api/v1/test/db-timeout", nil)
	req = req.WithContext(middleware.WithUser(req.Context(), &auth.SessionUser{
		ID:       "test-admin",
		Email:    "admin@transitops.com",
		Role:     "Admin",
		IsActive: true,
	}))
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Gateway Timeout for pg_sleep statement timeout, got %d: %s", w.Code, w.Body.String())
	}

	var errResp httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if errResp.Error.Code != "DB_TIMEOUT" {
		t.Fatalf("expected code DB_TIMEOUT, got %s", errResp.Error.Code)
	}
}

// 3. TestCompulsory_ReadyzDatabaseBudget
// Decode body and assert exactly {"status":"ok"} on success and {"status":"unavailable"} on failure.
func TestCompulsory_ReadyzDatabaseBudget(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	// 1. Live database -> 200 OK with {"status":"ok"}
	appLive := setupTestApp(t, db, true)
	reqLive := httptest.NewRequest("GET", "/readyz", nil)
	wLive := httptest.NewRecorder()
	appLive.ServeHTTP(wLive, reqLive)

	if wLive.Code != http.StatusOK {
		t.Fatalf("expected 200 for live readyz, got %d", wLive.Code)
	}
	var bodyLive map[string]string
	if err := json.Unmarshal(wLive.Body.Bytes(), &bodyLive); err != nil {
		t.Fatalf("failed to decode live readyz body: %v", err)
	}
	expectedLive := map[string]string{"status": "ok"}
	if !reflect.DeepEqual(bodyLive, expectedLive) {
		t.Fatalf("expected body exactly %+v, got %+v", expectedLive, bodyLive)
	}

	// 2. Closed/nil database -> 503 Service Unavailable with {"status":"unavailable"}
	appDown := setupTestApp(t, nil, true)
	reqDown := httptest.NewRequest("GET", "/readyz", nil)
	wDown := httptest.NewRecorder()
	appDown.ServeHTTP(wDown, reqDown)

	if wDown.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for down readyz, got %d", wDown.Code)
	}
	var bodyDown map[string]string
	if err := json.Unmarshal(wDown.Body.Bytes(), &bodyDown); err != nil {
		t.Fatalf("failed to decode down readyz body: %v", err)
	}
	expectedDown := map[string]string{"status": "unavailable"}
	if !reflect.DeepEqual(bodyDown, expectedDown) {
		t.Fatalf("expected body exactly %+v, got %+v", expectedDown, bodyDown)
	}
}

// 4. TestCompulsory_GracefulShutdownInFlight
// Starts a real http.Server on the application's handler, sends a slow request,
// calls server.Shutdown(ctx), asserts the in-flight request returns 200 and a new request is refused.
func TestCompulsory_GracefulShutdownInFlight(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	app := setupTestApp(t, db, true)

	inFlightStarted := make(chan struct{})
	inFlightRelease := make(chan struct{})

	// Custom handler wrapper that signals when in-flight request is inside handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test-in-flight" {
			close(inFlightStarted)
			<-inFlightRelease
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		app.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind listener: %v", err)
	}
	addr := listener.Addr().String()

	server := &http.Server{
		Handler: handler,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	inFlightCh := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/test-in-flight")
		if err != nil {
			inFlightCh <- 0
			return
		}
		defer resp.Body.Close()
		inFlightCh <- resp.StatusCode
	}()

	// Wait for handler to explicitly signal request is in-flight (no sleeping)
	<-inFlightStarted

	// Trigger Shutdown while request is actively running inside handler
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- server.Shutdown(shutdownCtx)
	}()

	// Assert new requests started after Shutdown is called are refused
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, errNew := client.Get("http://" + addr + "/healthz")
	if errNew == nil {
		t.Fatal("expected new request after server.Shutdown to be refused, but succeeded")
	}

	// Release the in-flight request
	close(inFlightRelease)

	// Verify server.Shutdown completes cleanly
	if err := <-shutdownDone; err != nil {
		t.Fatalf("server.Shutdown failed: %v", err)
	}

	// Verify in-flight request succeeded with 200
	select {
	case code := <-inFlightCh:
		if code != http.StatusOK {
			t.Fatalf("expected 200 for in-flight request, got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-flight request")
	}
}
