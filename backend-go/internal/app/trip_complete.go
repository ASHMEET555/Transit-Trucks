package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

// handleCompleteTrip moves Dispatched -> Completed
func (a *App) handleCompleteTrip(w http.ResponseWriter, r *http.Request) {
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

	var req completeTripRequest
	if len(bodyBytes) > 0 && strings.TrimSpace(string(bodyBytes)) != "{}" {
		dec := json.NewDecoder(strings.NewReader(string(bodyBytes)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid request body format.", nil)
			return
		}
	}

	// Fallback to query parameter if final_odometer not in body
	if req.FinalOdometer <= 0 && r.URL.Query().Get("final_odometer") != "" {
		if odo, err := strconv.ParseFloat(r.URL.Query().Get("final_odometer"), 64); err == nil {
			req.FinalOdometer = odo
		}
	}

	// Fuel validation: both must be present together and both > 0
	hasFuelLiters := req.FuelLiters != nil
	hasFuelCost := req.FuelCost != nil
	if hasFuelLiters != hasFuelCost {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Fuel liters and fuel cost must both be provided together.", nil)
		return
	}
	if hasFuelLiters && (*req.FuelLiters <= 0 || *req.FuelCost <= 0) {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Fuel liters and fuel cost must both be greater than zero.", nil)
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

	// Resource-level authorization
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != res.Trip.DriverLicense {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers may only complete trips assigned to their own license.", nil)
			return
		}
	} else if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleFleetManager {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to complete trips.", nil)
		return
	}

	if res.Trip.Status != "Dispatched" {
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_TRANSITION", fmt.Sprintf("Cannot complete trip with status '%s'. Only Dispatched trips can be completed.", res.Trip.Status), nil)
		return
	}

	startOdo := 0.0
	if res.Trip.StartOdometer != nil {
		startOdo = *res.Trip.StartOdometer
	} else {
		startOdo = res.Vehicle.Odometer
	}

	if req.FinalOdometer <= startOdo {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_ODOMETER", fmt.Sprintf("Final odometer reading (%.1f) must be higher than start odometer (%.1f).", req.FinalOdometer, startOdo), nil)
		return
	}

	// Update Trip to Completed
	_, err = tx.Exec(ctx, `UPDATE trips SET status = 'Completed', final_odometer = $1, fuel_liters = $2, fuel_cost = $3, updated_at = now()
		WHERE id = $4`, req.FinalOdometer, req.FuelLiters, req.FuelCost, tripID)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Update Vehicle master odometer and restore status to Available
	_, err = tx.Exec(ctx, `UPDATE vehicles SET odometer = $1, status = 'Available', updated_at = now() WHERE registration_number = $2`, req.FinalOdometer, res.Trip.VehicleReg)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Restore Driver status to Available
	_, err = tx.Exec(ctx, `UPDATE drivers SET status = 'Available', updated_at = now() WHERE license_number = $1`, res.Trip.DriverLicense)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Audit log
	recordAuditLogTx(ctx, tx, "trips", fmt.Sprintf("%d", tripID), "complete", user.Email, map[string]any{
		"vehicle_reg":    res.Trip.VehicleReg,
		"driver_license": res.Trip.DriverLicense,
		"final_odometer": req.FinalOdometer,
		"fuel_liters":    req.FuelLiters,
		"fuel_cost":      req.FuelCost,
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
		Status:          "Completed",
		StartOdometer:   &startOdo,
		FinalOdometer:   &req.FinalOdometer,
		FuelLiters:      req.FuelLiters,
		FuelCost:        req.FuelCost,
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
