package app

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/domain"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/users"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Secret         string
	Logger         *slog.Logger
	AllowedOrigins []string
	Env            string
	CookieDomain   string
	DatabaseURL       string
	MetricsToken      string
	JobSecret         string
	TrustedProxyCIDRs []*net.IPNet
}

func (c *Config) IsLocal() bool {
	return c.Env == "local" || c.Env == ""
}

type App struct {
	db       *pgxpool.Pool
	cfg      Config
	log      *slog.Logger
	mux      *http.ServeMux
	handler  http.Handler
	registry *rbac.Registry
	clients  map[*websocket.Conn]bool
	cmu      sync.Mutex
	usersH   *users.Handler
	hub      *realtime.Hub
	metrics  *MetricsCollector
}

func New(db *pgxpool.Pool, c Config) *App {
	dbURL := c.DatabaseURL
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
		if dbURL == "" {
			dbURL = os.Getenv("POSTGRES_DSN")
		}
	}
	hub := realtime.NewHub(dbURL, db, c.AllowedOrigins, c.Logger)

	a := &App{
		db:       db,
		cfg:      c,
		log:      c.Logger,
		mux:      http.NewServeMux(),
		registry: rbac.NewRegistry(),
		clients:  make(map[*websocket.Conn]bool),
		usersH:   users.NewHandler(db),
		hub:      hub,
		metrics:  newMetricsCollector(),
	}
	a.routes()
	a.buildHandler()
	return a
}

func (a *App) Start(ctx context.Context) {
	if a.hub != nil {
		a.hub.Start(ctx)
	}
}

func (a *App) Ready() <-chan struct{} {
	if a.hub != nil {
		return a.hub.Ready()
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (a *App) Close() {
	if a.hub != nil {
		a.hub.Close()
	}
}

func (a *App) Hub() *realtime.Hub {
	return a.hub
}

func (a *App) Registry() *rbac.Registry {
	return a.registry
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.handler.ServeHTTP(w, r)
}

func (a *App) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, a.handler)
}

func (a *App) Metrics() *MetricsCollector {
	return a.metrics
}

func (a *App) getRoutePattern(r *http.Request) string {
	if r.URL.Path == "/" {
		return "root"
	}
	_, pattern := a.mux.Handler(r)
	parts := strings.SplitN(pattern, " ", 2)
	var pathPattern string
	if len(parts) == 2 {
		pathPattern = parts[1]
	} else {
		pathPattern = parts[0]
	}
	if pathPattern == "" || pathPattern == "/" {
		return "unmatched"
	}
	if pathPattern == "/healthz" || pathPattern == "/readyz" || pathPattern == "/metrics" {
		return pathPattern
	}
	for _, route := range a.registry.Routes() {
		if route.Path == pathPattern && route.Method == r.Method {
			return route.Path
		}
	}
	return "unmatched"
}

func (a *App) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routePattern := a.getRoutePattern(r)
		r = middleware.SetRoutePattern(r, routePattern)
		a.metrics.IncrementInFlight()
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		defer func() {
			a.metrics.DecrementInFlight()
			duration := time.Since(start)
			a.metrics.RecordHTTPRequest(r.Method, routePattern, rec.statusCode, duration)
		}()
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (rw *statusRecorder) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (a *App) buildHandler() {
	// Middleware pipeline in exact required order:
	// metrics -> recover -> request-id -> access log -> secure headers -> CORS -> body size limit -> rate limit -> auth -> router
	chain := a.mux

	var h http.Handler = chain
	// 8. Auth
	h = middleware.Auth(a.db)(h)
	// 7. Rate Limit (120 req/min per IP)
	h = middleware.RateLimit(120, time.Minute, a.cfg.TrustedProxyCIDRs)(h)
	// 6. Body Size Limit (1 MB)
	h = middleware.BodySizeLimit(middleware.MaxBodyBytes)(h)
	// 5. CORS
	h = middleware.CORS(a.cfg.AllowedOrigins)(h)
	// 4. Secure Headers
	h = middleware.SecureHeaders(a.cfg.IsLocal())(h)
	// 3. Access Log
	h = middleware.AccessLog(a.log)(h)
	// 2. RequestID
	h = middleware.RequestID()(h)
	// 1. Recover
	h = middleware.Recover(a.log)(h)
	// 0b. Request timeout (default 15s)
	h = middleware.RequestTimeout(15 * time.Second)(h)
	// 0a. Metrics
	h = a.metricsMiddleware(h)

	a.handler = h
}

func (a *App) registerRoute(method, path string, roles []string, isPublic bool, h http.HandlerFunc) {
	if !isPublic && len(roles) == 0 {
		panic(fmt.Sprintf("route %s %s must specify allowed roles", method, path))
	}
	if isPublic {
		allowed := (method == "GET" && (path == "/healthz" || path == "/readyz")) || (method == "POST" && path == "/api/v1/auth/login")
		if !allowed {
			panic(fmt.Sprintf("route %s %s cannot be public", method, path))
		}
	}
	a.registry.Register(method, path, roles, isPublic, "")
	pattern := fmt.Sprintf("%s %s", method, path)

	if isPublic {
		a.mux.HandleFunc(pattern, h)
	} else if path == "/api/v1/ws" || path == "/metrics" || path == "/api/v1/internal/jobs/license-check" {
		// Endpoints with custom authorization models (tickets, Bearer tokens, or shared job secret)
		a.mux.HandleFunc(pattern, h)
	} else {
		guarded := middleware.RequireRoles(roles)(h)
		a.mux.Handle(pattern, guarded)
	}
}

