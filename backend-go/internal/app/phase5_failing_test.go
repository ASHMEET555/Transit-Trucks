package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

func TestFailing_UnauthenticatedWebSocketAccepted_S05(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	// In Phase 5, connecting to /api/v1/ws without a valid ticket must return 401 Unauthorized before upgrade.
	req := httptest.NewRequest("GET", "/api/v1/ws", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for websocket without ticket, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFailing_MissingMetricsEndpoint(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	// In Phase 5, GET /metrics without Bearer token must return 404 (not revealing its existence).
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unauthenticated /metrics, got %d", w.Code)
	}
}

func TestFailing_LicenseCheckJobMissing(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	// In Phase 5, POST /api/v1/internal/jobs/license-check requires X-Job-Secret header.
	req := httptest.NewRequest("POST", "/api/v1/internal/jobs/license-check", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for job without secret, got %d", w.Code)
	}
}

func TestFailing_RealtimeTicketEndpointMissing(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	adminCookie := createTestUserAndLogin(t, app, "admin.p5.fail@transitops.com", rbac.RoleAdmin)

	// In Phase 5, POST /api/v1/realtime/ticket returns a single-use ticket.
	req := httptest.NewRequest("POST", "/api/v1/realtime/ticket", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("expected 200/201 for realtime ticket request, got %d: %s", w.Code, w.Body.String())
	}
}
