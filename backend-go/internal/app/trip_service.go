package app

import (
	"context"
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
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/jackc/pgx/v5"
)

// Global Lock Order Helper:
// Vehicle first -> Driver -> Trip. Ascending keys within each type.
type lockedResources struct {
	Vehicle struct {
		Reg      string
		Model    string
		Type     string
		Capacity float64
		Odometer float64
		Status   string
		IsActive bool
	}
	Driver struct {
		License    string
		Name       string
		Category   string
		ExpiryDate string
		Status     string
		IsActive   bool
	}
	Trip struct {
		ID              int64
		SourceName      string
		DestinationName string
		VehicleReg      string
		DriverLicense   string
		CargoWeight     float64
		PlannedDistance float64
		Revenue         float64
		Status          string
		StartOdometer   *float64
		FinalOdometer   *float64
		FuelLiters      *float64
		FuelCost        *float64
	}
}

// lockVehicleAndDriver acquires FOR UPDATE locks in global order: Vehicle first, then Driver.
func lockVehicleAndDriver(ctx context.Context, tx pgx.Tx, vehicleReg, driverLicense string) (*lockedResources, error) {
	res := &lockedResources{}

	// 1. Lock vehicle first
	err := tx.QueryRow(ctx, `SELECT registration_number, model, type, max_load_capacity, odometer, status, is_active
		FROM vehicles WHERE registration_number = $1 FOR UPDATE`, vehicleReg).
		Scan(&res.Vehicle.Reg, &res.Vehicle.Model, &res.Vehicle.Type, &res.Vehicle.Capacity, &res.Vehicle.Odometer, &res.Vehicle.Status, &res.Vehicle.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("vehicle_not_found")
		}
		return nil, err
	}

	// 2. Lock driver second
	err = tx.QueryRow(ctx, `SELECT license_number, name, license_category, TO_CHAR(license_expiry_date, 'YYYY-MM-DD'), status, is_active
		FROM drivers WHERE license_number = $1 FOR UPDATE`, driverLicense).
		Scan(&res.Driver.License, &res.Driver.Name, &res.Driver.Category, &res.Driver.ExpiryDate, &res.Driver.Status, &res.Driver.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("driver_not_found")
		}
		return nil, err
	}

	return res, nil
}

// lockTripWithResources looks up the trip's vehicle and driver first without locks,
// then acquires locks in strict global order: Vehicle -> Driver -> Trip.
func lockTripWithResources(ctx context.Context, tx pgx.Tx, tripID int64) (*lockedResources, error) {
	var vReg, dLic string
	err := tx.QueryRow(ctx, `SELECT vehicle_reg, driver_license FROM trips WHERE id = $1`, tripID).Scan(&vReg, &dLic)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("trip_not_found")
		}
		return nil, err
	}

	res, err := lockVehicleAndDriver(ctx, tx, vReg, dLic)
	if err != nil {
		return nil, err
	}

	// 3. Lock trip third
	err = tx.QueryRow(ctx, `SELECT id, source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, fuel_liters, fuel_cost
		FROM trips WHERE id = $1 FOR UPDATE`, tripID).
		Scan(&res.Trip.ID, &res.Trip.SourceName, &res.Trip.DestinationName, &res.Trip.VehicleReg, &res.Trip.DriverLicense,
			&res.Trip.CargoWeight, &res.Trip.PlannedDistance, &res.Trip.Revenue, &res.Trip.Status,
			&res.Trip.StartOdometer, &res.Trip.FinalOdometer, &res.Trip.FuelLiters, &res.Trip.FuelCost)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("trip_not_found")
		}
		return nil, err
	}

	return res, nil
}