func (a *App) routes() {
	// Catch-all 404 for any unregistered endpoint
	a.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"status": "online",
				"system": "TransitOps Backend Architecture Operational",
			})
			return
		}
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Path %s not found", r.URL.Path), nil)
	})

	// System / Health probes (Public)
	a.registerRoute("GET", "/healthz", nil, true, a.healthz)
	a.registerRoute("GET", "/readyz", nil, true, a.readyz)
	if a.cfg.IsLocal() {
		a.registerRoute("GET", "/api/v1/test/slow", rbac.AnyAuthenticated, false, a.handleTestSlow)
		a.registerRoute("GET", "/api/v1/test/db-timeout", rbac.AnyAuthenticated, false, a.handleTestDBTimeout)
	}

	// Auth endpoints
	a.registerRoute("POST", "/api/v1/auth/login", nil, true, a.login)
	a.registerRoute("POST", "/api/v1/auth/logout", rbac.AnyAuthenticated, false, a.logout)
	a.registerRoute("GET", "/api/v1/auth/me", rbac.AnyAuthenticated, false, a.me)
	a.registerRoute("POST", "/api/v1/auth/change-password", rbac.AnyAuthenticated, false, a.changePassword)

	// User management endpoints (Admin only)
	a.registerRoute("GET", "/api/v1/users", []string{rbac.RoleAdmin}, false, a.usersH.List)
	a.registerRoute("POST", "/api/v1/users", []string{rbac.RoleAdmin}, false, a.usersH.Create)
	a.registerRoute("PATCH", "/api/v1/users/{id}", []string{rbac.RoleAdmin}, false, a.usersH.Update)
	a.registerRoute("POST", "/api/v1/users/{id}/reset-password", []string{rbac.RoleAdmin}, false, a.usersH.ResetPassword)

	// Fleet: Vehicles
	for _, prefix := range []string{"/api/v1/vehicles", "/api/v1/fleet/vehicles"} {
		a.registerRoute("GET", prefix, rbac.AnyAuthenticated, false, a.vehicles)
		a.registerRoute("POST", prefix, []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.createVehicle)
		a.registerRoute("GET", prefix+"/{reg}", rbac.AnyAuthenticated, false, a.vehicle)
		a.registerRoute("PATCH", prefix+"/{reg}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.updateVehicle)
		a.registerRoute("PUT", prefix+"/{reg}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.updateVehicle)
		a.registerRoute("POST", prefix+"/{reg}/retire", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.retireVehicle)
		a.registerRoute("DELETE", prefix+"/{reg}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteVehicle)
	}

	// Fleet: Drivers
	for _, prefix := range []string{"/api/v1/drivers", "/api/v1/fleet/drivers"} {
		a.registerRoute("GET", prefix, []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleDriver}, false, a.drivers)
		a.registerRoute("POST", prefix, []string{rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.createDriver)
		a.registerRoute("GET", prefix+"/{license}", []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleDriver}, false, a.driver)
		a.registerRoute("PATCH", prefix+"/{license}", []string{rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.updateDriver)
		a.registerRoute("PUT", prefix+"/{license}", []string{rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.updateDriver)
		a.registerRoute("POST", prefix+"/{license}/suspend", []string{rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.suspendDriver)
		a.registerRoute("POST", prefix+"/{license}/reinstate", []string{rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.reinstateDriver)
		a.registerRoute("POST", prefix+"/{license}/set-off-duty", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.setDriverOffDuty)
		a.registerRoute("POST", prefix+"/{license}/set-available", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.setDriverAvailable)
		a.registerRoute("DELETE", prefix+"/{license}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteDriver)
	}

	// Destinations
	for _, prefix := range []string{"/api/v1/destinations", "/api/v1/destinations/"} {
		a.registerRoute("GET", prefix, rbac.AnyAuthenticated, false, a.destinations)
		a.registerRoute("POST", prefix, []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.createDestination)
	}
	a.registerRoute("DELETE", "/api/v1/destinations/{name}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteDestination)

	// Trips
	for _, prefix := range []string{"/api/v1/trips", "/api/v1/trips/"} {
		a.registerRoute("GET", prefix, rbac.AnyAuthenticated, false, a.trips)
		a.registerRoute("POST", prefix, []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleCreateTrip)
	}
	a.registerRoute("GET", "/api/v1/trips/{id}", rbac.AnyAuthenticated, false, a.handleGetTrip)
	a.registerRoute("GET", "/api/v1/trips/live", rbac.AnyAuthenticated, false, a.handleLiveTrips)
	a.registerRoute("POST", "/api/v1/trips/{id}/dispatch", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleDispatchTrip)
	a.registerRoute("POST", "/api/v1/trips/{id}/complete", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleCompleteTrip)
	a.registerRoute("PUT", "/api/v1/trips/{id}/complete", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleCompleteTrip)
	a.registerRoute("POST", "/api/v1/trips/{id}/cancel", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleCancelTrip)
	a.registerRoute("PUT", "/api/v1/trips/{id}/cancel", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleCancelTrip)
	a.registerRoute("POST", "/api/v1/trips/suggest", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.handleSuggest)

	// Maintenance
	maintReadRoles := []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleSafetyOfficer}
	maintWriteRoles := []string{rbac.RoleAdmin, rbac.RoleFleetManager}
	for _, prefix := range []string{"/api/v1/maintenance", "/api/v1/maintenance/"} {
		a.registerRoute("GET", prefix, maintReadRoles, false, a.handleListMaintenance)
		a.registerRoute("POST", prefix, maintWriteRoles, false, a.handleOpenMaintenance)
	}
	a.registerRoute("GET", "/api/v1/maintenance/{id}", maintReadRoles, false, a.handleGetMaintenance)
	a.registerRoute("POST", "/api/v1/maintenance/{id}/close", maintWriteRoles, false, a.handleCloseMaintenance)
	a.registerRoute("PUT", "/api/v1/maintenance/{id}/close", maintWriteRoles, false, a.handleCloseMaintenance)

	// Finance & Expenses
	financeRoles := []string{rbac.RoleFinancialAnalyst, rbac.RoleFleetManager, rbac.RoleAdmin}
	for _, prefix := range []string{"/api/v1/expenses", "/api/v1/expenses/", "/api/v1/finance/expenses", "/api/v1/finance/expenses/"} {
		a.registerRoute("POST", prefix, financeRoles, false, a.handleCreateExpense)
	}
	for _, prefix := range []string{"/api/v1/expenses", "/api/v1/expenses/"} {
		a.registerRoute("GET", prefix, financeRoles, false, a.handleListExpenses)
	}
	a.registerRoute("GET", "/api/v1/finance/expenses/{id}", financeRoles, false, a.handleGetTripExpenses)
	a.registerRoute("GET", "/api/v1/finance/summary", financeRoles, false, a.handleFinanceSummary)

	// Dashboard
	dashboardRoles := []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleFinancialAnalyst, rbac.RoleSafetyOfficer}
	exportRoles := []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleFinancialAnalyst}
	a.registerRoute("GET", "/api/v1/dashboard/kpis", dashboardRoles, false, a.handleDashboardKPIs)
	a.registerRoute("GET", "/api/v1/dashboard/utilization", append(append([]string{}, dashboardRoles...), rbac.RoleDriver), false, a.handleDashboardUtilization)
	a.registerRoute("GET", "/api/v1/dashboard/me", []string{rbac.RoleDriver}, false, a.handleDriverSelfDashboard)
	a.registerRoute("GET", "/api/v1/dashboard/export.csv", exportRoles, false, a.handleDashboardExportCSV)

	// Audit Log
	a.registerRoute("GET", "/api/v1/audit", []string{rbac.RoleAdmin}, false, a.handleListAuditLogs)

	// Realtime Hub (Phase 5)
	a.registerRoute("POST", "/api/v1/realtime/ticket", rbac.AnyAuthenticated, false, a.handleIssueRealtimeTicket)
	a.registerRoute("GET", "/api/v1/ws", rbac.AnyAuthenticated, false, a.handleWebSocketUpgrade)

	// Metrics endpoint (Phase 5)
	a.registerRoute("GET", "/metrics", []string{rbac.RoleAdmin}, false, a.handleMetrics)

	// Notifications & Background Jobs (Phase 5)
	a.registerRoute("POST", "/api/v1/internal/jobs/license-check", []string{rbac.RoleAdmin}, false, a.handleLicenseCheckJob)
	notifRoles := []string{rbac.RoleAdmin, rbac.RoleFleetManager, rbac.RoleSafetyOfficer}
	a.registerRoute("GET", "/api/v1/notifications", notifRoles, false, a.handleListNotifications)
	a.registerRoute("POST", "/api/v1/notifications/{id}/read", notifRoles, false, a.handleMarkNotificationRead)
}

func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if a.db == nil || a.db.Ping(ctx) != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleTestSlow(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(16 * time.Second):
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case <-r.Context().Done():
		return
	}
}

func (a *App) handleTestDBTimeout(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database not connected", nil)
		return
	}
	// Run pg_sleep with a statement_timeout
	conn, err := a.db.Acquire(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to acquire conn", nil)
		return
	}
	defer conn.Release()

	_, _ = conn.Exec(r.Context(), "SET statement_timeout = '50ms'")
	_, err = conn.Exec(r.Context(), "SELECT pg_sleep(1)")
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) getRemoteIP(r *http.Request) string {
	return middleware.GetClientIP(r, a.cfg.TrustedProxyCIDRs)
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var email, pw string
	var workspace, company, expectedRole string
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var body struct {
			Email        string `json:"email"`
			Username     string `json:"username"`
			Password     string `json:"password"`
			Workspace    string `json:"workspace"`
			Company      string `json:"company"`
			ExpectedRole string `json:"expected_role"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if len(body.Workspace) > 64 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "workspace must not exceed 64 characters", map[string]any{"field": "workspace"})
			return
		}
		if len(body.Company) > 64 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "company must not exceed 64 characters", map[string]any{"field": "company"})
			return
		}
		if body.ExpectedRole != "" {
			switch body.ExpectedRole {
			case "Admin", "Fleet Manager", "Driver", "Safety Officer", "Financial Analyst":
			default:
				httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid expected_role", map[string]any{"field": "expected_role"})
				return
			}
		}
		email = body.Email
		if email == "" {
			email = body.Username
		}
		pw = body.Password
		workspace = body.Workspace
		company = body.Company
		expectedRole = body.ExpectedRole
	} else {
		if err := r.ParseForm(); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid form data", nil)
			return
		}
		email = r.Form.Get("email")
		if email == "" {
			email = r.Form.Get("username")
		}
		pw = r.Form.Get("password")
		workspace = r.Form.Get("workspace")
		company = r.Form.Get("company")
		expectedRole = r.Form.Get("expected_role")
		if len(workspace) > 64 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "workspace must not exceed 64 characters", map[string]any{"field": "workspace"})
			return
		}
		if len(company) > 64 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "company must not exceed 64 characters", map[string]any{"field": "company"})
			return
		}
		if expectedRole != "" {
			switch expectedRole {
			case "Admin", "Fleet Manager", "Driver", "Safety Officer", "Financial Analyst":
			default:
				httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid expected_role", map[string]any{"field": "expected_role"})
				return
			}
		}
	}
	_ = workspace
	_ = company

	email = strings.TrimSpace(email)
	ip := a.getRemoteIP(r)

	// Check lockout before DB query
	locked, retrySecs, err := auth.CheckLockout(r.Context(), a.db, email, ip)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Login verification service error", nil)
		return
	}
	if locked {
		w.Header().Set("Retry-After", strconv.Itoa(retrySecs))
		httpx.WriteError(w, r, http.StatusTooManyRequests, "ACCOUNT_LOCKED", "Too many failed login attempts. Please try again later.", nil)
		return
	}

	var hash, role string
	var isActive bool
	err = a.db.QueryRow(r.Context(), `SELECT password_hash, role, is_active FROM users WHERE email = $1`, email).
		Scan(&hash, &role, &isActive)

	if err != nil {
		// Non-existent user: run dummy Argon2id verification to guarantee timing invariance
		auth.DummyVerify(pw)
		_ = auth.RecordLoginAttempt(r.Context(), a.db, email, ip, false)
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Incorrect email or password", nil)
		return
	}

	valid, _ := auth.VerifyPassword(pw, hash)
	if !valid {
		_ = auth.RecordLoginAttempt(r.Context(), a.db, email, ip, false)
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Incorrect email or password", nil)
		return
	}

	if !isActive {
		httpx.WriteError(w, r, http.StatusUnauthorized, "ACCOUNT_INACTIVE", "Your account has been deactivated.", nil)
		return
	}
	if expectedRole != "" && expectedRole != role {
		httpx.WriteError(w, r, http.StatusForbidden, "ROLE_MISMATCH", "This account is not a "+expectedRole+" account.", map[string]any{
			"expected_role": expectedRole,
			"actual_role":   role,
		})
		return
	}

	// Login succeeded: reset failure count
	_ = auth.RecordLoginAttempt(r.Context(), a.db, email, ip, true)

	token, expiresAt, err := auth.CreateSession(r.Context(), a.db, email, ip, r.UserAgent())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Session establishment failed", nil)
		return
	}

	auth.SetSessionCookie(w, token, expiresAt, a.cfg.IsLocal(), a.cfg.CookieDomain)

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"email": email,
		"role":  role,
	})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	token := auth.GetSessionToken(r)
	if token != "" {
		_ = auth.RevokeSession(r.Context(), a.db, token)
	}
	auth.ClearSessionCookie(w, a.cfg.IsLocal(), a.cfg.CookieDomain)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated", nil)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if len(req.NewPassword) < 8 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "New password must be at least 8 characters long.", nil)
		return
	}

	ctx := r.Context()
	var storedHash string
	err := a.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE email = $1`, user.Email).Scan(&storedHash)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to verify account state", nil)
		return
	}

	valid, _ := auth.VerifyPassword(req.CurrentPassword, storedHash)
	if !valid {
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Current password does not match.", nil)
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash new password", nil)
		return
	}

	_, err = a.db.Exec(ctx, `UPDATE users SET password_hash = $1, updated_at = now() WHERE email = $2`, newHash, user.Email)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update password", nil)
		return
	}

	// Invalidate other sessions for this user
	currentToken := auth.GetSessionToken(r)
	_ = auth.RevokeOtherSessions(ctx, a.db, user.Email, currentToken)

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Password updated successfully. Other active sessions revoked.",
	})
}

