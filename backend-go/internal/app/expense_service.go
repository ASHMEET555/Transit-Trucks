package app

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
)

type createExpenseRequest struct {
	TripID     *int64   `json:"trip_id,omitempty"`
	VehicleReg *string  `json:"vehicle_reg,omitempty"`
	Type       string   `json:"type"`
	Amount     *float64 `json:"amount,omitempty"`
	Cost       *float64 `json:"cost,omitempty"`
	Liters     *float64 `json:"liters,omitempty"`
	Date       *string  `json:"date,omitempty"`
}

type VehicleFinanceSummary struct {
	VehicleReg            string   `json:"vehicle_reg"`
	FuelExpenses          float64  `json:"fuel_expenses"`
	MaintenanceExpenses   float64  `json:"maintenance_expenses"`
	OtherExpenses         float64  `json:"other_expenses"`
	ClosedMaintenanceCost float64  `json:"closed_maintenance_cost"`
	TotalDistance         float64  `json:"total_distance"`
	CostPerKm             *float64 `json:"cost_per_km"`
}

type FinanceSummaryResponse struct {
	From     *string                 `json:"from,omitempty"`
	To       *string                 `json:"to,omitempty"`
	Vehicles []VehicleFinanceSummary `json:"vehicles"`
	Items    []VehicleFinanceSummary `json:"items"`
	Totals   VehicleFinanceSummary   `json:"totals"`
}

// POST /api/v1/expenses and POST /api/v1/finance/expenses
func (a *App) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	_, key, bodyHash, proceed := a.checkIdempotency(w, r, bodyBytes)
	if !proceed {
		return
	}

	var req createExpenseRequest
	dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
		return
	}

	// Validate Type
	validTypes := map[string]bool{
		"Fuel":        true,
		"Toll":        true,
		"Maintenance": true,
		"Other":       true,
	}
	if !validTypes[req.Type] {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid expense type. Must be one of: Fuel, Toll, Maintenance, Other.", nil)
		return
	}

	// Validate Amount
	var amount float64
	if req.Amount != nil {
		amount = *req.Amount
	} else if req.Cost != nil {
		amount = *req.Cost
	} else {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Amount is required.", nil)
		return
	}
	if amount < 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Amount must be greater than or equal to 0.", nil)
		return
	}

	// Validate Liters
	if req.Type == "Fuel" {
		if req.Liters == nil || *req.Liters <= 0 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Fuel expenses require liters > 0.", nil)
			return
		}
	} else {
		if req.Liters != nil && *req.Liters > 0 {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Liters can only be specified for Fuel expenses.", nil)
			return
		}
		req.Liters = nil
	}

	// Validate Date
	expenseDate := time.Now().UTC().Format("2006-01-02")
	if req.Date != nil && strings.TrimSpace(*req.Date) != "" {
		parsedDate, err := time.Parse("2006-01-02", *req.Date)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Date must be in YYYY-MM-DD format.", nil)
			return
		}
		// Cannot be > 1 day in future
		tomorrow := time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		if parsedDate.Truncate(24 * time.Hour).After(tomorrow) {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Expense date cannot be more than 1 day in the future.", nil)
			return
		}
		expenseDate = parsedDate.Format("2006-01-02")
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction begin failed.", nil)
		return
	}
	defer tx.Rollback(ctx)

	// Validate Trip / Vehicle
	var finalVehicleReg string
	if req.TripID != nil {
		var tripStatus, tripVehicleReg string
		err := tx.QueryRow(ctx, `SELECT status, vehicle_reg FROM trips WHERE id = $1`, *req.TripID).Scan(&tripStatus, &tripVehicleReg)
		if err != nil {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip record not found.", nil)
			return
		}
		if tripStatus == "Draft" || tripStatus == "Cancelled" {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "EXPENSE_ON_INACTIVE_TRIP", "Expenses can only be logged against Dispatched or Completed trips.", nil)
			return
		}
		if req.VehicleReg != nil && strings.TrimSpace(*req.VehicleReg) != "" {
			if *req.VehicleReg != tripVehicleReg {
				httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Provided vehicle_reg does not match trip vehicle registration.", nil)
				return
			}
		}
		finalVehicleReg = tripVehicleReg
	} else if req.VehicleReg != nil && strings.TrimSpace(*req.VehicleReg) != "" {
		finalVehicleReg = strings.TrimSpace(*req.VehicleReg)
		var exists bool
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vehicles WHERE registration_number = $1)`, finalVehicleReg).Scan(&exists)
		if err != nil || !exists {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found.", nil)
			return
		}
	} else {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Either trip_id or vehicle_reg must be provided.", nil)
		return
	}

	var insertedID int
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO expenses (trip_id, vehicle_reg, type, liters, cost, date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::DATE, now(), now())
		RETURNING id, created_at, updated_at`,
		req.TripID, finalVehicleReg, req.Type, req.Liters, amount, expenseDate).
		Scan(&insertedID, &createdAt, &updatedAt)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	recordAuditLogTx(ctx, tx, "expenses", fmt.Sprintf("%d", insertedID), "create", user.Email, map[string]any{
		"trip_id":     req.TripID,
		"vehicle_reg": finalVehicleReg,
		"type":        req.Type,
		"liters":      req.Liters,
		"cost":        amount,
		"date":        expenseDate,
	})

	createdAtStr := createdAt.Format(time.RFC3339)
	updatedAtStr := updatedAt.Format(time.RFC3339)
	var tripIDInt *int
	if req.TripID != nil {
		val := int(*req.TripID)
		tripIDInt = &val
	}

	res := expenseRow{
		ID:         insertedID,
		TripID:     tripIDInt,
		VehicleReg: &finalVehicleReg,
		Type:       req.Type,
		Liters:     req.Liters,
		Cost:       amount,
		Amount:     amount,
		Date:       expenseDate,
		CreatedAt:  &createdAtStr,
		UpdatedAt:  &updatedAtStr,
	}

	respBytes, _ := json.Marshal(res)
	if err := saveIdempotency(ctx, tx, key, user.Email, r.URL.Path, bodyHash, http.StatusCreated, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Transactional event publishing via pg_notify
	_ = realtime.PublishTx(ctx, tx, realtime.EventExpenseCreated, map[string]any{
		"expense_id":  insertedID,
		"trip_id":     req.TripID,
		"vehicle_reg": finalVehicleReg,
		"type":        req.Type,
	})

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction commit failed.", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, res)
}