// ValidateDispatchRules performs all dispatch rules on locked rows
func validateDispatchRules(source, destination string, cargoWeight float64, v *lockedResources) (int, string, string) {
	if source == destination {
		return http.StatusUnprocessableEntity, "SAME_ENDPOINTS", "Source and destination hubs must be different."
	}

	if !v.Vehicle.IsActive || v.Vehicle.Status != "Available" {
		return http.StatusConflict, "VEHICLE_NOT_AVAILABLE", fmt.Sprintf("Vehicle is not available for dispatch (status: %s, active: %v).", v.Vehicle.Status, v.Vehicle.IsActive)
	}

	if !v.Driver.IsActive || v.Driver.Status != "Available" {
		return http.StatusConflict, "DRIVER_NOT_AVAILABLE", fmt.Sprintf("Driver is not available for dispatch (status: %s, active: %v).", v.Driver.Status, v.Driver.IsActive)
	}

	todayUTC := time.Now().UTC()
	if IsDriverLicenseExpired(v.Driver.ExpiryDate, todayUTC) {
		return http.StatusConflict, "DRIVER_LICENSE_EXPIRED", "Driver license is expired as of dispatch date."
	}

	if !IsLicenseCategoryCompatible(v.Driver.Category, v.Vehicle.Type) {
		return http.StatusConflict, "LICENSE_CATEGORY_MISMATCH", fmt.Sprintf("Driver license category %s is incompatible with vehicle type %s.", v.Driver.Category, v.Vehicle.Type)
	}

	if cargoWeight > v.Vehicle.Capacity {
		return http.StatusConflict, "TRIP_OVERLOADED", fmt.Sprintf("Cargo weight (%.1f kg) exceeds vehicle maximum capacity (%.1f kg).", cargoWeight, v.Vehicle.Capacity)
	}

	return 0, "", ""
}

// ---------------- HANDLERS ----------------