func decodeCursor(c string) (string, error) {
	if c == "" {
		return "", nil
	}
	b, err := base64.URLEncoding.DecodeString(c)
	if err != nil {
		b, err = base64.RawURLEncoding.DecodeString(c)
		if err != nil {
			b, err = base64.StdEncoding.DecodeString(c)
			if err != nil {
				return "", fmt.Errorf("invalid cursor")
			}
		}
	}
	return string(b), nil
}

func encodeCursor(val string) string {
	return base64.URLEncoding.EncodeToString([]byte(val))
}

func parsePaginationParams(r *http.Request) (cursor string, limit int, err error) {
	limit = 25
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		parsed, err := strconv.Atoi(lStr)
		if err != nil || parsed <= 0 {
			limit = 25
		} else if parsed > 100 {
			limit = 100
		} else {
			limit = parsed
		}
	}
	cursorRaw := r.URL.Query().Get("cursor")
	if cursorRaw != "" {
		c, err := decodeCursor(cursorRaw)
		if err != nil {
			return "", 0, err
		}
		cursor = c
	}
	return cursor, limit, nil
}

// Business handlers
func (a *App) vehicles(w http.ResponseWriter, r *http.Request) {
	cursor, limit, err := parsePaginationParams(r)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Invalid pagination cursor", nil)
		return
	}

	q := r.URL.Query().Get("q")
	vType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	includeInactive := false
	if r.URL.Query().Get("include_inactive") == "true" {
		user := middleware.GetUser(r)
		if user != nil && (user.Role == rbac.RoleAdmin || user.Role == rbac.RoleFleetManager) {
			includeInactive = true
		}
	}

	query := `
		SELECT registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active
		FROM vehicles
		WHERE ($1::text = '' OR registration_number > $1)
		  AND ($2::text = '' OR type = $2)
		  AND ($3::text = '' OR status = $3)
		  AND ($4::boolean OR is_active = true)
		  AND ($5::text = '' OR (registration_number ILIKE '%' || $5 || '%' OR model ILIKE '%' || $5 || '%'))
		ORDER BY registration_number ASC
		LIMIT $6`

	v, err := rows[vehicle](r.Context(), a.db, query, cursor, vType, status, includeInactive, q, limit+1)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch vehicles", nil)
		return
	}

	var nextCursor *string
	items := v
	if len(v) > limit {
		nc := encodeCursor(v[limit-1].RegistrationNumber)
		nextCursor = &nc
		items = v[:limit]
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[vehicle]{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func (a *App) vehicle(w http.ResponseWriter, r *http.Request) {
	reg := r.PathValue("reg")
	var v vehicle
	err := a.db.QueryRow(r.Context(),
		`SELECT registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active
		 FROM vehicles WHERE registration_number = $1`, reg).
		Scan(&v.RegistrationNumber, &v.Model, &v.Type, &v.MaxLoadCapacity, &v.Odometer, &v.AcquisitionCost, &v.Status, &v.IsActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) createVehicle(w http.ResponseWriter, r *http.Request) {
	var v vehicle
	if err := httpx.DecodeJSON(w, r, &v); err != nil {
		return
	}
	if v.RegistrationNumber == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "registration_number is required", map[string]any{"field": "registration_number"})
		return
	}
	if v.Model == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "model is required", map[string]any{"field": "model"})
		return
	}
	if !domain.VehicleType(v.Type).IsValid() {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid vehicle type", map[string]any{"field": "type"})
		return
	}
	if v.MaxLoadCapacity <= 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "max_load_capacity must be greater than zero", map[string]any{"field": "max_load_capacity"})
		return
	}
	if v.Odometer < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "odometer must be non-negative", map[string]any{"field": "odometer"})
		return
	}
	if v.AcquisitionCost < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "acquisition_cost must be non-negative", map[string]any{"field": "acquisition_cost"})
		return
	}
	if v.Status == "" {
		v.Status = string(domain.VehicleStatusAvailable)
	} else if !domain.VehicleStatus(v.Status).IsValid() {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid vehicle status", map[string]any{"field": "status"})
		return
	}
	v.IsActive = true

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var n int
	tx.QueryRow(r.Context(), "SELECT count(*) FROM vehicles WHERE registration_number=$1", v.RegistrationNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_EXISTS", "Vehicle Registration Number must be unique.", nil)
		return
	}
	_, e := tx.Exec(r.Context(), "INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8)",
		v.RegistrationNumber, v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status, v.IsActive)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "vehicles", v.RegistrationNumber, "create", actorEmail, map[string]any{
		"model": v.Model,
		"type":  v.Type,
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (a *App) updateVehicle(w http.ResponseWriter, r *http.Request) {
	reg := r.PathValue("reg")
	if reg == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Registration number is required", nil)
		return
	}

	var req updateVehicleRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if req.Type != nil && !domain.VehicleType(*req.Type).IsValid() {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid vehicle type", map[string]any{"field": "type"})
		return
	}
	if req.MaxLoadCapacity != nil && *req.MaxLoadCapacity <= 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "max_load_capacity must be greater than zero", map[string]any{"field": "max_load_capacity"})
		return
	}
	if req.Odometer != nil && *req.Odometer < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "odometer must be non-negative", map[string]any{"field": "odometer"})
		return
	}
	if req.AcquisitionCost != nil && *req.AcquisitionCost < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "acquisition_cost must be non-negative", map[string]any{"field": "acquisition_cost"})
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var v vehicle
	err = tx.QueryRow(r.Context(),
		`SELECT registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active
		 FROM vehicles WHERE registration_number = $1 FOR UPDATE`, reg).
		Scan(&v.RegistrationNumber, &v.Model, &v.Type, &v.MaxLoadCapacity, &v.Odometer, &v.AcquisitionCost, &v.Status, &v.IsActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}

	if req.Model != nil {
		v.Model = *req.Model
	}
	if req.Type != nil {
		v.Type = *req.Type
	}
	if req.MaxLoadCapacity != nil {
		v.MaxLoadCapacity = *req.MaxLoadCapacity
	}
	if req.Odometer != nil {
		v.Odometer = *req.Odometer
	}
	if req.AcquisitionCost != nil {
		v.AcquisitionCost = *req.AcquisitionCost
	}

	_, err = tx.Exec(r.Context(),
		`UPDATE vehicles
		 SET model = $1, type = $2, max_load_capacity = $3, odometer = $4, acquisition_cost = $5, updated_at = now()
		 WHERE registration_number = $6`,
		v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, reg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "vehicles", reg, "update", actorEmail, map[string]any{
		"model": v.Model,
		"type":  v.Type,
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) retireVehicle(w http.ResponseWriter, r *http.Request) {
	reg := r.PathValue("reg")
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM vehicles WHERE registration_number = $1 FOR UPDATE`, reg).Scan(&status)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}

	if status == string(domain.VehicleStatusOnTrip) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Cannot retire vehicle while on trip", nil)
		return
	}

	var activeTrips int
	err = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM trips WHERE vehicle_reg = $1 AND status IN ('Draft', 'Dispatched')`, reg).Scan(&activeTrips)
	if err == nil && activeTrips > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Cannot retire vehicle with active or draft trips", nil)
		return
	}

	_, err = tx.Exec(r.Context(), `UPDATE vehicles SET status = 'Retired', updated_at = now() WHERE registration_number = $1`, reg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	user := middleware.GetUser(r)
	actorEmail := "system"
	if user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "vehicles", reg, "retire", actorEmail, map[string]any{"old_status": status, "new_status": "Retired"})

	_ = realtime.PublishTx(r.Context(), tx, realtime.EventVehicleStatusChange, map[string]any{
		"vehicle_reg": reg,
		"status":      "Retired",
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Vehicle retired successfully",
		"status":  "Retired",
	})
}

