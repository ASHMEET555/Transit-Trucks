package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
)

// handleCancelTrip moves Draft or Dispatched -> Cancelled
func (a *App) handleCancelTrip(w http.ResponseWriter, r *http.Request) {
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
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Resource-level authorization
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" || user.DriverLicense != res.Trip.DriverLicense {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers may only cancel trips assigned to their own license.", nil)
			return
		}
	} else if user.Role != rbac.RoleAdmin && user.Role != rbac.RoleFleetManager {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to cancel trips.", nil)
		return
	}

	if res.Trip.Status != "Draft" && res.Trip.Status != "Dispatched" {
		httpx.WriteError(w, r, http.StatusConflict, "INVALID_TRANSITION", fmt.Sprintf("Cannot cancel trip with status '%s'. Only Draft or Dispatched trips can be cancelled.", res.Trip.Status), nil)
		return
	}

	wasDispatched := res.Trip.Status == "Dispatched"

	// Update Trip to Cancelled
	_, err = tx.Exec(ctx, `UPDATE trips SET status = 'Cancelled', updated_at = now() WHERE id = $1`, tripID)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// If it was Dispatched, release resources back to Available
	if wasDispatched {
		_, err = tx.Exec(ctx, `UPDATE vehicles SET status = 'Available', updated_at = now() WHERE registration_number = $1`, res.Trip.VehicleReg)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, a.log)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}

		_, err = tx.Exec(ctx, `UPDATE drivers SET status = 'Available', updated_at = now() WHERE license_number = $1`, res.Trip.DriverLicense)
		if err != nil {
			status, code, msg := httpx.MapDBError(err, a.log)
			httpx.WriteError(w, r, status, code, msg, nil)
			return
		}
	}

	// Audit log
	recordAuditLogTx(ctx, tx, "trips", fmt.Sprintf("%d", tripID), "cancel", user.Email, map[string]any{
		"previous_status": res.Trip.Status,
		"vehicle_reg":     res.Trip.VehicleReg,
		"driver_license":  res.Trip.DriverLicense,
		"released":        wasDispatched,
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
		Status:          "Cancelled",
		StartOdometer:   res.Trip.StartOdometer,
	}

	respBytes, _ := json.Marshal(responseTrip)
	if err := saveIdempotency(ctx, tx, key, user.ID, r.URL.Path, bodyHash, http.StatusOK, string(respBytes)); err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	// Transactional event publishing via pg_notify
	_ = realtime.PublishTx(ctx, tx, realtime.EventTripCancelled, map[string]any{
		"trip_id":        tripID,
		"vehicle_reg":    res.Trip.VehicleReg,
		"driver_license": res.Trip.DriverLicense,
	})
	if wasDispatched {
		_ = realtime.PublishTx(ctx, tx, realtime.EventVehicleStatusChange, map[string]any{
			"vehicle_reg": res.Trip.VehicleReg,
			"status":      "Available",
		})
		_ = realtime.PublishTx(ctx, tx, realtime.EventDriverStatusChange, map[string]any{
			"driver_license": res.Trip.DriverLicense,
			"status":         "Available",
		})
	}

	if err := tx.Commit(ctx); err != nil {
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
