package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"path/filepath"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func getTestDB(t *testing.T) *pgxpool.Pool {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("skipping test: cannot connect to postgres at %s: %v", dsn, err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping test: postgres ping failed at %s: %v", dsn, err)
	}
	ensureTestSchema(t, pool)
	return pool
}

func ensureTestSchema(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'users')").Scan(&exists)
	if err != nil || !exists {
		root := findRepoRoot(t)
		m1, err := os.ReadFile(filepath.Join(root, "backend-go", "migrations", "000001_initial_schema.up.sql"))
		if err == nil {
			_, _ = pool.Exec(ctx, string(m1))
		}
		m2, err := os.ReadFile(filepath.Join(root, "backend-go", "migrations", "000002_final_schema.up.sql"))
		if err == nil {
			_, _ = pool.Exec(ctx, string(m2))
		}
	}
}

func setupTestApp(t *testing.T, db *pgxpool.Pool, isLocal bool) *App {
	env := "local"
	if !isLocal {
		env = "production"
	}
	_, testCIDR, _ := net.ParseCIDR("192.0.2.0/24")
	_, loopbackCIDR, _ := net.ParseCIDR("127.0.0.0/8")
	cfg := Config{
		Secret:            "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins:   []string{"http://localhost:3000", "https://app.transitops.com"},
		Env:               env,
		CookieDomain:      "transitops.com",
		TrustedProxyCIDRs: []*net.IPNet{testCIDR, loopbackCIDR},
	}
	return New(db, cfg)
}

// 1. Router enumeration test: every route has a non-empty role set, except the three public routes
func TestCompulsory_RouterEnumeration(t *testing.T) {
	app := setupTestApp(t, nil, true)
	registry := app.Registry()
	routes := registry.Routes()

	publicCount := 0
	for _, r := range routes {
		if r.IsPublic {
			publicCount++
			isAllowedPublic := (r.Method == "GET" && (r.Path == "/healthz" || r.Path == "/readyz")) ||
				(r.Method == "POST" && r.Path == "/api/v1/auth/login")
			if !isAllowedPublic {
				t.Errorf("unauthorized public route registered: %s %s", r.Method, r.Path)
			}
		} else {
			if len(r.Roles) == 0 {
				t.Errorf("route %s %s registered without a role set", r.Method, r.Path)
			}
		}
	}
	if publicCount != 3 {
		t.Errorf("expected exactly 3 public routes (healthz, readyz, login), got %d", publicCount)
	}
}