func (a *App) deleteVehicle(w http.ResponseWriter, r *http.Request) {
	reg := r.PathValue("reg")
	if reg == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Registration number is required", nil)
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	var isActive bool
	err = tx.QueryRow(r.Context(),
		`SELECT status, is_active FROM vehicles WHERE registration_number = $1 FOR UPDATE`, reg).
		Scan(&status, &isActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}

	// 1. Check if on trip or dispatched trip
	var dispatchedTrips int
	_ = tx.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM trips WHERE vehicle_reg = $1 AND status = 'Dispatched'`, reg).
		Scan(&dispatchedTrips)
	if status == string(domain.VehicleStatusOnTrip) || dispatchedTrips > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_IN_USE", "Cannot delete vehicle while on trip", nil)
		return
	}

	// 2. Check if open maintenance
	var openMaint int
	_ = tx.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM maintenance_logs WHERE vehicle_reg = $1 AND status = 'Open'`, reg).
		Scan(&openMaint)
	if status == string(domain.VehicleStatusInShop) || openMaint > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_IN_USE", "Cannot delete vehicle with open maintenance", nil)
		return
	}

	// 3. Check if any history exists (trips or maintenance logs)
	var tripHistory int
	_ = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM trips WHERE vehicle_reg = $1`, reg).Scan(&tripHistory)

	var maintHistory int
	_ = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM maintenance_logs WHERE vehicle_reg = $1`, reg).Scan(&maintHistory)

	if tripHistory > 0 || maintHistory > 0 {
		// Soft delete: set is_active = false
		_, err = tx.Exec(r.Context(),
			`UPDATE vehicles SET is_active = false, updated_at = now() WHERE registration_number = $1`, reg)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, a.log)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}
		actorEmail := "system"
		if user := middleware.GetUser(r); user != nil {
			actorEmail = user.Email
		}
		recordAuditLogTx(r.Context(), tx, "vehicles", reg, "soft_delete", actorEmail, map[string]any{"is_active": false})

		if err := tx.Commit(r.Context()); err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"message":      "Vehicle archived successfully",
			"soft_deleted": true,
		})
		return
	}

	// Zero history -> Hard delete
	_, err = tx.Exec(r.Context(), `DELETE FROM vehicles WHERE registration_number = $1`, reg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "vehicles", reg, "hard_delete", actorEmail, map[string]any{"purged": true})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"message":      "Vehicle deleted successfully",
		"soft_deleted": false,
	})
}

