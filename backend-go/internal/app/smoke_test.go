package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/auth"
	"github.com/google/uuid"
)

func TestManualSmokeScenario(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	adminEmail := "admin.smoke@transitops.com"
	adminPw := "AdminSmokePassword123!"

	// Ensure the smoke admin exists
	hash, err := auth.HashPassword(adminPw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin', true, now(), now())
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, is_active = true`,
		uuid.NewString(), adminEmail, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, adminEmail)
		_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, adminEmail)
	}()

	// 1. Log in with admin
	loginBody, _ := json.Marshal(map[string]string{
		"email":    adminEmail,
		"password": adminPw,
	})
	rLogin := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	rLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	app.ServeHTTP(wLogin, rLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("smoke login failed: status=%d, body=%s", wLogin.Code, wLogin.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("session cookie missing from login response")
	}

	// 2. Create second user via POST /api/v1/users
	newUserEmail := "dispatcher.smoke@transitops.com"
	createBody, _ := json.Marshal(map[string]string{
		"email":    newUserEmail,
		"password": "DispatcherPassword123!",
		"role":     "Fleet Manager",
	})
	rCreate := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(createBody))
	rCreate.Header.Set("Content-Type", "application/json")
	rCreate.AddCookie(sessionCookie)
	wCreate := httptest.NewRecorder()
	app.ServeHTTP(wCreate, rCreate)

	if wCreate.Code != http.StatusCreated && wCreate.Code != http.StatusOK {
		t.Fatalf("smoke create user failed: status=%d, body=%s", wCreate.Code, wCreate.Body.String())
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, newUserEmail)
	}()

	// 3. Log out via POST /api/v1/auth/logout
	rLogout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rLogout.AddCookie(sessionCookie)
	wLogout := httptest.NewRecorder()
	app.ServeHTTP(wLogout, rLogout)

	if wLogout.Code != http.StatusOK {
		t.Fatalf("smoke logout failed: status=%d, body=%s", wLogout.Code, wLogout.Body.String())
	}

	// 4. Confirm the old cookie returns 401
	rMeOld := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rMeOld.AddCookie(sessionCookie)
	wMeOld := httptest.NewRecorder()
	app.ServeHTTP(wMeOld, rMeOld)

	if wMeOld.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 using revoked cookie, got %d", wMeOld.Code)
	}

	// 5. Confirm POST /api/v1/auth/register returns 404
	rReg := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader([]byte(`{}`)))
	wReg := httptest.NewRecorder()
	app.ServeHTTP(wReg, rReg)

	if wReg.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for POST /api/v1/auth/register, got %d", wReg.Code)
	}
}
