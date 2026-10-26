package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	IsActive      bool      `json:"is_active"`
	DriverLicense *string   `json:"driver_license,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateUserRequest struct {
	Email         string  `json:"email"`
	Password      string  `json:"password"`
	Role          string  `json:"role"`
	DriverLicense *string `json:"driver_license,omitempty"`
}

type UpdateUserRequest struct {
	Role          *string `json:"role,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
	DriverLicense *string `json:"driver_license,omitempty"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(),
		`SELECT id, email, role, is_active, driver_license, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	defer rows.Close()

	var result []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.IsActive, &u.DriverLicense, &u.CreatedAt, &u.UpdatedAt); err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan users", nil)
			return
		}
		result = append(result, u)
	}
	if result == nil {
		result = []User{}
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func recordAuditLog(ctx context.Context, tx pgx.Tx, entity, entityID, action, actorEmail string, details any) {
	var detailsBytes []byte
	if details != nil {
		detailsBytes, _ = json.Marshal(details)
	}
	_, _ = tx.Exec(ctx, `INSERT INTO audit_log (entity, entity_id, action, actor_email, details, at)
		VALUES ($1, $2, $3, $4, $5, now())`,
		entity, entityID, action, actorEmail, detailsBytes)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "A valid email address is required.", nil)
		return
	}

	if len(req.Password) < 8 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Password must be at least 8 characters.", nil)
		return
	}

	if !slices.Contains(rbac.AllRoles, req.Role) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", fmt.Sprintf("Role must be one of: %v", rbac.AllRoles), nil)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash password", nil)
		return
	}

	id := uuid.NewString()
	now := time.Now().UTC()

	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction failed", nil)
		return
	}
	defer tx.Rollback(ctx)

	var created User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (id, email, password_hash, role, is_active, driver_license, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, true, $5, $6, $6)
		 RETURNING id, email, role, is_active, driver_license, created_at, updated_at`,
		id, req.Email, hash, req.Role, req.DriverLicense, now).
		Scan(&created.ID, &created.Email, &created.Role, &created.IsActive, &created.DriverLicense, &created.CreatedAt, &created.UpdatedAt)

	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLog(ctx, tx, "users", created.ID, "create", actorEmail, map[string]any{
		"email": req.Email,
		"role":  req.Role,
	})

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Commit failed", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "User identifier is required.", nil)
		return
	}

	var req UpdateUserRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction failed", nil)
		return
	}
	defer tx.Rollback(ctx)

	var current User
	err = tx.QueryRow(ctx,
		`SELECT id, email, role, is_active, driver_license, created_at, updated_at FROM users WHERE id = $1 OR email = $1 FOR UPDATE`,
		id).Scan(&current.ID, &current.Email, &current.Role, &current.IsActive, &current.DriverLicense, &current.CreatedAt, &current.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database error", nil)
		return
	}

	// Guard: Admin cannot demote or deactivate the last active Admin
	if current.Role == rbac.RoleAdmin && current.IsActive {
		demoting := req.Role != nil && *req.Role != rbac.RoleAdmin
		deactivating := req.IsActive != nil && !*req.IsActive
		if demoting || deactivating {
			var activeAdmins int
			err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = $1 AND is_active = true`, rbac.RoleAdmin).Scan(&activeAdmins)
			if err != nil {
				httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database error", nil)
				return
			}
			if activeAdmins <= 1 {
				httpx.WriteError(w, r, http.StatusConflict, "LAST_ADMIN_PROTECTED", "Cannot demote or deactivate the last active Admin.", nil)
				return
			}
		}
	}

	newRole := current.Role
	if req.Role != nil {
		if !slices.Contains(rbac.AllRoles, *req.Role) {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid role specified.", nil)
			return
		}
		newRole = *req.Role
	}

	newIsActive := current.IsActive
	if req.IsActive != nil {
		newIsActive = *req.IsActive
	}

	newDriverLicense := current.DriverLicense
	if req.DriverLicense != nil {
		newDriverLicense = req.DriverLicense
	}

	now := time.Now().UTC()
	var updated User
	err = tx.QueryRow(ctx,
		`UPDATE users
		 SET role = $1, is_active = $2, driver_license = $3, updated_at = $4
		 WHERE id = $5
		 RETURNING id, email, role, is_active, driver_license, created_at, updated_at`,
		newRole, newIsActive, newDriverLicense, now, current.ID).
		Scan(&updated.ID, &updated.Email, &updated.Role, &updated.IsActive, &updated.DriverLicense, &updated.CreatedAt, &updated.UpdatedAt)

	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	if !newIsActive {
		// Invalidate all active sessions for deactivated user
		_, _ = tx.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, current.Email)
	}

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLog(ctx, tx, "users", updated.ID, "update", actorEmail, map[string]any{
		"old_role":      current.Role,
		"new_role":      newRole,
		"old_is_active": current.IsActive,
		"new_is_active": newIsActive,
	})

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Commit failed", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "User identifier is required.", nil)
		return
	}

	var req ResetPasswordRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if len(req.NewPassword) < 8 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Password must be at least 8 characters.", nil)
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to hash password", nil)
		return
	}

	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction failed", nil)
		return
	}
	defer tx.Rollback(ctx)

	var email, userID string
	err = tx.QueryRow(ctx,
		`UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3 OR email = $3 RETURNING id, email`,
		hash, time.Now().UTC(), id).Scan(&userID, &email)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.", nil)
			return
		}
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Invalidate all sessions on password reset
	_, _ = tx.Exec(ctx, `DELETE FROM sessions WHERE user_email = $1`, email)

	actorEmail := "system"
	if user := middleware.GetUser(r); user != nil {
		actorEmail = user.Email
	}
	recordAuditLog(ctx, tx, "users", userID, "reset-password", actorEmail, map[string]any{
		"email": email,
	})

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Commit failed", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Password reset successfully. All sessions revoked.",
	})
}