func (a *App) drivers(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user != nil && user.Role == rbac.RoleDriver {
		ownLicense := user.DriverLicense
		if ownLicense == "" {
			httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[driver]{
				Items:      []driver{},
				NextCursor: nil,
			})
			return
		}
		var d driver
		err := a.db.QueryRow(r.Context(),
			`SELECT license_number, name, license_category, license_expiry_date::text, contact_number, safety_score, status, is_active
			 FROM drivers WHERE license_number = $1`, ownLicense).
			Scan(&d.LicenseNumber, &d.Name, &d.LicenseCategory, &d.LicenseExpiryDate, &d.ContactNumber, &d.SafetyScore, &d.Status, &d.IsActive)
		if err != nil {
			httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[driver]{
				Items:      []driver{},
				NextCursor: nil,
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[driver]{
			Items:      []driver{d},
			NextCursor: nil,
		})
		return
	}

	cursor, limit, err := parsePaginationParams(r)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Invalid pagination cursor", nil)
		return
	}

	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	expiringDays := 0
	if dStr := r.URL.Query().Get("license_expiring_within_days"); dStr != "" {
		expiringDays, _ = strconv.Atoi(dStr)
	}

	includeInactive := false
	if r.URL.Query().Get("include_inactive") == "true" {
		if user != nil && (user.Role == rbac.RoleAdmin || user.Role == rbac.RoleFleetManager) {
			includeInactive = true
		}
	}

	query := `
		SELECT license_number, name, license_category, license_expiry_date::text, contact_number, safety_score, status, is_active
		FROM drivers
		WHERE ($1::text = '' OR license_number > $1)
		  AND ($2::text = '' OR status = $2)
		  AND ($3::boolean OR is_active = true)
		  AND ($4::text = '' OR (name ILIKE '%' || $4 || '%' OR license_number ILIKE '%' || $4 || '%'))
		  AND ($5::int = 0 OR (license_expiry_date <= CURRENT_DATE + ($5::int * INTERVAL '1 day')))
		ORDER BY license_number ASC
		LIMIT $6`

	v, err := rows[driver](r.Context(), a.db, query, cursor, status, includeInactive, q, expiringDays, limit+1)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve drivers", nil)
		return
	}

	var nextCursor *string
	items := v
	if len(v) > limit {
		nc := encodeCursor(v[limit-1].LicenseNumber)
		nextCursor = &nc
		items = v[:limit]
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[driver]{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func (a *App) driver(w http.ResponseWriter, r *http.Request) {
	lic := r.PathValue("license")
	user := middleware.GetUser(r)
	if user != nil && user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != lic {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Access denied: drivers may only access their own record", nil)
			return
		}
	}

	var d driver
	err := a.db.QueryRow(r.Context(),
		`SELECT license_number, name, license_category, license_expiry_date::text, contact_number, safety_score, status, is_active
		 FROM drivers WHERE license_number = $1`, lic).
		Scan(&d.LicenseNumber, &d.Name, &d.LicenseCategory, &d.LicenseExpiryDate, &d.ContactNumber, &d.SafetyScore, &d.Status, &d.IsActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver profile not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

func (a *App) createDriver(w http.ResponseWriter, r *http.Request) {
	var v driver
	if err := httpx.DecodeJSON(w, r, &v); err != nil {
		return
	}
	if v.LicenseNumber == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "license_number is required", map[string]any{"field": "license_number"})
		return
	}
	if v.Name == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name is required", map[string]any{"field": "name"})
		return
	}
	if v.LicenseCategory == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "license_category is required", map[string]any{"field": "license_category"})
		return
	}
	if _, err := domain.ParseDate(v.LicenseExpiryDate); err != nil {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid license_expiry_date format, must be YYYY-MM-DD", map[string]any{"field": "license_expiry_date"})
		return
	}
	if v.SafetyScore == 0 {
		v.SafetyScore = 100
	}
	if v.SafetyScore < 0 || v.SafetyScore > 100 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "safety_score must be between 0 and 100", map[string]any{"field": "safety_score"})
		return
	}
	if v.Status == "" {
		v.Status = string(domain.DriverStatusAvailable)
	} else if !domain.DriverStatus(v.Status).IsValid() {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid driver status", map[string]any{"field": "status"})
		return
	}
	v.IsActive = true

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var n int
	tx.QueryRow(r.Context(), "SELECT count(*) FROM drivers WHERE license_number=$1", v.LicenseNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "DRIVER_EXISTS", "Driver License Number already registered.", nil)
		return
	}
	_, e := tx.Exec(r.Context(), "INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8)",
		v.LicenseNumber, v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status, v.IsActive)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", v.LicenseNumber, "create", actorEmail, map[string]any{
		"name":             v.Name,
		"license_category": v.LicenseCategory,
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (a *App) updateDriver(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	if license == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "License number is required", nil)
		return
	}

	var req updateDriverRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if req.LicenseExpiryDate != nil {
		if _, err := domain.ParseDate(*req.LicenseExpiryDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid license_expiry_date format, must be YYYY-MM-DD", map[string]any{"field": "license_expiry_date"})
			return
		}
	}
	if req.SafetyScore != nil && (*req.SafetyScore < 0 || *req.SafetyScore > 100) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "safety_score must be between 0 and 100", map[string]any{"field": "safety_score"})
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var v driver
	err = tx.QueryRow(r.Context(),
		`SELECT license_number, name, license_category, license_expiry_date::text, contact_number, safety_score, status, is_active
		 FROM drivers WHERE license_number = $1 FOR UPDATE`, license).
		Scan(&v.LicenseNumber, &v.Name, &v.LicenseCategory, &v.LicenseExpiryDate, &v.ContactNumber, &v.SafetyScore, &v.Status, &v.IsActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver profile not found", nil)
		return
	}

	if req.Name != nil {
		v.Name = *req.Name
	}
	if req.LicenseCategory != nil {
		v.LicenseCategory = *req.LicenseCategory
	}
	if req.LicenseExpiryDate != nil {
		v.LicenseExpiryDate = *req.LicenseExpiryDate
	}
	if req.ContactNumber != nil {
		v.ContactNumber = *req.ContactNumber
	}
	if req.SafetyScore != nil {
		v.SafetyScore = *req.SafetyScore
	}

	_, err = tx.Exec(r.Context(),
		`UPDATE drivers
		 SET name = $1, license_category = $2, license_expiry_date = $3::date, contact_number = $4, safety_score = $5, updated_at = now()
		 WHERE license_number = $6`,
		v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "update", actorEmail, map[string]any{
		"name":             v.Name,
		"license_category": v.LicenseCategory,
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) suspendDriver(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM drivers WHERE license_number = $1 FOR UPDATE`, license).Scan(&status)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}

	if status == string(domain.DriverStatusOnTrip) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Cannot suspend driver while on trip", nil)
		return
	}

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Suspended', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	user := middleware.GetUser(r)
	actorEmail := "system"
	if user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "suspend", actorEmail, map[string]any{"old_status": status, "new_status": "Suspended"})
	_ = realtime.PublishTx(r.Context(), tx, realtime.EventDriverStatusChange, map[string]any{
		"driver_license": license,
		"status":         "Suspended",
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Driver suspended successfully",
		"status":  "Suspended",
	})
}

func (a *App) reinstateDriver(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM drivers WHERE license_number = $1 FOR UPDATE`, license).Scan(&status)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}

	if status == string(domain.DriverStatusOnTrip) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Cannot reinstate driver while on trip", nil)
		return
	}
	if status != string(domain.DriverStatusSuspended) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", fmt.Sprintf("Can only reinstate a suspended driver (current status: %s)", status), nil)
		return
	}

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Available', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	user := middleware.GetUser(r)
	actorEmail := "system"
	if user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "reinstate", actorEmail, map[string]any{"old_status": status, "new_status": "Available"})
	_ = realtime.PublishTx(r.Context(), tx, realtime.EventDriverStatusChange, map[string]any{
		"driver_license": license,
		"status":         "Available",
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Driver reinstated successfully",
		"status":  "Available",
	})
}

