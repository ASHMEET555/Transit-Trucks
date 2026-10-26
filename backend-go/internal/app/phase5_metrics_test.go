package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompulsory_ProtectedMetricsEndpoint(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "production",
		CookieDomain:   "transitops.com",
		MetricsToken:   "super-secret-metrics-token-12345",
	}
	app := New(db, cfg)

	// 1. Unauthenticated request -> 404 Not Found (does not reveal endpoint existence)
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unauthenticated /metrics request, got %d", w.Code)
	}

	// 2. Request with invalid Bearer token -> 404 Not Found
	reqInvalid := httptest.NewRequest("GET", "/metrics", nil)
	reqInvalid.Header.Set("Authorization", "Bearer wrong-token")
	wInvalid := httptest.NewRecorder()
	app.ServeHTTP(wInvalid, reqInvalid)

	if wInvalid.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for invalid Bearer token, got %d", wInvalid.Code)
	}

	// Make a few standard requests to generate metrics
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	app.ServeHTTP(httptest.NewRecorder(), reqHealth)

	// 3. Request with valid Bearer token -> 200 OK with Prometheus format
	reqValid := httptest.NewRequest("GET", "/metrics", nil)
	reqValid.Header.Set("Authorization", "Bearer super-secret-metrics-token-12345")
	wValid := httptest.NewRecorder()
	app.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated /metrics, got %d: %s", wValid.Code, wValid.Body.String())
	}

	ct := wValid.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") && !strings.Contains(ct, "version=0.0.4") {
		t.Errorf("expected prometheus content type, got: %s", ct)
	}

	body := wValid.Body.String()

	// Assert essential Prometheus metric families are present
	requiredMetrics := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_in_flight_requests",
		"db_pool_total_connections",
		"db_pool_connections",
		"ws_connections",
		"events_published_total",
		"events_delivered_total",
		"events_dropped_total",
	}
	for _, m := range requiredMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("missing expected metric family %q in /metrics output", m)
		}
	}
}

func TestCompulsory_BoundedRouteLabelsInMetrics(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
		MetricsToken:   "metrics-token-test",
	}
	app := New(db, cfg)

	// Hit a parameterized route with sensitive path parameter
	reqDriver := httptest.NewRequest("GET", "/api/v1/drivers/DL-LEAK-TEST-999", nil)
	app.ServeHTTP(httptest.NewRecorder(), reqDriver)

	// Hit 50 random unmatched routes with high cardinality random paths
	for i := 0; i < 50; i++ {
		randomPath := fmt.Sprintf("/random-non-existent-path-%d", i)
		req := httptest.NewRequest("GET", randomPath, nil)
		app.ServeHTTP(httptest.NewRecorder(), req)
	}

	// Fetch /metrics
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	reqMetrics.Header.Set("Authorization", "Bearer metrics-token-test")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, reqMetrics)

	body := w.Body.String()

	// High cardinality random paths and dynamic IDs must NOT appear as route label values
	if strings.Contains(body, "random-non-existent-path") {
		t.Fatalf("high-cardinality path leaked into route labels! Body: %s", body)
	}
	if strings.Contains(body, "DL-LEAK-TEST-999") {
		t.Fatalf("sensitive path parameter leaked into route labels! Body: %s", body)
	}

	// Dynamic path must be parameterized
	if !strings.Contains(body, `route="/api/v1/drivers/{license}"`) {
		t.Fatalf("expected parameterized route pattern /api/v1/drivers/{license}, got: %s", body)
	}

	// Unmatched routes must be collapsed to route="unmatched"
	if !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("expected unmatched requests to be bounded to route=\"unmatched\", got: %s", body)
	}
}
