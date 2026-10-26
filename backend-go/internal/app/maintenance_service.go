package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
	"github.com/jackc/pgx/v5"
)

type openMaintenanceRequest struct {
	VehicleReg  string  `json:"vehicle_reg"`
	Description string  `json:"description"`
	Cost        float64 `json:"cost"`
	StartDate   string  `json:"start_date,omitempty"`
}

type closeMaintenanceRequest struct {
	EndDate string `json:"end_date,omitempty"`
}

// handleListMaintenance handles GET /api/v1/maintenance and /api/v1/maintenance/
// Roles: Admin, Fleet Manager, Safety Officer
func (a *App) handleListMaintenance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vReg := r.URL.Query().Get("vehicle_reg")
	status := r.URL.Query().Get("status")

	query := `SELECT id, vehicle_reg, description, cost, TO_CHAR(start_date, 'YYYY-MM-DD'), TO_CHAR(end_date, 'YYYY-MM-DD'), status, created_at, updated_at
		FROM maintenance_logs WHERE 1=1`
	args := []any{}
	argIdx := 1

	if vReg != "" {
		query += fmt.Sprintf(" AND vehicle_reg = $%d", argIdx)
		args = append(args, vReg)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	query += " ORDER BY id DESC"

	cursorStr := r.URL.Query().Get("cursor")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsedL, err := strconv.Atoi(l); err == nil && parsedL > 0 && parsedL <= 100 {
			limit = parsedL
		}
	}

	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve maintenance logs", nil)
		return
	}
	defer rows.Close()

	items := []maintenance{}
	for rows.Next() {
		var m maintenance
		var startDate, endDate *string
		var createdAt, updatedAt *time.Time
		if err := rows.Scan(&m.ID, &m.VehicleReg, &m.Description, &m.Cost, &startDate, &endDate, &m.Status, &createdAt, &updatedAt); err != nil {
			continue
		}
		if startDate != nil {
			m.StartDate = *startDate
		}
		m.EndDate = endDate
		if createdAt != nil {
			c := createdAt.Format(time.RFC3339)
			m.CreatedAt = &c
		}
		if updatedAt != nil {
			u := updatedAt.Format(time.RFC3339)
			m.UpdatedAt = &u
		}
		items = append(items, m)
	}

	// Apply cursor pagination in memory if cursor provided
	startIdx := 0
	if cursorStr != "" {
		if cID, err := strconv.Atoi(cursorStr); err == nil {
			for idx, item := range items {
				if item.ID < cID {
					startIdx = idx
					break
				}
			}
		}
	}

	endIdx := startIdx + limit
	var nextCursor *string
	if endIdx < len(items) {
		c := strconv.Itoa(items[endIdx-1].ID)
		nextCursor = &c
		items = items[startIdx:endIdx]
	} else if startIdx < len(items) {
		items = items[startIdx:]
	} else if startIdx >= len(items) && len(items) > 0 {
		items = []maintenance{}
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[maintenance]{
		Items:      items,
		NextCursor: nextCursor,
	})
}

// handleGetMaintenance handles GET /api/v1/maintenance/{id}
func (a *App) handleGetMaintenance(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	logID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid maintenance log id.", nil)
		return
	}

	ctx := r.Context()
	var m maintenance
	var startDate, endDate *string
	var createdAt, updatedAt *time.Time
	err = a.db.QueryRow(ctx, `SELECT id, vehicle_reg, description, cost, TO_CHAR(start_date, 'YYYY-MM-DD'), TO_CHAR(end_date, 'YYYY-MM-DD'), status, created_at, updated_at
		FROM maintenance_logs WHERE id = $1`, logID).
		Scan(&m.ID, &m.VehicleReg, &m.Description, &m.Cost, &startDate, &endDate, &m.Status, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Maintenance log not found.", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve maintenance log", nil)
		return
	}

	if startDate != nil {
		m.StartDate = *startDate
	}
	m.EndDate = endDate
	if createdAt != nil {
		c := createdAt.Format(time.RFC3339)
		m.CreatedAt = &c
	}
	if updatedAt != nil {
		u := updatedAt.Format(time.RFC3339)
		m.UpdatedAt = &u
	}

	httpx.WriteJSON(w, http.StatusOK, m)
}