func (a *App) setDriverOffDuty(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM drivers WHERE license_number = $1 FOR UPDATE`, license).Scan(&status)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}

	if status == string(domain.DriverStatusSuspended) || status == string(domain.DriverStatusOnTrip) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", fmt.Sprintf("Cannot set driver off-duty while %s", status), nil)
		return
	}

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Off Duty', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	user := middleware.GetUser(r)
	actorEmail := "system"
	if user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "set_off_duty", actorEmail, map[string]any{"old_status": status, "new_status": "Off Duty"})
	_ = realtime.PublishTx(r.Context(), tx, realtime.EventDriverStatusChange, map[string]any{
		"driver_license": license,
		"status":         "Off Duty",
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Driver status set to Off Duty",
		"status":  "Off Duty",
	})
}

func (a *App) setDriverAvailable(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM drivers WHERE license_number = $1 FOR UPDATE`, license).Scan(&status)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}

	if status == string(domain.DriverStatusSuspended) || status == string(domain.DriverStatusOnTrip) {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", fmt.Sprintf("Cannot set driver available while %s", status), nil)
		return
	}

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Available', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	user := middleware.GetUser(r)
	actorEmail := "system"
	if user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "set_available", actorEmail, map[string]any{"old_status": status, "new_status": "Available"})
	_ = realtime.PublishTx(r.Context(), tx, realtime.EventDriverStatusChange, map[string]any{
		"driver_license": license,
		"status":         "Available",
	})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Driver status set to Available",
		"status":  "Available",
	})
}