// GET /api/v1/expenses
func (a *App) handleListExpenses(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role == rbac.RoleDriver || user.Role == rbac.RoleSafetyOfficer {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to view expenses.", nil)
		return
	}

	q := r.URL.Query()
	tripIDStr := q.Get("trip_id")
	if tripIDStr == "" {
		tripIDStr = q.Get("trip")
	}
	vehicleReg := q.Get("vehicle_reg")
	if vehicleReg == "" {
		vehicleReg = q.Get("vehicle")
	}
	expType := q.Get("type")
	fromDate := q.Get("from")
	toDate := q.Get("to")
	cursorStr := q.Get("cursor")

	limit := 20
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	var conditions []string
	var args []any
	argIdx := 1

	if tripIDStr != "" {
		if tripID, err := strconv.ParseInt(tripIDStr, 10, 64); err == nil {
			conditions = append(conditions, fmt.Sprintf("trip_id = $%d", argIdx))
			args = append(args, tripID)
			argIdx++
		}
	}
	if vehicleReg != "" {
		conditions = append(conditions, fmt.Sprintf("vehicle_reg = $%d", argIdx))
		args = append(args, vehicleReg)
		argIdx++
	}
	if expType != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, expType)
		argIdx++
	}
	if fromDate != "" {
		conditions = append(conditions, fmt.Sprintf("date >= $%d::DATE", argIdx))
		args = append(args, fromDate)
		argIdx++
	}
	if toDate != "" {
		conditions = append(conditions, fmt.Sprintf("date <= $%d::DATE", argIdx))
		args = append(args, toDate)
		argIdx++
	}
	if cursorStr != "" {
		if cursorID, err := strconv.Atoi(cursorStr); err == nil {
			conditions = append(conditions, fmt.Sprintf("id < $%d", argIdx))
			args = append(args, cursorID)
			argIdx++
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, trip_id, vehicle_reg, type, liters, cost, TO_CHAR(date, 'YYYY-MM-DD'), created_at, updated_at
		FROM expenses
		%s
		ORDER BY id DESC
		LIMIT $%d`, whereClause, argIdx)
	args = append(args, limit+1)

	ctx := r.Context()
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query expenses.", nil)
		return
	}
	defer rows.Close()

	var items []expenseRow
	for rows.Next() {
		var row expenseRow
		var tripID *int
		var vReg *string
		var liters *float64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&row.ID, &tripID, &vReg, &row.Type, &liters, &row.Cost, &row.Date, &createdAt, &updatedAt); err != nil {
			continue
		}
		row.TripID = tripID
		row.VehicleReg = vReg
		row.Liters = liters
		row.Amount = row.Cost
		cStr := createdAt.Format(time.RFC3339)
		uStr := updatedAt.Format(time.RFC3339)
		row.CreatedAt = &cStr
		row.UpdatedAt = &uStr
		items = append(items, row)
	}

	var nextCursor *string
	if len(items) > limit {
		lastItem := items[limit-1]
		c := fmt.Sprintf("%d", lastItem.ID)
		nextCursor = &c
		items = items[:limit]
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[expenseRow]{
		Items:      items,
		NextCursor: nextCursor,
	})
}

// GET /api/v1/finance/expenses/{id} (Backward compatibility with trip expenses)
func (a *App) handleGetTripExpenses(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role == rbac.RoleDriver || user.Role == rbac.RoleSafetyOfficer {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to view expenses.", nil)
		return
	}

	idVal := r.PathValue("id")
	tripID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid trip id", nil)
		return
	}

	ctx := r.Context()
	rows, err := a.db.Query(ctx, `
		SELECT id, trip_id, vehicle_reg, type, liters, cost, TO_CHAR(date, 'YYYY-MM-DD'), created_at, updated_at
		FROM expenses
		WHERE trip_id = $1
		ORDER BY id ASC`, tripID)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve expenses", nil)
		return
	}
	defer rows.Close()

	var items []expenseRow
	for rows.Next() {
		var row expenseRow
		var tID *int
		var vReg *string
		var liters *float64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&row.ID, &tID, &vReg, &row.Type, &liters, &row.Cost, &row.Date, &createdAt, &updatedAt); err != nil {
			continue
		}
		row.TripID = tID
		row.VehicleReg = vReg
		row.Liters = liters
		row.Amount = row.Cost
		cStr := createdAt.Format(time.RFC3339)
		uStr := updatedAt.Format(time.RFC3339)
		row.CreatedAt = &cStr
		row.UpdatedAt = &uStr
		items = append(items, row)
	}

	httpx.WriteJSON(w, http.StatusOK, items)
}

// GET /api/v1/finance/summary
func (a *App) handleFinanceSummary(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role == rbac.RoleDriver || user.Role == rbac.RoleSafetyOfficer {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to view finance summary.", nil)
		return
	}

	q := r.URL.Query()
	fromDate := q.Get("from")
	toDate := q.Get("to")
	vehicleReg := q.Get("vehicle_reg")

	// Validate date formats if provided
	if fromDate != "" {
		if _, err := time.Parse("2006-01-02", fromDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid 'from' date format. Use YYYY-MM-DD.", nil)
			return
		}
	}
	if toDate != "" {
		if _, err := time.Parse("2006-01-02", toDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid 'to' date format. Use YYYY-MM-DD.", nil)
			return
		}
	}

	ctx := r.Context()

	// 1. Identify which vehicles to report on
	var vehicles []string
	if vehicleReg != "" {
		var exists bool
		err := a.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vehicles WHERE registration_number = $1)`, vehicleReg).Scan(&exists)
		if err != nil || !exists {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found.", nil)
			return
		}
		vehicles = []string{vehicleReg}
	} else {
		rows, err := a.db.Query(ctx, `SELECT registration_number FROM vehicles WHERE is_active = true ORDER BY registration_number ASC`)
		if err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load vehicles", nil)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var reg string
			if err := rows.Scan(&reg); err == nil {
				vehicles = append(vehicles, reg)
			}
		}
	}

	var results []VehicleFinanceSummary
	var totalFuel, totalMaintExp, totalOtherExp, totalMaintLog, totalDist float64

	for _, reg := range vehicles {
		// Expenses query
		expQuery := `
			SELECT
				COALESCE(SUM(CASE WHEN type = 'Fuel' THEN cost ELSE 0 END), 0) as fuel_cost,
				COALESCE(SUM(CASE WHEN type IN ('Maintenance', 'Maintenance-Surge') THEN cost ELSE 0 END), 0) as maint_cost,
				COALESCE(SUM(CASE WHEN type IN ('Toll', 'Other') THEN cost ELSE 0 END), 0) as other_cost
			FROM expenses
			WHERE vehicle_reg = $1`
		var expArgs []any
		expArgs = append(expArgs, reg)
		argIdx := 2
		if fromDate != "" {
			expQuery += fmt.Sprintf(" AND date >= $%d::DATE", argIdx)
			expArgs = append(expArgs, fromDate)
			argIdx++
		}
		if toDate != "" {
			expQuery += fmt.Sprintf(" AND date <= $%d::DATE", argIdx)
			expArgs = append(expArgs, toDate)
			argIdx++
		}

		var fuelCost, maintExpCost, otherCost float64
		_ = a.db.QueryRow(ctx, expQuery, expArgs...).Scan(&fuelCost, &maintExpCost, &otherCost)

		// Closed maintenance logs cost query
		maintLogQuery := `
			SELECT COALESCE(SUM(cost), 0)
			FROM maintenance_logs
			WHERE vehicle_reg = $1 AND status = 'Closed'`
		var maintArgs []any
		maintArgs = append(maintArgs, reg)
		mArgIdx := 2
		if fromDate != "" {
			maintLogQuery += fmt.Sprintf(" AND end_date >= $%d::DATE", mArgIdx)
			maintArgs = append(maintArgs, fromDate)
			mArgIdx++
		}
		if toDate != "" {
			maintLogQuery += fmt.Sprintf(" AND end_date <= $%d::DATE", mArgIdx)
			maintArgs = append(maintArgs, toDate)
			mArgIdx++
		}

		var closedMaintCost float64
		_ = a.db.QueryRow(ctx, maintLogQuery, maintArgs...).Scan(&closedMaintCost)

		// Distance query: completed trips in range
		distQuery := `
			SELECT COALESCE(SUM(
				CASE
					WHEN final_odometer IS NOT NULL AND start_odometer IS NOT NULL AND final_odometer >= start_odometer THEN (final_odometer - start_odometer)
					ELSE planned_distance
				END
			), 0)
			FROM trips
			WHERE vehicle_reg = $1 AND status = 'Completed'`
		var distArgs []any
		distArgs = append(distArgs, reg)
		dArgIdx := 2
		if fromDate != "" {
			distQuery += fmt.Sprintf(" AND (COALESCE(completed_at, updated_at) AT TIME ZONE 'UTC')::date >= $%d::DATE", dArgIdx)
			distArgs = append(distArgs, fromDate)
			dArgIdx++
		}
		if toDate != "" {
			distQuery += fmt.Sprintf(" AND (COALESCE(completed_at, updated_at) AT TIME ZONE 'UTC')::date <= $%d::DATE", dArgIdx)
			distArgs = append(distArgs, toDate)
			dArgIdx++
		}

		var dist float64
		_ = a.db.QueryRow(ctx, distQuery, distArgs...).Scan(&dist)

		var costPerKm *float64
		if dist > 0 {
			totalVehicleCost := fuelCost + maintExpCost + otherCost + closedMaintCost
			val := roundToTwoDecimals(totalVehicleCost / dist)
			costPerKm = &val
		}

		vSummary := VehicleFinanceSummary{
			VehicleReg:            reg,
			FuelExpenses:          roundToTwoDecimals(fuelCost),
			MaintenanceExpenses:   roundToTwoDecimals(maintExpCost),
			OtherExpenses:         roundToTwoDecimals(otherCost),
			ClosedMaintenanceCost: roundToTwoDecimals(closedMaintCost),
			TotalDistance:         roundToTwoDecimals(dist),
			CostPerKm:             costPerKm,
		}
		results = append(results, vSummary)

		totalFuel += fuelCost
		totalMaintExp += maintExpCost
		totalOtherExp += otherCost
		totalMaintLog += closedMaintCost
		totalDist += dist
	}

	var overallCostPerKm *float64
	if totalDist > 0 {
		overallCost := totalFuel + totalMaintExp + totalOtherExp + totalMaintLog
		val := roundToTwoDecimals(overallCost / totalDist)
		overallCostPerKm = &val
	}

	totals := VehicleFinanceSummary{
		VehicleReg:            "TOTAL",
		FuelExpenses:          roundToTwoDecimals(totalFuel),
		MaintenanceExpenses:   roundToTwoDecimals(totalMaintExp),
		OtherExpenses:         roundToTwoDecimals(totalOtherExp),
		ClosedMaintenanceCost: roundToTwoDecimals(totalMaintLog),
		TotalDistance:         roundToTwoDecimals(totalDist),
		CostPerKm:             overallCostPerKm,
	}

	var fromPtr, toPtr *string
	if fromDate != "" {
		fromPtr = &fromDate
	}
	if toDate != "" {
		toPtr = &toDate
	}

	resp := FinanceSummaryResponse{
		From:     fromPtr,
		To:       toPtr,
		Vehicles: results,
		Items:    results,
		Totals:   totals,
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func roundToTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}