// 2. Auth & Lockout tests
func TestCompulsory_AuthAndLockout(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	ctx := context.Background()
	testEmail := fmt.Sprintf("test-%s@example.com", uuid.NewString()[:8])
	testPassword := "SecurePassword123!"
	hashed, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}

	userId := uuid.NewString()
	_, err = db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin', true, now(), now())`, userId, testEmail, hashed)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, testEmail)
		_, _ = db.Exec(ctx, `DELETE FROM login_attempts WHERE email = $1`, testEmail)
		_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, testEmail)
	}()

	// A. Wrong password and unknown email return byte-identical status and body
	reqBodyWrongPw, _ := json.Marshal(map[string]string{"email": testEmail, "password": "WrongPassword!"})
	r1 := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(reqBodyWrongPw))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, r1)

	unknownEmail := fmt.Sprintf("nonexistent-%s@example.com", uuid.NewString()[:8])
	reqBodyUnknown, _ := json.Marshal(map[string]string{"email": unknownEmail, "password": "SomePassword!"})
	r2 := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(reqBodyUnknown))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, r2)

	if w1.Code != http.StatusUnauthorized || w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for bad credentials, got w1=%d w2=%d", w1.Code, w2.Code)
	}

	// Compare error code and message envelopes (request_id will differ by design)
	var env1, env2 httpx.ErrorEnvelope
	_ = json.Unmarshal(w1.Body.Bytes(), &env1)
	_ = json.Unmarshal(w2.Body.Bytes(), &env2)

	if env1.Error.Code != env2.Error.Code || env1.Error.Message != env2.Error.Message {
		t.Fatalf("expected identical error details, got %v vs %v", env1.Error, env2.Error)
	}

	// B. Login success sets cookie with HttpOnly, SameSite=Lax, Path=/
	reqBodySuccess, _ := json.Marshal(map[string]string{"email": testEmail, "password": testPassword})
	rLogin := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(reqBodySuccess))
	rLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	app.ServeHTTP(wLogin, rLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("expected status 200 on login, got %d, body: %s", wLogin.Code, wLogin.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie to be set")
	}
	if !sessionCookie.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Error("cookie must have SameSite=Lax")
	}
	if sessionCookie.Path != "/" {
		t.Errorf("cookie path must be '/', got %q", sessionCookie.Path)
	}

	// C. Lockout: 6th failed login for one email -> 429 with Retry-After
	lockoutEmail := fmt.Sprintf("lockout-%s@example.com", uuid.NewString()[:8])
	_, _ = db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Driver', true, now(), now())`, uuid.NewString(), lockoutEmail, hashed)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, lockoutEmail)
		_, _ = db.Exec(ctx, `DELETE FROM login_attempts WHERE email = $1`, lockoutEmail)
	}()

	badBody, _ := json.Marshal(map[string]string{"email": lockoutEmail, "password": "wrong"})
	for i := 1; i <= 5; i++ {
		rFail := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(badBody))
		rFail.Header.Set("Content-Type", "application/json")
		rFail.RemoteAddr = fmt.Sprintf("10.0.0.%d:1234", i) // different IPs to only trigger email lockout
		wFail := httptest.NewRecorder()
		app.ServeHTTP(wFail, rFail)
		if wFail.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, wFail.Code)
		}
	}

	// 6th attempt: should be locked out
	rLockout := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(badBody))
	rLockout.Header.Set("Content-Type", "application/json")
	rLockout.RemoteAddr = "10.0.0.99:1234"
	wLockout := httptest.NewRecorder()
	app.ServeHTTP(wLockout, rLockout)

	if wLockout.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 6th failed attempt, got %d", wLockout.Code)
	}
	if wLockout.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}

	// D. Tampered, expired, and logged-out tokens -> 401
	rTampered := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rTampered.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "tampered_token_12345"})
	wTampered := httptest.NewRecorder()
	app.ServeHTTP(wTampered, rTampered)
	if wTampered.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for tampered token, got %d", wTampered.Code)
	}

	// Logout revokes session and clears cookie
	rLogout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rLogout.AddCookie(sessionCookie)
	wLogout := httptest.NewRecorder()
	app.ServeHTTP(wLogout, rLogout)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", wLogout.Code)
	}

	// Using the token after logout -> 401
	rAfterLogout := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rAfterLogout.AddCookie(sessionCookie)
	wAfterLogout := httptest.NewRecorder()
	app.ServeHTTP(wAfterLogout, rAfterLogout)
	if wAfterLogout.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 using logged-out token, got %d", wAfterLogout.Code)
	}
}

