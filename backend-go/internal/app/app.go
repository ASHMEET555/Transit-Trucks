package app

import (
	"context"
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

	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/auth"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/middleware"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/platform/httpx"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/rbac"
	"github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/users"
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
	h = middleware.SecureHeaders()(h)
	// 3. Access Log
	h = middleware.AccessLog(a.log)(h)
	// 2. RequestID
	h = middleware.RequestID()(h)
	// 1. Recover
	h = middleware.Recover(a.log)(h)

	a.handler = h
}

func (a *App) registerRoute(method, path string, roles []string, isPublic bool, h http.HandlerFunc) {
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
	a.registerRoute("GET", "/api/v1/fleet/vehicles", rbac.AnyAuthenticated, false, a.vehicles)
	a.registerRoute("POST", "/api/v1/fleet/vehicles", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.createVehicle)
	a.registerRoute("GET", "/api/v1/fleet/vehicles/{reg}", rbac.AnyAuthenticated, false, a.vehicle)
	a.registerRoute("PUT", "/api/v1/fleet/vehicles/{reg}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.updateVehicle)
	a.registerRoute("DELETE", "/api/v1/fleet/vehicles/{reg}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteVehicle)

	// Fleet: Drivers
	a.registerRoute("GET", "/api/v1/fleet/drivers", rbac.AnyAuthenticated, false, a.drivers)
	a.registerRoute("POST", "/api/v1/fleet/drivers", []string{rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.createDriver)
	a.registerRoute("GET", "/api/v1/fleet/drivers/{license}", rbac.AnyAuthenticated, false, a.driver)
	a.registerRoute("PUT", "/api/v1/fleet/drivers/{license}", []string{rbac.RoleFleetManager, rbac.RoleSafetyOfficer, rbac.RoleAdmin}, false, a.updateDriver)
	a.registerRoute("DELETE", "/api/v1/fleet/drivers/{license}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteDriver)

	// Destinations
	a.registerRoute("GET", "/api/v1/destinations/", rbac.AnyAuthenticated, false, a.destinations)
	a.registerRoute("POST", "/api/v1/destinations/", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.createDestination)
	a.registerRoute("DELETE", "/api/v1/destinations/{name}", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.deleteDestination)

	// Trips
	a.registerRoute("GET", "/api/v1/trips/", rbac.AnyAuthenticated, false, a.trips)
	a.registerRoute("POST", "/api/v1/trips/suggest", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.suggest)
	a.registerRoute("POST", "/api/v1/trips/", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.dispatch)
	a.registerRoute("PUT", "/api/v1/trips/{id}/complete", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.complete)
	a.registerRoute("PUT", "/api/v1/trips/{id}/cancel", []string{rbac.RoleFleetManager, rbac.RoleDriver, rbac.RoleAdmin}, false, a.cancel)

	// Maintenance
	a.registerRoute("GET", "/api/v1/maintenance/", rbac.AnyAuthenticated, false, a.maintenance)
	a.registerRoute("POST", "/api/v1/maintenance/", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.openMaintenance)
	a.registerRoute("PUT", "/api/v1/maintenance/{id}/close", []string{rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.closeMaintenance)

	// Finance
	a.registerRoute("POST", "/api/v1/finance/expenses", []string{rbac.RoleFinancialAnalyst, rbac.RoleFleetManager, rbac.RoleAdmin}, false, a.expense)
	a.registerRoute("GET", "/api/v1/finance/expenses/{id}", rbac.AnyAuthenticated, false, a.expenses)

	// Dashboard
	a.registerRoute("GET", "/api/v1/dashboard/kpis", rbac.AnyAuthenticated, false, a.kpis)

	// WebSocket Hub
	a.mux.HandleFunc("GET /ws/fleet", a.ws)
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

// Business handlers
func (a *App) vehicles(w http.ResponseWriter, r *http.Request) {
	v, e := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles")
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch vehicles", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) vehicle(w http.ResponseWriter, r *http.Request) {
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", r.PathValue("reg"))
	if e != nil {
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
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM vehicles WHERE registration_number=$1", v.RegistrationNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Vehicle Registration Number must be unique.", nil)
		return
	}
	if v.Status == "" {
		v.Status = "Available"
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO vehicles VALUES($1,$2,$3,$4,$5,$6,$7)", v.RegistrationNumber, v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (a *App) updateVehicle(w http.ResponseWriter, r *http.Request) {
	var v vehicle
	if err := httpx.DecodeJSON(w, r, &v); err != nil {
		return
	}
	ct, e := a.db.Exec(r.Context(), "UPDATE vehicles SET model=$1,type=$2,max_load_capacity=$3,odometer=$4,acquisition_cost=$5,status=$6 WHERE registration_number=$7", v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status, r.PathValue("reg"))
	if e != nil || ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found or modification failed", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Vehicle updated successfully"})
}

func (a *App) deleteVehicle(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM vehicles WHERE registration_number=$1", r.PathValue("reg"))
	if e != nil || ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Vehicle deleted successfully"})
}

func (a *App) drivers(w http.ResponseWriter, r *http.Request) {
	v, e := rows[driver](r.Context(), a.db, "SELECT * FROM drivers")
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve drivers", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) driver(w http.ResponseWriter, r *http.Request) {
	v, e := one[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE license_number=$1", r.PathValue("license"))
	if e != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver profile not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (a *App) createDriver(w http.ResponseWriter, r *http.Request) {
	var v driver
	if err := httpx.DecodeJSON(w, r, &v); err != nil {
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM drivers WHERE license_number=$1", v.LicenseNumber).Scan(&n)
	if n > 0 {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Driver License Number already registered.", nil)
		return
	}
	if v.Status == "" {
		v.Status = "Available"
	}
	if v.SafetyScore == 0 {
		v.SafetyScore = 100
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO drivers VALUES($1,$2,$3,$4,$5,$6,$7)", v.LicenseNumber, v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, v)
}

func (a *App) updateDriver(w http.ResponseWriter, r *http.Request) {
	var v driver
	if err := httpx.DecodeJSON(w, r, &v); err != nil {
		return
	}
	ct, e := a.db.Exec(r.Context(), "UPDATE drivers SET name=$1,license_category=$2,license_expiry_date=$3,contact_number=$4,safety_score=$5,status=$6 WHERE license_number=$7", v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status, r.PathValue("license"))
	if e != nil || ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver profile not found or modification failed", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Driver updated successfully"})
}

func (a *App) deleteDriver(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM drivers WHERE license_number=$1", r.PathValue("license"))
	if e != nil || ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Driver not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Driver profile purged successfully"})
}

func (a *App) destinations(w http.ResponseWriter, r *http.Request) {
	x, e := rows[destination](r.Context(), a.db, "SELECT * FROM destinations")
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch destinations", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) createDestination(w http.ResponseWriter, r *http.Request) {
	var x destination
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO destinations VALUES($1,$2)", x.Name, x.Region)
	if e != nil {
		status, code, msg := httpx.MapDBError(e, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) deleteDestination(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM destinations WHERE name=$1", r.PathValue("name"))
	if e != nil || ct.RowsAffected() == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Destination hub not found", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Destination deleted successfully"})
}

func (a *App) trips(w http.ResponseWriter, r *http.Request) {
	x, e := rows[trip](r.Context(), a.db, "SELECT * FROM trips")
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch trips", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) suggest(w http.ResponseWriter, r *http.Request) {
	var x trip
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	if x.SourceName == x.DestinationName {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Source and destination must be different.", nil)
		return
	}
	vs, _ := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE status='Available' AND max_load_capacity >= $1", x.CargoWeight)
	if len(vs) == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "No available vehicle can carry this cargo.", nil)
		return
	}
	ds, _ := rows[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE status='Available' AND license_expiry_date > $1", time.Now().Format("2006-01-02"))
	if len(ds) == 0 {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "No available and compliant driver found.", nil)
		return
	}
	var bv vehicle
	var bd driver
	best := -math.MaxFloat64
	var util float64
	for _, v := range vs {
		u := x.CargoWeight / v.MaxLoadCapacity
		for _, d := range ds {
			s := u*100 + d.SafetyScore*.6 - v.Odometer/100000 - (v.MaxLoadCapacity-x.CargoWeight)/1000
			if s > best {
				best = s
				bv = v
				bd = d
				util = u
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"source_name":              x.SourceName,
		"destination_name":         x.DestinationName,
		"cargo_weight":             x.CargoWeight,
		"suggested_vehicle_reg":    bv.RegistrationNumber,
		"suggested_driver_license": bd.LicenseNumber,
		"suggested_driver_name":    bd.Name,
		"capacity_utilization":     math.Round(util*10000) / 10000,
		"heuristic_score":          math.Round(best*100) / 100,
	})
}

func (a *App) dispatch(w http.ResponseWriter, r *http.Request) {
	var x trip
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	d, de := one[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE license_number=$1", x.DriverLicense)
	if e != nil || de != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Assigned Vehicle or Driver profile not found.", nil)
		return
	}
	if x.CargoWeight > v.MaxLoadCapacity {
		httpx.WriteError(w, r, http.StatusBadRequest, "OVERLOAD_ERROR", fmt.Sprintf("Cargo (%vkg) exceeds max vehicle capacity (%vkg).", x.CargoWeight, v.MaxLoadCapacity), nil)
		return
	}
	if v.Status == "In Shop" || v.Status == "Retired" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_STATE", fmt.Sprintf("Vehicle cannot be dispatched. Current Status: %s.", v.Status), nil)
		return
	}
	if v.Status == "On Trip" {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Vehicle is already actively deployed on another trip.", nil)
		return
	}
	if d.LicenseExpiryDate <= time.Now().Format("2006-01-02") {
		httpx.WriteError(w, r, http.StatusBadRequest, "COMPLIANCE_FAILURE", "Driver license has expired.", nil)
		return
	}
	if d.Status == "Suspended" {
		httpx.WriteError(w, r, http.StatusBadRequest, "COMPLIANCE_FAILURE", "Assigned driver is currently suspended.", nil)
		return
	}
	if d.Status == "On Trip" {
		httpx.WriteError(w, r, http.StatusConflict, "CONFLICT", "Driver is already assigned to an active trip.", nil)
		return
	}
	x.Status = "Dispatched"
	e = a.db.QueryRow(r.Context(), "INSERT INTO trips(source_name,destination_name,vehicle_reg,driver_license,cargo_weight,planned_distance,revenue,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id", x.SourceName, x.DestinationName, x.VehicleReg, x.DriverLicense, x.CargoWeight, x.PlannedDistance, x.Revenue, x.Status).Scan(&x.ID)
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Server Error", nil)
		return
	}
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='On Trip' WHERE registration_number=$1", x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='On Trip' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.dispatched", map[string]any{"trip_id": x.ID, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Dispatched"})
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (a *App) complete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[trip](r.Context(), a.db, "SELECT * FROM trips WHERE id=$1", id)
	if e != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip record not found.", nil)
		return
	}
	if x.Status != "Dispatched" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_STATE", "Only actively dispatched transits can be finalized.", nil)
		return
	}
	n, _ := strconv.ParseFloat(r.URL.Query().Get("final_odometer"), 64)
	v, _ := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	if n <= v.Odometer {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_ODOMETER", fmt.Sprintf("Odometer reading must be higher than starting values (%v).", v.Odometer), nil)
		return
	}
	a.db.Exec(r.Context(), "UPDATE trips SET status='Completed' WHERE id=$1", id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET odometer=$1,status='Available' WHERE registration_number=$2", n, x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='Available' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.completed", map[string]any{"trip_id": id, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Completed", "final_odometer": n})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Trip completed successfully. Resources released back to active pools."})
}

func (a *App) cancel(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[trip](r.Context(), a.db, "SELECT * FROM trips WHERE id=$1", id)
	if e != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip record not found.", nil)
		return
	}
	if x.Status != "Dispatched" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_STATE", "Only actively dispatched transits can be cancelled.", nil)
		return
	}
	a.db.Exec(r.Context(), "UPDATE trips SET status='Cancelled' WHERE id=$1", id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='Available' WHERE registration_number=$1", x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='Available' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.cancelled", map[string]any{"trip_id": id, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Cancelled"})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Trip cancelled. Vehicle and driver restored to Available."})
}

