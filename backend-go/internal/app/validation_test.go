package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
)

func createTestUserAndLogin(t *testing.T, app *App, email, role string) *http.Cookie {
	ctx := context.Background()
	pw := "StrongPass123!"
	hashed, _ := auth.HashPassword(pw)
	userId := uuid.NewString()
	_, err := app.db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, now(), now()) ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role, is_active = true`,
		userId, email, hashed, role)
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": pw})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s: %d %s", email, w.Code, w.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "session_token" || c.Name == "transitops_session" || strings.Contains(c.Name, "session") {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie not found after login")
	}
	return sessionCookie
}

func TestValidation_UnknownFieldsRejected(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)

	adminCookie := createTestUserAndLogin(t, app, "val_admin@example.com", rbac.RoleAdmin)

	body := map[string]any{
		"registration_number": "TEST-REG-VAL-1",
		"model":               "Ford Transit",
		"type":                "Van",
		"max_load_capacity":   1500.0,
		"odometer":            10000.0,
		"acquisition_cost":    30000.0,
		"banana":              "unexpected",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/fleet/vehicles", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unknown field, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidation_InvalidEnumsAndDecimals(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "val_admin2@example.com", rbac.RoleAdmin)

	tests := []struct {
		name       string
		path       string
		payload    map[string]any
		wantStatus int
	}{
		{
			name: "invalid vehicle type Banana",
			path: "/api/v1/fleet/vehicles",
			payload: map[string]any{
				"registration_number": "TEST-REG-VAL-2",
				"model":               "Ford Transit",
				"type":                "Banana",
				"max_load_capacity":   1000.0,
				"odometer":            5000.0,
				"acquisition_cost":    20000.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "negative vehicle odometer",
			path: "/api/v1/fleet/vehicles",
			payload: map[string]any{
				"registration_number": "TEST-REG-VAL-3",
				"model":               "Ford Transit",
				"type":                "Van",
				"max_load_capacity":   1000.0,
				"odometer":            -50.0,
				"acquisition_cost":    20000.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "negative acquisition cost",
			path: "/api/v1/fleet/vehicles",
			payload: map[string]any{
				"registration_number": "TEST-REG-VAL-4",
				"model":               "Ford Transit",
				"type":                "Van",
				"max_load_capacity":   1000.0,
				"odometer":            100.0,
				"acquisition_cost":    -1000.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "zero max load capacity",
			path: "/api/v1/fleet/vehicles",
			payload: map[string]any{
				"registration_number": "TEST-REG-VAL-5",
				"model":               "Ford Transit",
				"type":                "Van",
				"max_load_capacity":   0.0,
				"odometer":            100.0,
				"acquisition_cost":    1000.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "driver invalid expiry date format",
			path: "/api/v1/fleet/drivers",
			payload: map[string]any{
				"license_number":      "DL-VAL-1",
				"name":                "John Doe",
				"license_category":    "Heavy",
				"license_expiry_date": "not-a-date",
				"contact_number":      "555-0100",
				"safety_score":        95.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "driver safety score above 100",
			path: "/api/v1/fleet/drivers",
			payload: map[string]any{
				"license_number":      "DL-VAL-2",
				"name":                "John Doe",
				"license_category":    "Heavy",
				"license_expiry_date": "2027-01-01",
				"contact_number":      "555-0100",
				"safety_score":        105.0,
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest("POST", tc.path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(adminCookie)
			w := httptest.NewRecorder()

			app.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("[%s] expected %d, got %d: %s", tc.name, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestValidation_DuplicateRegistrationCodes(t *testing.T) {
	pool := getTestDB(t)
	app := setupTestApp(t, pool, true)
	adminCookie := createTestUserAndLogin(t, app, "val_admin3@example.com", rbac.RoleAdmin)

	reg := fmt.Sprintf("TEST-DUP-%s", uuid.NewString()[:6])
	_, _ = pool.Exec(context.Background(), "DELETE FROM vehicles WHERE registration_number = $1", reg)

	vBody := map[string]any{
		"registration_number": reg,
		"model":               "Ford Transit",
		"type":                "Van",
		"max_load_capacity":   1500.0,
		"odometer":            1000.0,
		"acquisition_cost":    25000.0,
	}
	b, _ := json.Marshal(vBody)

	// First create succeeds
	req1 := httptest.NewRequest("POST", "/api/v1/fleet/vehicles", bytes.NewReader(b))
	req1.Header.Set("Content-Type", "application/json")
	req1.AddCookie(adminCookie)
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first vehicle creation failed: %d: %s", w1.Code, w1.Body.String())
	}

	// Second create fails with 409 VEHICLE_EXISTS
	req2 := httptest.NewRequest("POST", "/api/v1/fleet/vehicles", bytes.NewReader(b))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate vehicle, got %d: %s", w2.Code, w2.Body.String())
	}
	var errResp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &errResp)
	errObj := errResp["error"].(map[string]any)
	if errObj["code"] != "VEHICLE_EXISTS" {
		t.Errorf("expected error code VEHICLE_EXISTS, got %v", errObj["code"])
	}
}