// handleOpenMaintenance handles POST /api/v1/maintenance and /api/v1/maintenance/
// Roles: Fleet Manager, Admin. Requires Idempotency-Key.
func (a *App) handleOpenMaintenance(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Unable to read request body.", nil)
		return
	}

	_, key, bodyHash, proceed := a.checkIdempotency(w, r, bodyBytes)
	if !proceed {
		return
	}

	var req openMaintenanceRequest
	dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
		return
	}

	if strings.TrimSpace(req.VehicleReg) == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Vehicle registration number is required.", nil)
		return
	}

	if strings.TrimSpace(req.Description) == "" || len(req.Description) > 500 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Description is required and must not exceed 500 characters.", nil)
		return
	}

	if req.Cost < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Cost must be greater than or equal to zero.", nil)
		return
	}

	nowUTC := time.Now().UTC()
	startDateStr := nowUTC.Format("2006-01-02")
	if req.StartDate != "" {
		parsedDate, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid start_date format, expected YYYY-MM-DD.", nil)
			return
		}
		// Must not be in the future by more than 1 day
		todayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
		if parsedDate.After(todayUTC.AddDate(0, 0, 1)) {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Start date cannot be more than 1 day in the future.", nil)
			return
		}
		startDateStr = req.StartDate
	}

	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction begin failed.", nil)
		return
	}
	defer tx.Rollback(ctx)

	// Lock vehicle first
	var vStatus string
	var isActive bool
	err = tx.QueryRow(ctx, `SELECT status, is_active FROM vehicles WHERE registration_number = $1 FOR UPDATE`, req.VehicleReg).
		Scan(&vStatus, &isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found.", nil)
			return
		}
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Validation against vehicle state
	if vStatus == "Retired" {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_RETIRED", "Cannot open maintenance on a retired vehicle.", nil)
		return
	}
	if vStatus == "On Trip" {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_NOT_AVAILABLE", "Cannot pull an active vehicle off transit into maintenance.", nil)
		return
	}
	if vStatus == "In Shop" {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_NOT_AVAILABLE", "Vehicle currently has an open maintenance log.", nil)
		return
	}
	if !isActive || vStatus != "Available" {
		httpx.WriteError(w, r, http.StatusConflict, "VEHICLE_NOT_AVAILABLE", fmt.Sprintf("Vehicle is not available for maintenance (status: %s, active: %v).", vStatus, isActive), nil)
		return
	}

	// Insert maintenance log
	var logID int64
	err = tx.QueryRow(ctx, `INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Open', now(), now()) RETURNING id`,
		req.VehicleReg, req.Description, req.Cost, startDateStr).Scan(&logID)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Set vehicle status to In Shop
	_, err = tx.Exec(ctx, `UPDATE vehicles SET status = 'In Shop', updated_at = now() WHERE registration_number = $1`, req.VehicleReg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Audit log
	recordAuditLogTx(ctx, tx, "maintenance", fmt.Sprintf("%d", logID), "open", user.Email, map[string]any{
		"vehicle_reg": req.VehicleReg,
		"cost":        req.Cost,
		"start_date":  startDateStr,
		"description": req.Description,
	})

	resLog := maintenance{
		ID:          int(logID),
		VehicleReg:  req.VehicleReg,
		Description: req.Description,
		Cost:        req.Cost,
		StartDate:   startDateStr,
		Status:      "Open",
	}

	respBytes, _ := json.Marshal(resLog)
	if err := saveIdempotency(ctx, tx, key, user.Email, r.URL.Path, bodyHash, http.StatusCreated, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Transactional event publishing via pg_notify
	_ = realtime.PublishTx(ctx, tx, realtime.EventMaintenanceOpened, map[string]any{
		"maintenance_id": logID,
		"vehicle_reg":    req.VehicleReg,
	})
	_ = realtime.PublishTx(ctx, tx, realtime.EventVehicleStatusChange, map[string]any{
		"vehicle_reg": req.VehicleReg,
		"status":      "In Shop",
	})

	if err := tx.Commit(ctx); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(respBytes)
}

// handleCloseMaintenance handles POST & PUT /api/v1/maintenance/{id}/close
// Roles: Fleet Manager, Admin. Requires Idempotency-Key.
func (a *App) handleCloseMaintenance(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	logID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid maintenance log id.", nil)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	_, key, bodyHash, proceed := a.checkIdempotency(w, r, bodyBytes)
	if !proceed {
		return
	}

	var req closeMaintenanceRequest
	if len(bodyBytes) > 0 && strings.TrimSpace(string(bodyBytes)) != "{}" {
		dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
			return
		}
	}

	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction begin failed.", nil)
		return
	}
	defer tx.Rollback(ctx)

	// Step 1: Lookup log to find vehicle_reg and start_date
	var vReg, currentStatus string
	var startDateStr string
	err = tx.QueryRow(ctx, `SELECT vehicle_reg, TO_CHAR(start_date, 'YYYY-MM-DD'), status FROM maintenance_logs WHERE id = $1`, logID).
		Scan(&vReg, &startDateStr, &currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Maintenance log not found.", nil)
			return
		}
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Step 2: Lock vehicle first (lock order: Vehicle -> Log)
	var vStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM vehicles WHERE registration_number = $1 FOR UPDATE`, vReg).
		Scan(&vStatus)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Step 3: Lock maintenance log second
	var logRecord maintenance
	var sDate, eDate *string
	err = tx.QueryRow(ctx, `SELECT id, vehicle_reg, description, cost, TO_CHAR(start_date, 'YYYY-MM-DD'), TO_CHAR(end_date, 'YYYY-MM-DD'), status
		FROM maintenance_logs WHERE id = $1 FOR UPDATE`, logID).
		Scan(&logRecord.ID, &logRecord.VehicleReg, &logRecord.Description, &logRecord.Cost, &sDate, &eDate, &logRecord.Status)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	if logRecord.Status != "Open" {
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_TRANSITION", fmt.Sprintf("Cannot close maintenance log with status '%s'. Log is already closed.", logRecord.Status), nil)
		return
	}

	// Step 4: Validate end_date
	nowUTC := time.Now().UTC()
	endDateStr := nowUTC.Format("2006-01-02")
	if req.EndDate != "" {
		if _, err := time.Parse("2006-01-02", req.EndDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid end_date format, expected YYYY-MM-DD.", nil)
			return
		}
		endDateStr = req.EndDate
	}

	// end_date must be >= start_date
	if endDateStr < startDateStr {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "End date must be greater than or equal to start date.", nil)
		return
	}

	// Step 5: Update maintenance log to Closed
	_, err = tx.Exec(ctx, `UPDATE maintenance_logs SET status = 'Closed', end_date = $1, updated_at = now() WHERE id = $2`, endDateStr, logID)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Step 6: Vehicle status transition
	// Check if any other Open log exists for this vehicle
	var otherOpenLogs int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM maintenance_logs WHERE vehicle_reg = $1 AND status = 'Open' AND id != $2`, vReg, logID).
		Scan(&otherOpenLogs)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// If no other open log exists and vehicle is not Retired, set to Available. If Retired, leave Retired.
	if otherOpenLogs == 0 && vStatus != "Retired" {
		_, err = tx.Exec(ctx, `UPDATE vehicles SET status = 'Available', updated_at = now() WHERE registration_number = $1`, vReg)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}
	}

	// Audit log
	recordAuditLogTx(ctx, tx, "maintenance", fmt.Sprintf("%d", logID), "close", user.Email, map[string]any{
		"vehicle_reg":     vReg,
		"end_date":        endDateStr,
		"other_open_logs": otherOpenLogs,
		"vehicle_status":  vStatus,
	})

	respPayload := map[string]any{
		"message":     "Workshop operations finished. Vehicle returned to fleet.",
		"status":      "Closed",
		"id":          logID,
		"vehicle_reg": vReg,
		"end_date":    endDateStr,
	}

	respBytes, _ := json.Marshal(respPayload)
	if err := saveIdempotency(ctx, tx, key, user.Email, r.URL.Path, bodyHash, http.StatusOK, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Transactional event publishing via pg_notify
	_ = realtime.PublishTx(ctx, tx, realtime.EventMaintenanceClosed, map[string]any{
		"maintenance_id": logID,
		"vehicle_reg":    vReg,
	})
	if vStatus == "Available" {
		_ = realtime.PublishTx(ctx, tx, realtime.EventVehicleStatusChange, map[string]any{
			"vehicle_reg": vReg,
			"status":      "Available",
		})
	}

	if err := tx.Commit(ctx); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