// handleCreateTrip handles POST /api/v1/trips (Draft or Atomic Create & Dispatch)
func (a *App) handleCreateTrip(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Unable to read request body.", nil)
		return
	}

	_, key, bodyHash, proceed := a.checkIdempotency(w, r, bodyBytes)
	if !proceed {
		return
	}

	var req createTripRequest
	dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
		return
	}

	if req.SourceName == req.DestinationName {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "SAME_ENDPOINTS", "Source and destination hubs must be different.", nil)
		return
	}

	if req.CargoWeight <= 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Cargo weight must be greater than zero.", nil)
		return
	}

	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	// Resource-level authorization: Driver can only create trip for their own linked license
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != req.DriverLicense {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers may only create trips assigned to their own license.", nil)
			return
		}
	} else if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleFleetManager {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to create trips.", nil)
		return
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction begin failed.", nil)
		return
	}
	defer tx.Rollback(ctx)

	shouldDispatch := req.Dispatch != nil && *req.Dispatch

	var tripID int64
	var tripStatus string
	var startOdo *float64

	if shouldDispatch {
		// Global lock order: Vehicle -> Driver
		res, err := lockVehicleAndDriver(ctx, tx, req.VehicleReg, req.DriverLicense)
		if err != nil {
			if err.Error() == "vehicle_not_found" || err.Error() == "driver_not_found" {
				httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Assigned vehicle or driver profile not found.", nil)
				return
			}
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		if status, code, msg := validateDispatchRules(req.SourceName, req.DestinationName, req.CargoWeight, res); status != 0 {
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		tripStatus = "Dispatched"
		odo := res.Vehicle.Odometer
		startOdo = &odo

		err = tx.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now()) RETURNING id`,
			req.SourceName, req.DestinationName, req.VehicleReg, req.DriverLicense, req.CargoWeight, req.PlannedDistance, req.Revenue, tripStatus, startOdo).Scan(&tripID)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		// Update vehicle and driver to On Trip
		_, err = tx.Exec(ctx, `UPDATE vehicles SET status = 'On Trip', updated_at = now() WHERE registration_number = $1`, req.VehicleReg)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		_, err = tx.Exec(ctx, `UPDATE drivers SET status = 'On Trip', updated_at = now() WHERE license_number = $1`, req.DriverLicense)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		recordAuditLogTx(ctx, tx, "trips", fmt.Sprintf("%d", tripID), "create_and_dispatch", user.Email, map[string]any{
			"vehicle_reg":    req.VehicleReg,
			"driver_license": req.DriverLicense,
			"start_odometer": startOdo,
		})
	} else {
		// Draft creation: does NOT lock rows and does NOT change status of vehicle or driver
		tripStatus = "Draft"
		err = tx.QueryRow(ctx, `INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now()) RETURNING id`,
			req.SourceName, req.DestinationName, req.VehicleReg, req.DriverLicense, req.CargoWeight, req.PlannedDistance, req.Revenue, tripStatus).Scan(&tripID)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, nil)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		recordAuditLogTx(ctx, tx, "trips", fmt.Sprintf("%d", tripID), "create_draft", user.Email, map[string]any{
			"vehicle_reg":    req.VehicleReg,
			"driver_license": req.DriverLicense,
		})
	}

	responseTrip := trip{
		ID:              int(tripID),
		SourceName:      req.SourceName,
		DestinationName: req.DestinationName,
		VehicleReg:      req.VehicleReg,
		DriverLicense:   req.DriverLicense,
		CargoWeight:     req.CargoWeight,
		PlannedDistance: req.PlannedDistance,
		Revenue:         req.Revenue,
		Status:          tripStatus,
		StartOdometer:   startOdo,
	}

	respBytes, _ := json.Marshal(responseTrip)
	if err := saveIdempotency(ctx, tx, key, user.Email, r.URL.Path, bodyHash, http.StatusCreated, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// PHASE5: publish fleet event after commit
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(respBytes)
}

// handleDispatchTrip dispatches an existing Draft trip
func (a *App) handleDispatchTrip(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	tripID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid trip id.", nil)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	_, key, bodyHash, proceed := a.checkIdempotency(w, r, bodyBytes)
	if !proceed {
		return
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

	// Global lock order: Vehicle -> Driver -> Trip
	res, err := lockTripWithResources(ctx, tx, tripID)
	if err != nil {
		if err.Error() == "trip_not_found" {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip record not found.", nil)
			return
		}
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Resource authorization
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != res.Trip.DriverLicense {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers may only dispatch trips assigned to their own license.", nil)
			return
		}
	} else if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleFleetManager {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to dispatch trips.", nil)
		return
	}

	if res.Trip.Status != "Draft" {
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_TRANSITION", fmt.Sprintf("Cannot dispatch trip with status '%s'. Only Draft trips can be dispatched.", res.Trip.Status), nil)
		return
	}

	if status, code, msg := validateDispatchRules(res.Trip.SourceName, res.Trip.DestinationName, res.Trip.CargoWeight, res); status != 0 {
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	startOdo := res.Vehicle.Odometer
	_, err = tx.Exec(ctx, `UPDATE trips SET status = 'Dispatched', start_odometer = $1, updated_at = now() WHERE id = $2`, startOdo, tripID)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	_, err = tx.Exec(ctx, `UPDATE vehicles SET status = 'On Trip', updated_at = now() WHERE registration_number = $1`, res.Trip.VehicleReg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	_, err = tx.Exec(ctx, `UPDATE drivers SET status = 'On Trip', updated_at = now() WHERE license_number = $1`, res.Trip.DriverLicense)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	recordAuditLogTx(ctx, tx, "trips", fmt.Sprintf("%d", tripID), "dispatch", user.Email, map[string]any{
		"vehicle_reg":    res.Trip.VehicleReg,
		"driver_license": res.Trip.DriverLicense,
		"start_odometer": startOdo,
	})

	responseTrip := trip{
		ID:              int(tripID),
		SourceName:      res.Trip.SourceName,
		DestinationName: res.Trip.DestinationName,
		VehicleReg:      res.Trip.VehicleReg,
		DriverLicense:   res.Trip.DriverLicense,
		CargoWeight:     res.Trip.CargoWeight,
		PlannedDistance: res.Trip.PlannedDistance,
		Revenue:         res.Trip.Revenue,
		Status:          "Dispatched",
		StartOdometer:   &startOdo,
	}

	respBytes, _ := json.Marshal(responseTrip)
	if err := saveIdempotency(ctx, tx, key, user.Email, r.URL.Path, bodyHash, http.StatusOK, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// PHASE5: publish fleet event after commit
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

// handleGetTrip returns a single trip by ID with resource-level authorization
func (a *App) handleGetTrip(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	tripID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid trip id.", nil)
		return
	}

	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	ctx := r.Context()
	var t trip
	var startOdo, finalOdo, fuelLiters, fuelCost *float64
	var createdAt, updatedAt time.Time
	err = a.db.QueryRow(ctx, `SELECT id, source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, fuel_liters, fuel_cost, created_at, updated_at
		FROM trips WHERE id = $1`, tripID).
		Scan(&t.ID, &t.SourceName, &t.DestinationName, &t.VehicleReg, &t.DriverLicense, &t.CargoWeight, &t.PlannedDistance, &t.Revenue, &t.Status, &startOdo, &finalOdo, &fuelLiters, &fuelCost, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Trip record not found.", nil)
			return
		}
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// S-04: Resource-level authorization
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != t.DriverLicense {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers may only view trips assigned to their own license.", nil)
			return
		}
	}

	t.StartOdometer = startOdo
	t.FinalOdometer = finalOdo
	t.FuelLiters = fuelLiters
	t.FuelCost = fuelCost
	t.CreatedAt = createdAt.Format(time.RFC3339)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)

	httpx.WriteJSON(w, http.StatusOK, t)
}
