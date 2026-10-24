package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func TestPagination_Vehicles(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "page_admin@example.com", rbac.RoleAdmin)
	ctx := context.Background()

	prefix := fmt.Sprintf("PAGE-%s", uuid.NewString()[:4])
	for i := 1; i <= 5; i++ {
		reg := fmt.Sprintf("%s-VEH-%02d", prefix, i)
		_, err := pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active)
			VALUES ($1, 'Model V', 'Van', 1000, 100, 10000, 'Available', true)`, reg)
		if err != nil {
			t.Fatal(err)
		}
		defer func(r string) {
			_, _ = pool.Exec(ctx, "DELETE FROM vehicles WHERE registration_number = $1", r)
		}(reg)
	}

	// 1. Invalid cursor returns 400 INVALID_CURSOR
	reqBad := httptest.NewRequest("GET", "/api/v1/vehicles?cursor=!!!invalid-base64@@@", nil)
	reqBad.AddCookie(adminCookie)
	wBad := httptest.NewRecorder()
	app.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad cursor, got %d: %s", wBad.Code, wBad.Body.String())
	}
	var errResp map[string]any
	_ = json.Unmarshal(wBad.Body.Bytes(), &errResp)
	errObj := errResp["error"].(map[string]any)
	if errObj["code"] != "INVALID_CURSOR" {
		t.Errorf("expected error code INVALID_CURSOR, got %v", errObj["code"])
	}

	// 2. Page 1: limit = 2
	reqP1 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&limit=2", prefix), nil)
	reqP1.AddCookie(adminCookie)
	wP1 := httptest.NewRecorder()
	app.ServeHTTP(wP1, reqP1)
	if wP1.Code != http.StatusOK {
		t.Fatalf("page 1 failed: %d: %s", wP1.Code, wP1.Body.String())
	}

	var p1 PaginatedResponse[vehicle]
	if err := json.Unmarshal(wP1.Body.Bytes(), &p1); err != nil {
		t.Fatalf("unmarshal page 1 failed: %v", err)
	}
	if len(p1.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(p1.Items))
	}
	if p1.NextCursor == nil || *p1.NextCursor == "" {
		t.Fatalf("expected non-empty next_cursor on page 1")
	}

	// 3. Page 2: with cursor from page 1
	reqP2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&limit=2&cursor=%s", prefix, *p1.NextCursor), nil)
	reqP2.AddCookie(adminCookie)
	wP2 := httptest.NewRecorder()
	app.ServeHTTP(wP2, reqP2)
	if wP2.Code != http.StatusOK {
		t.Fatalf("page 2 failed: %d: %s", wP2.Code, wP2.Body.String())
	}

	var p2 PaginatedResponse[vehicle]
	if err := json.Unmarshal(wP2.Body.Bytes(), &p2); err != nil {
		t.Fatalf("unmarshal page 2 failed: %v", err)
	}
	if len(p2.Items) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(p2.Items))
	}
	if p2.Items[0].RegistrationNumber == p1.Items[1].RegistrationNumber {
		t.Fatalf("duplicate item found across pages: %s", p2.Items[0].RegistrationNumber)
	}
	if p2.NextCursor == nil {
		t.Fatalf("expected next_cursor on page 2")
	}

	// 4. Page 3: final item
	reqP3 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/vehicles?q=%s&limit=2&cursor=%s", prefix, *p2.NextCursor), nil)
	reqP3.AddCookie(adminCookie)
	wP3 := httptest.NewRecorder()
	app.ServeHTTP(wP3, reqP3)
	if wP3.Code != http.StatusOK {
		t.Fatalf("page 3 failed: %d: %s", wP3.Code, wP3.Body.String())
	}

	var p3 PaginatedResponse[vehicle]
	if err := json.Unmarshal(wP3.Body.Bytes(), &p3); err != nil {
		t.Fatalf("unmarshal page 3 failed: %v", err)
	}
	if len(p3.Items) != 1 {
		t.Fatalf("expected 1 item on page 3, got %d", len(p3.Items))
	}
	if p3.NextCursor != nil {
		t.Fatalf("expected next_cursor to be nil on last page, got %v", *p3.NextCursor)
	}
}

func TestPagination_LimitClamping(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "limit_admin@example.com", rbac.RoleAdmin)

	// Test limit > 100 is clamped and returns without error
	req := httptest.NewRequest("GET", "/api/v1/vehicles?limit=500", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with clamped limit, got %d: %s", w.Code, w.Body.String())
	}
}