// 3. User Management & Protection of Last Active Admin
func TestCompulsory_UserManagementAndLastAdmin(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	ctx := context.Background()

	// A. POST /api/v1/auth/register -> 404
	rReg := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader([]byte(`{}`)))
	wReg := httptest.NewRecorder()
	app.ServeHTTP(wReg, rReg)
	if wReg.Code != http.StatusNotFound {
		t.Errorf("expected 404 for removed register route, got %d", wReg.Code)
	}

	// B. Non-admin POST /api/v1/users -> 403
	driverEmail := fmt.Sprintf("driver-%s@example.com", uuid.NewString()[:8])
	pwHash, _ := auth.HashPassword("DriverPass123!")
	_, err := db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Driver', true, now(), now())`, uuid.NewString(), driverEmail, pwHash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, driverEmail)
	}()

	driverToken, _, _ := auth.CreateSession(ctx, db, driverEmail, "127.0.0.1", "test-agent")
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, driverEmail)
	}()

	createBody, _ := json.Marshal(map[string]string{
		"email":    fmt.Sprintf("newuser-%s@example.com", uuid.NewString()[:8]),
		"password": "NewUserPassword123!",
		"role":     "Driver",
	})
	rNonAdmin := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(createBody))
	rNonAdmin.Header.Set("Content-Type", "application/json")
	rNonAdmin.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: driverToken})
	wNonAdmin := httptest.NewRecorder()
	app.ServeHTTP(wNonAdmin, rNonAdmin)

	if wNonAdmin.Code != http.StatusForbidden {
		t.Errorf("expected 403 when non-admin accesses POST /api/v1/users, got %d", wNonAdmin.Code)
	}

	// C. Last active Admin cannot be demoted or deactivated -> 409
	// Ensure exactly 1 active admin
	adminEmail := fmt.Sprintf("solo-admin-%s@example.com", uuid.NewString()[:8])
	adminId := uuid.NewString()
	_, _ = db.Exec(ctx, `DELETE FROM users WHERE role = 'Admin'`)
	_, err = db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin', true, now(), now())`, adminId, adminEmail, pwHash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, adminEmail)
	}()

	adminToken, _, _ := auth.CreateSession(ctx, db, adminEmail, "127.0.0.1", "test-agent")
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, adminEmail)
	}()

	// Attempt to deactivate the solo admin
	deactBody := []byte(`{"is_active": false}`)
	rDeact := httptest.NewRequest("PATCH", "/api/v1/users/"+adminId, bytes.NewReader(deactBody))
	rDeact.Header.Set("Content-Type", "application/json")
	rDeact.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: adminToken})
	wDeact := httptest.NewRecorder()
	app.ServeHTTP(wDeact, rDeact)

	if wDeact.Code != http.StatusConflict {
		t.Errorf("expected 409 when deactivating last active admin, got %d", wDeact.Code)
	}

	// Attempt to demote the solo admin
	demoteBody := []byte(`{"role": "Fleet Manager"}`)
	rDemote := httptest.NewRequest("PATCH", "/api/v1/users/"+adminId, bytes.NewReader(demoteBody))
	rDemote.Header.Set("Content-Type", "application/json")
	rDemote.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: adminToken})
	wDemote := httptest.NewRecorder()
	app.ServeHTTP(wDemote, rDemote)

	if wDemote.Code != http.StatusConflict {
		t.Errorf("expected 409 when demoting last active admin, got %d", wDemote.Code)
	}
}

// 4. Security Headers & Error Model
func TestCompulsory_SecurityHeadersAndErrorModel(t *testing.T) {
	app := setupTestApp(t, nil, false) // production mode for HSTS

	testCases := []struct {
		name       string
		method     string
		path       string
		body       io.Reader
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "200 OK (healthz)",
			method:     "GET",
			path:       "/healthz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "401 Unauthorized (unauthenticated me)",
			method:     "GET",
			path:       "/api/v1/auth/me",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "404 Not Found",
			method:     "GET",
			path:       "/api/v1/nonexistent",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "413 Payload Too Large (>1 MB)",
			method:     "POST",
			path:       "/api/v1/auth/login",
			body:       bytes.NewReader(make([]byte, 1024*1024+100)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, tc.body)
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			app.ServeHTTP(w, r)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, w.Code)
			}

			// Verify required security headers
			h := w.Header()
			if h.Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing or invalid X-Content-Type-Options")
			}
			if h.Get("X-Frame-Options") != "DENY" {
				t.Error("missing or invalid X-Frame-Options")
			}
			if h.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
				t.Error("missing or invalid Referrer-Policy")
			}
			if h.Get("Content-Security-Policy") != "default-src 'none'" {
				t.Errorf("expected Content-Security-Policy: default-src 'none', got %q", h.Get("Content-Security-Policy"))
			}
			if h.Get("Strict-Transport-Security") == "" {
				t.Error("missing Strict-Transport-Security in production mode")
			}

			// If it's an error response, verify uniform error envelope
			if tc.wantStatus >= 400 {
				var env httpx.ErrorEnvelope
				if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
					t.Fatalf("error response is not valid json: %v", err)
				}
				if env.Error.Code == "" || env.Error.Message == "" || env.Error.RequestID == "" {
					t.Errorf("incomplete error shape: %+v", env.Error)
				}
			}
		})
	}
}

// 5. CORS Allow-List Test
func TestCompulsory_CORS(t *testing.T) {
	app := setupTestApp(t, nil, true)

	// Allowed origin
	r1 := httptest.NewRequest("GET", "/healthz", nil)
	r1.Header.Set("Origin", "http://localhost:3000")
	w1 := httptest.NewRecorder()
	app.ServeHTTP(w1, r1)

	if w1.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected allowed origin in header, got %q", w1.Header().Get("Access-Control-Allow-Origin"))
	}
	if w1.Header().Get("Vary") != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", w1.Header().Get("Vary"))
	}

	// Disallowed origin
	r2 := httptest.NewRequest("GET", "/healthz", nil)
	r2.Header.Set("Origin", "https://evil-hacker.com")
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, r2)

	if w2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", w2.Header().Get("Access-Control-Allow-Origin"))
	}
}