func (a *App) maintenance(w http.ResponseWriter, r *http.Request) {
	x, e := rows[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs")
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve maintenance logs", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) openMaintenance(w http.ResponseWriter, r *http.Request) {
	var x maintenance
	if err := httpx.DecodeJSON(w, r, &x); err != nil {
		return
	}
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	if e != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found.", nil)
		return
	}
	if v.Status == "On Trip" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_STATE", "Cannot pull an active vehicle off transit into maintenance.", nil)
		return
	}
	x.Status = "Open"
	e = a.db.QueryRow(r.Context(), "INSERT INTO maintenance_logs(vehicle_reg,description,cost,start_date,status) VALUES($1,$2,$3,$4,'Open') RETURNING id", x.VehicleReg, x.Description, x.Cost, x.StartDate).Scan(&x.ID)
	if e != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to open maintenance log", nil)
		return
	}
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='In Shop' WHERE registration_number=$1", x.VehicleReg)
	a.broadcast("maintenance.opened", map[string]any{"log_id": x.ID, "vehicle_reg": x.VehicleReg, "status": "Open"})
	httpx.WriteJSON(w, http.StatusOK, x)
}

func (a *App) closeMaintenance(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs WHERE id=$1", id)
	if e != nil {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Maintenance log index not found.", nil)
		return
	}
	if x.Status == "Closed" {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_STATE", "Work order is already closed.", nil)
		return
	}
	var body struct {
		EndDate string `json:"end_date"`
	}
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		return
	}
	a.db.Exec(r.Context(), "UPDATE maintenance_logs SET status='Closed',end_date=$1 WHERE id=$2", body.EndDate, id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='Available' WHERE registration_number=$1", x.VehicleReg)
	a.broadcast("maintenance.closed", map[string]any{"log_id": id, "vehicle_reg": x.VehicleReg, "status": "Closed"})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "Workshop operations finished. Vehicle returned to fleet."})
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