func (a *App) deleteDriver(w http.ResponseWriter, r *http.Request) {
	license := r.PathValue("license")
	if license == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "License number is required", nil)
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to start transaction", nil)
		return
	}
	defer tx.Rollback(r.Context())

	var status string
	var isActive bool
	err = tx.QueryRow(r.Context(),
		`SELECT status, is_active FROM drivers WHERE license_number = $1 FOR UPDATE`, license).
		Scan(&status, &isActive)
	if err != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}

	// 1. Check if currently on trip
	var dispatchedTrips int
	_ = tx.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM trips WHERE driver_license = $1 AND status = 'Dispatched'`, license).
		Scan(&dispatchedTrips)
	if status == string(domain.DriverStatusOnTrip) || dispatchedTrips > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "DRIVER_IN_USE", "Cannot delete driver currently on trip", nil)
		return
	}

	// 2. Check if any trip history exists
	var tripHistory int
	_ = tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM trips WHERE driver_license = $1`, license).Scan(&tripHistory)

	if tripHistory > 0 {
		// Soft delete: set is_active = false
		_, err = tx.Exec(r.Context(),
			`UPDATE drivers SET is_active = false, updated_at = now() WHERE license_number = $1`, license)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, a.log)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}
		actorEmail := "system"
		if user := middleware.GetUser(r); user != nil {
			actorEmail = user.Email
		}
		recordAuditLogTx(r.Context(), tx, "drivers", license, "soft_delete", actorEmail, map[string]any{"is_active": false})

		if err := tx.Commit(r.Context()); err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"message":      "Driver archived successfully",
			"soft_deleted": true,
		})
		return
	}

	// Zero history -> Hard delete
	_, err = tx.Exec(r.Context(), `DELETE FROM drivers WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLogTx(r.Context(), tx, "drivers", license, "hard_delete", actorEmail, map[string]any{"purged": true})

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"message":      "Driver profile purged successfully",
		"soft_deleted": false,
	})
}

func (a *App) destinations(w http.ResponseWriter, r *http.Request) {
	cursor, limit, err := parsePaginationParams(r)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Invalid pagination cursor", nil)
		return
	}

	q := r.URL.Query().Get("q")
	query := `
		SELECT name, region, latitude, longitude
		FROM destinations
		WHERE ($1::text = '' OR name > $1)
		  AND ($2::text = '' OR (name ILIKE '%' || $2 || '%' OR region ILIKE '%' || $2 || '%'))
		ORDER BY name ASC
		LIMIT $3`

	x, err := rows[destination](r.Context(), a.db, query, cursor, q, limit+1)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch destinations", nil)
		return
	}

	var nextCursor *string
	items := x
	if len(x) > limit {
		nc := encodeCursor(x[limit-1].Name)
		nextCursor = &nc
		items = x[:limit]
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[destination]{
		Items:      items,
		NextCursor: nextCursor,
	})
}

func (a *App) createDestination(w http.ResponseWriter, r *http.Request) {
	var x destination
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	if x.Name == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "name is required", map[string]any{"field": "name"})
		return
	}
	if x.Region == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "region is required", map[string]any{"field": "region"})
		return
	}
	if (x.Latitude == nil) != (x.Longitude == nil) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "latitude and longitude must both be set or both be empty", map[string]any{"field": "latitude"})
		return
	}
	if x.Latitude != nil && (*x.Latitude < -90 || *x.Latitude > 90) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "latitude must be between -90 and 90", map[string]any{"field": "latitude"})
		return
	}
	if x.Longitude != nil && (*x.Longitude < -180 || *x.Longitude > 180) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "longitude must be between -180 and 180", map[string]any{"field": "longitude"})
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM destinations WHERE name=$1", x.Name).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "DESTINATION_EXISTS", "Destination name must be unique.", nil)
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO destinations (name, region, latitude, longitude) VALUES ($1, $2, $3, $4)", x.Name, x.Region, x.Latitude, x.Longitude)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (a *App) deleteDestination(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ct, e := a.db.Exec(r.Context(), "DELETE FROM destinations WHERE name=$1", name)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	if ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Destination hub not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Destination deleted successfully"})
}

func (a *App) trips(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rowsTrips, err := a.db.Query(ctx, `SELECT t.id, t.source_name, t.destination_name, t.vehicle_reg, t.driver_license, t.cargo_weight, t.planned_distance, t.revenue, t.status, t.start_odometer, t.final_odometer, t.created_at, t.updated_at, t.completed_at, e.liters, e.cost
		FROM trips t
		LEFT JOIN (
			SELECT trip_id, liters, cost,
				ROW_NUMBER() OVER (PARTITION BY trip_id ORDER BY id DESC) as rn
			FROM expenses WHERE type = 'Fuel'
		) e ON e.trip_id = t.id AND e.rn = 1
		ORDER BY t.id ASC`)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch trips", nil)
		return
	}
	defer rowsTrips.Close()

	var items []trip
	for rowsTrips.Next() {
		var t trip
		var startOdo, finalOdo, fuelLiters, fuelCost *float64
		var createdAt, updatedAt time.Time
		var completedAt *time.Time
		if err := rowsTrips.Scan(&t.ID, &t.SourceName, &t.DestinationName, &t.VehicleReg, &t.DriverLicense, &t.CargoWeight, &t.PlannedDistance, &t.Revenue, &t.Status, &startOdo, &finalOdo, &createdAt, &updatedAt, &completedAt, &fuelLiters, &fuelCost); err != nil {
			continue
		}
		t.StartOdometer = startOdo
		t.FinalOdometer = finalOdo
		t.FuelLiters = fuelLiters
		t.FuelCost = fuelCost
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		if completedAt != nil {
			cAt := completedAt.Format(time.RFC3339)
			t.CompletedAt = &cAt
		}
		items = append(items, t)
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[trip]{
		Items: items,
	})
}







func (a *App) handleIssueRealtimeTicket(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	ticket, expiresIn, err := realtime.IssueTicket(r.Context(), a.db, user.ID, user.Role)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to issue ticket.", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ticket":     ticket,
		"expires_in": expiresIn,
	})
}

func (a *App) getDriverLicense(ctx context.Context, userID string) string {
	if a.db == nil {
		return ""
	}
	var lic *string
	_ = a.db.QueryRow(ctx, "SELECT driver_license FROM users WHERE id = $1", userID).Scan(&lic)
	if lic != nil {
		return *lic
	}
	return ""
}

func (a *App) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	if a.hub == nil {
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Realtime hub is not initialized.", nil)
		return
	}
	_ = a.hub.Upgrade(w, r, a.getDriverLicense)
}

func (a *App) ws(w http.ResponseWriter, r *http.Request) {
	a.handleWebSocketUpgrade(w, r)
}

func (a *App) broadcast(event string, payload any) {
	// Maintained for backward compatibility with existing tests
	b, _ := json.Marshal(map[string]any{"event": event, "payload": payload})
	a.cmu.Lock()
	defer a.cmu.Unlock()
	for c := range a.clients {
		if e := c.WriteMessage(websocket.TextMessage, b); e != nil {
			delete(a.clients, c)
			c.Close()
		}
	}
}