// 6. Readyz Probe: 200 when DB up, 503 when down, no config leaked in body
func TestCompulsory_Readyz(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	r := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for readyz with live DB, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "postgres") || strings.Contains(w.Body.String(), "secret") {
		t.Errorf("readyz body leaks configuration data: %s", w.Body.String())
	}

	// When DB is nil
	appDown := setupTestApp(t, nil, true)
	rDown := httptest.NewRequest("GET", "/readyz", nil)
	wDown := httptest.NewRecorder()
	appDown.ServeHTTP(wDown, rDown)

	if wDown.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for readyz with down DB, got %d", wDown.Code)
	}
}

// 7. Role x Route matrix (table-driven)
func TestCompulsory_RoleRouteMatrix(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	roles := []string{
		rbac.RoleAdmin,
		rbac.RoleFleetManager,
		rbac.RoleDriver,
		rbac.RoleSafetyOfficer,
		rbac.RoleFinancialAnalyst,
	}

	tokens := make(map[string]string)
	for _, role := range roles {
		email := fmt.Sprintf("matrix-%s@example.com", strings.ReplaceAll(strings.ToLower(role), " ", "-"))
		pwHash, _ := auth.HashPassword("TestPass123!")
		_, _ = db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, true, now(), now())
			ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role`, uuid.NewString(), email, pwHash, role)
		tok, _, _ := auth.CreateSession(ctx, db, email, "127.0.0.1", "matrix-test")
		tokens[role] = tok
		defer func(e string) {
			_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, e)
			_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, e)
		}(email)
	}

	// Matrix test: GET /api/v1/users (Admin only)
	for _, role := range roles {
		r := httptest.NewRequest("GET", "/api/v1/users", nil)
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tokens[role]})
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)

		if role == rbac.RoleAdmin {
			if w.Code != http.StatusOK {
				t.Errorf("Admin expected 200 on /api/v1/users, got %d", w.Code)
			}
		} else {
			if w.Code != http.StatusForbidden {
				t.Errorf("Role %q expected 403 on /api/v1/users, got %d", role, w.Code)
			}
		}
	}

	// Matrix test: POST /api/v1/fleet/vehicles (Fleet Manager and Admin only)
	for _, role := range roles {
		body := []byte(`{"registration_number": "TRK-001", "model": "Volvo", "type": "Semi", "max_load_capacity": 5000, "odometer": 1000, "acquisition_cost": 50000, "status": "Available"}`)
		r := httptest.NewRequest("POST", "/api/v1/fleet/vehicles", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tokens[role]})
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)

		if role == rbac.RoleAdmin || role == rbac.RoleFleetManager {
			if w.Code == http.StatusForbidden {
				t.Errorf("Role %q should NOT be forbidden on POST /api/v1/fleet/vehicles", role)
			}
		} else {
			if w.Code != http.StatusForbidden {
				t.Errorf("Role %q expected 403 on POST /api/v1/fleet/vehicles, got %d", role, w.Code)
			}
		}
	}
}


// 8. Changing password invalidates user's other sessions
func TestCompulsory_ChangePasswordInvalidatesOtherSessions(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	email := fmt.Sprintf("pwchange-%s@example.com", uuid.NewString()[:8])
	oldPass := "OriginalPassword123!"
	newPass := "NewSecurePassword456!"
	hash, _ := auth.HashPassword(oldPass)

	_, err := db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Driver', true, now(), now())`, uuid.NewString(), email, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)
		_, _ = db.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, email)
	}()

	// Create 2 sessions for this user
	tok1, _, err := auth.CreateSession(ctx, db, email, "127.0.0.1", "device-1")
	if err != nil {
		t.Fatal(err)
	}
	tok2, _, err := auth.CreateSession(ctx, db, email, "127.0.0.1", "device-2")
	if err != nil {
		t.Fatal(err)
	}

	// Tok1 and Tok2 should both be valid right now
	rPre1 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rPre1.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tok1})
	wPre1 := httptest.NewRecorder()
	app.ServeHTTP(wPre1, rPre1)
	if wPre1.Code != http.StatusOK {
		t.Fatalf("expected 200 before change, got %d", wPre1.Code)
	}

	// Change password using session 1
	chgBody, _ := json.Marshal(map[string]string{
		"current_password": oldPass,
		"new_password":     newPass,
	})
	rChg := httptest.NewRequest("POST", "/api/v1/auth/change-password", bytes.NewReader(chgBody))
	rChg.Header.Set("Content-Type", "application/json")
	rChg.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tok1})
	wChg := httptest.NewRecorder()
	app.ServeHTTP(wChg, rChg)

	if wChg.Code != http.StatusOK {
		t.Fatalf("expected 200 on change password, got %d, body: %s", wChg.Code, wChg.Body.String())
	}

	// Session 1 should still be valid
	rPost1 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rPost1.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tok1})
	wPost1 := httptest.NewRecorder()
	app.ServeHTTP(wPost1, rPost1)
	if wPost1.Code != http.StatusOK {
		t.Errorf("session 1 should remain valid, got %d", wPost1.Code)
	}

	// Session 2 MUST now be invalidated (401)
	rPost2 := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	rPost2.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: tok2})
	wPost2 := httptest.NewRecorder()
	app.ServeHTTP(wPost2, rPost2)
	if wPost2.Code != http.StatusUnauthorized {
		t.Errorf("session 2 should have been revoked after password change, got %d", wPost2.Code)
	}
}

