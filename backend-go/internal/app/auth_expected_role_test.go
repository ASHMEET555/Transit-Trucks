package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_ExpectedRoleAndValidation(t *testing.T) {
	pool := getTestDB(t)
	if pool == nil {
		t.Skip("database unavailable")
	}

	app := setupTestApp(t, pool, true)

	// 1. Unknown field returns 422
	{
		body := map[string]any{
			"email":         "fleet@transitops.com",
			"password":      "truck@555",
			"unknown_field": "disallowed",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for unknown field, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 2. Invalid expected_role returns 422
	{
		body := map[string]any{
			"email":         "fleet@transitops.com",
			"password":      "truck@555",
			"expected_role": "SuperAdmin",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for invalid expected_role, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 3. No expected_role returns 200 (backward compatibility)
	{
		body := map[string]any{
			"email":     "fleet@transitops.com",
			"password":  "truck@555",
			"workspace": "us-east-1",
			"company":   "acme-freight",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for login without expected_role, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 4. Correct expected_role returns 200
	{
		body := map[string]any{
			"email":         "fleet@transitops.com",
			"password":      "truck@555",
			"expected_role": "Fleet Manager",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for correct expected_role, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 5. Wrong expected_role returns 403 ROLE_MISMATCH
	{
		body := map[string]any{
			"email":         "fleet@transitops.com",
			"password":      "truck@555",
			"expected_role": "Driver",
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for role mismatch, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Error.Code != "ROLE_MISMATCH" {
			t.Fatalf("expected error code ROLE_MISMATCH, got %s", resp.Error.Code)
		}
	}
}
