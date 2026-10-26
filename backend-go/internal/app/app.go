package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/domain"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
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
}

func New(db *pgxpool.Pool, c Config) *App {
	a := &App{
		db:       db,
		cfg:      c,
		log:      c.Logger,
		mux:      http.NewServeMux(),
		registry: rbac.NewRegistry(),
		clients:  make(map[*websocket.Conn]bool),
		usersH:   users.NewHandler(db),
	}
	a.routes()
	a.buildHandler()
	return a
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

func (a *App) buildHandler() {
	// Middleware pipeline in exact required order:
	// recover -> request-id -> access log -> secure headers -> CORS -> body size limit -> rate limit -> auth -> router
	chain := a.mux

	var h http.Handler = chain
	// 8. Auth
	h = middleware.Auth(a.db)(h)
	// 7. Rate Limit (120 req/min per IP)
	h = middleware.RateLimit(120, time.Minute)(h)
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

	// Finance
	a.registerRoute("POST", "/api/v1/finance/expenses", []string{rbac.RoleFinancialAnalyst, rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.expense)
	a.registerRoute("GET", "/api/v1/finance/expenses/{id}", rbac.AnyAuthenticated, false, a.expenses)

	// Dashboard
	a.registerRoute("GET", "/api/v1/dashboard/kpis", rbac.AnyAuthenticated, false, a.kpis)

	// WebSocket Hub
	a.registerRoute("GET", "/ws/fleet", rbac.AnyAuthenticated, false, a.ws)
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

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func getRemoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var email, pw string
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		var body struct {
			Email    string `json:"email"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid JSON request body", nil)
			return
		}
		email = body.Email
		if email == "" {
			email = body.Username
		}
		pw = body.Password
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
	}

	email = strings.TrimSpace(email)
	ip := getRemoteIP(r)

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

	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM vehicles WHERE registration_number=$1", v.RegistrationNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_EXISTS", "Vehicle Registration Number must be unique.", nil)
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8)",
		v.RegistrationNumber, v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status, v.IsActive)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
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

	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM drivers WHERE license_number=$1", v.LicenseNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "DRIVER_EXISTS", "Driver License Number already registered.", nil)
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8)",
		v.LicenseNumber, v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status, v.IsActive)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
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

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Suspended', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

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

	_, err = tx.Exec(r.Context(), `UPDATE drivers SET status = 'Available', updated_at = now() WHERE license_number = $1`, license)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

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
		SELECT name, region
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
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM destinations WHERE name=$1", x.Name).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "DESTINATION_EXISTS", "Destination name must be unique.", nil)
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO destinations (name, region) VALUES ($1, $2)", x.Name, x.Region)
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
	rowsTrips, err := a.db.Query(ctx, `SELECT id, source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, fuel_liters, fuel_cost, created_at, updated_at
		FROM trips ORDER BY id ASC`)
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
		if err := rowsTrips.Scan(&t.ID, &t.SourceName, &t.DestinationName, &t.VehicleReg, &t.DriverLicense, &t.CargoWeight, &t.PlannedDistance, &t.Revenue, &t.Status, &startOdo, &finalOdo, &fuelLiters, &fuelCost, &createdAt, &updatedAt); err != nil {
			continue
		}
		t.StartOdometer = startOdo
		t.FinalOdometer = finalOdo
		t.FuelLiters = fuelLiters
		t.FuelCost = fuelCost
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		items = append(items, t)
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[trip]{
		Items: items,
	})
}



func (a *App) expense(w http.ResponseWriter, r *http.Request) {
	var x expenseRow
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	ok := map[string]bool{"Fuel": true, "Toll": true, "Maintenance-Surge": true, "Other": true}
	if !ok[x.Type] {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid expense type. Must be one of: [Fuel Maintenance-Surge Other Toll]", nil)
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM trips WHERE id=$1", x.TripID).Scan(&n)
	if n == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip not found for expense logging.", nil)
		return
	}
	if x.Type == "Fuel" && (x.Liters == nil || *x.Liters <= 0) {
		httpx.WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "Fuel expenses require liters > 0.", nil)
		return
	}
	e := a.db.QueryRow(r.Context(), "INSERT INTO expenses(trip_id,type,liters,cost,date) VALUES($1,$2,$3,$4,$5) RETURNING id", x.TripID, x.Type, x.Liters, x.Cost, x.Date).Scan(&x.ID)
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error", nil)
		return
	}
	a.broadcast("expense.created", map[string]any{"expense_id": x.ID, "trip_id": x.TripID, "type": x.Type, "cost": x.Cost})
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (a *App) expenses(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM trips WHERE id=$1", id).Scan(&n)
	if n == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip not found.", nil)
		return
	}
	x, e := rows[expenseRow](r.Context(), a.db, "SELECT * FROM expenses WHERE trip_id=$1", id)
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) kpis(w http.ResponseWriter, r *http.Request) {
	vs, _ := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles")
	ds, _ := rows[driver](r.Context(), a.db, "SELECT * FROM drivers")
	ts, _ := rows[trip](r.Context(), a.db, "SELECT * FROM trips")
	ms, _ := rows[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs")
	active, available, shop, retired := 0, 0, 0, 0
	for _, v := range vs {
		switch v.Status {
		case "On Trip":
			active++
		case "Available":
			available++
		case "In Shop":
			shop++
		case "Retired":
			retired++
		}
	}
	at, pt, ct, duty, ad := 0, 0, 0, 0, 0
	for _, t := range ts {
		if t.Status == "Dispatched" {
			at++
		}
		if t.Status == "Draft" {
			pt++
		}
		if t.Status == "Completed" {
			ct++
		}
	}
	for _, d := range ds {
		if d.Status == "On Trip" {
			duty++
		}
		if d.Status == "Available" {
			ad++
		}
	}
	var tf, td, tc, tr float64
	analytics := []map[string]any{}
	for _, v := range vs {
		var rev, dist, fc, fl, mc float64
		for _, t := range ts {
			if t.VehicleReg == v.RegistrationNumber {
				rev += t.Revenue
				dist += t.PlannedDistance
				es, _ := rows[expenseRow](r.Context(), a.db, "SELECT * FROM expenses WHERE trip_id=$1", t.ID)
				for _, e := range es {
					if e.Type == "Fuel" {
						fc += e.Cost
						if e.Liters != nil {
							fl += *e.Liters
						}
					}
				}
			}
		}
		for _, m := range ms {
			if m.VehicleReg == v.RegistrationNumber {
				mc += m.Cost
			}
		}
		op := fc + mc
		var roi any = nil
		var eff any = nil
		if v.AcquisitionCost > 0 {
			roi = (rev - op) / v.AcquisitionCost
		}
		if fl > 0 {
			eff = dist / fl
		}
		tf += fl
		td += dist
		tc += op
		tr += rev
		analytics = append(analytics, map[string]any{
			"registration_number": v.RegistrationNumber,
			"model":               v.Model,
			"type":                v.Type,
			"status":              v.Status,
			"acquisition_cost":    v.AcquisitionCost,
			"revenue":             rev,
			"fuel_cost":           fc,
			"maintenance_cost":    mc,
			"operational_cost":    op,
			"fuel_liters":         fl,
			"distance":            dist,
			"fuel_efficiency":     eff,
			"roi":                 roi,
		})
	}
	var util float64
	if len(vs)-retired > 0 {
		util = math.Round(float64(active)/float64(len(vs)-retired)*10000) / 100
	}
	var oe any = nil
	if tf > 0 {
		oe = td / tf
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"active_vehicles":          active,
		"available_vehicles":       available,
		"in_shop_vehicles":         shop,
		"retired_vehicles":         retired,
		"active_trips":             at,
		"pending_trips":            pt,
		"completed_trips":          ct,
		"drivers_on_duty":          duty,
		"available_drivers":        ad,
		"fleet_utilization":        util,
		"overall_fuel_efficiency":  oe,
		"total_operational_cost":   tc,
		"total_revenue":            tr,
		"vehicles":                 analytics,
	})
}

func (a *App) ws(w http.ResponseWriter, r *http.Request) {
	c, e := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(w, r, nil)
	if e != nil {
		return
	}
	a.cmu.Lock()
	a.clients[c] = true
	a.cmu.Unlock()
	defer func() {
		a.cmu.Lock()
		delete(a.clients, c)
		a.cmu.Unlock()
		c.Close()
	}()
	for {
		if _, _, e := c.ReadMessage(); e != nil {
			return
		}
	}
}

func (a *App) broadcast(event string, payload any) {
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