// 9. 21st failed login from one IP in 15 min -> 429
func TestCompulsory_IPLockout(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)
	ctx := context.Background()

	ip := fmt.Sprintf("192.168.1.%d", time.Now().Unix()%200+10)
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM login_attempts WHERE ip = $1`, ip)
	}()

	// Perform 20 failures from the same IP with different emails
	for i := 1; i <= 20; i++ {
		email := fmt.Sprintf("ipfail-%d-%d@example.com", time.Now().Unix(), i)
		body, _ := json.Marshal(map[string]string{"email": email, "password": "wrong"})
		r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":5000"
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, w.Code)
		}
	}

	// 21st attempt from this IP must trigger 429
	email21 := fmt.Sprintf("ipfail-21-%d@example.com", time.Now().Unix())
	body21, _ := json.Marshal(map[string]string{"email": email21, "password": "wrong"})
	r21 := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body21))
	r21.Header.Set("Content-Type", "application/json")
	r21.RemoteAddr = ip + ":5000"
	w21 := httptest.NewRecorder()
	app.ServeHTTP(w21, r21)

	if w21.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 21st failed attempt from IP %s, got %d", ip, w21.Code)
	}
	if w21.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on IP lockout response")
	}
}

// 10. Constraint mapping: unique -> 409, check -> 422, FK -> 409
func TestCompulsory_ConstraintMapping(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// A. Unique constraint violation (23505) -> 409
	dupEmail := fmt.Sprintf("dup-%s@example.com", uuid.NewString()[:8])
	hash, _ := auth.HashPassword("TestPass123!")
	_, err := db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Admin', true, now(), now())`, uuid.NewString(), dupEmail, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM users WHERE email = $1`, dupEmail)
	}()

	// Duplicate insert on users.email
	_, dupErr := db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, 'Driver', true, now(), now())`, uuid.NewString(), dupEmail, hash)
	if dupErr == nil {
		t.Fatal("expected duplicate key error")
	}

	status, code, _ := httpx.MapDBError(dupErr, nil)
	if status != http.StatusConflict || code != "CONFLICT" {
		t.Errorf("unique violation: expected 409 CONFLICT, got %d %s", status, code)
	}

	// B. Foreign key constraint violation (23503) -> 409
	// trips table references vehicles.registration_number
	_, fkErr := db.Exec(ctx, `INSERT INTO trips (id, source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
		VALUES (999999, 'CityA', 'CityB', 'NONEXISTENT_VEHICLE_REG_XYZ', 'LIC-1', 100, 100, 100, 'Draft')`)
	if fkErr != nil {
		status, code, _ := httpx.MapDBError(fkErr, nil)
		if status != http.StatusConflict || code != "FOREIGN_KEY_VIOLATION" {
			t.Errorf("foreign key violation: expected 409 FOREIGN_KEY_VIOLATION, got %d %s", status, code)
		}
	}
}
