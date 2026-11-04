package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
)

// simEnabled reports whether simulator-source ingest is accepted.
func simEnabled() bool {
	return os.Getenv("SIM_ENABLED") == "1"
}

type positionRequest struct {
	VehicleReg     string    `json:"vehicle_reg"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	HeadingDeg     float64   `json:"heading_deg"`
	SpeedKmh       float64   `json:"speed_kmh"`
	RecordedAt     time.Time `json:"recorded_at"`
	Source         string    `json:"source"`
	FuelPct        *float64  `json:"fuel_pct,omitempty"`
	OdometerKm     *float64  `json:"odometer_km,omitempty"`
	ReeferTempC    *float64  `json:"reefer_temp_c,omitempty"`
	DrivingMinutes *int      `json:"driving_minutes,omitempty"`
}

func (a *App) handlePostPosition(w http.ResponseWriter, r *http.Request) {
	var req positionRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	if req.VehicleReg == "" {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "vehicle_reg is required", map[string]any{"field": "vehicle_reg"})
		return
	}
	if req.Source == "device" {
		httpx.WriteError(w, r, http.StatusForbidden, "DEVICE_INGEST_NOT_ENABLED", "device ingest is not enabled", nil)
		return
	}
	if req.Source == "simulator" && !simEnabled() {
		httpx.WriteError(w, r, http.StatusForbidden, "SIMULATOR_DISABLED", "simulator ingest is disabled (SIM_ENABLED!=1)", nil)
		return
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Transaction begin failed.", nil)
		return
	}
	defer tx.Rollback(ctx)

	var vehicleExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM vehicles WHERE registration_number = $1)`,
		req.VehicleReg).Scan(&vehicleExists); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Vehicle lookup failed.", nil)
		return
	}
	if !vehicleExists {
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found", nil)
		return
	}

	duplicate, err := IngestPosition(ctx, tx, PositionInput{
		VehicleReg:     req.VehicleReg,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		HeadingDeg:     req.HeadingDeg,
		SpeedKmh:       req.SpeedKmh,
		RecordedAt:     req.RecordedAt,
		Source:         req.Source,
		FuelPct:        req.FuelPct,
		OdometerKm:     req.OdometerKm,
		ReeferTempC:    req.ReeferTempC,
		DrivingMinutes: req.DrivingMinutes,
	})
	if err != nil {
		var ingErr *IngestError
		if errors.As(err, &ingErr) {
			if ingErr.Code == "DEVICE_INGEST_NOT_ENABLED" {
				httpx.WriteError(w, r, http.StatusForbidden, ingErr.Code, ingErr.Message, map[string]any{"field": ingErr.Field})
				return
			}
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, ingErr.Code, ingErr.Message, map[string]any{"field": ingErr.Field})
			return
		}
		status, code, msg := httpx.MapDBError(err, a.log)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}

	_ = realtime.PublishTx(ctx, tx, realtime.EventVehiclePosition, map[string]any{
		"vehicle_reg": req.VehicleReg,
	})

	if err := tx.Commit(ctx); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to commit transaction", nil)
		return
	}

	if duplicate {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"duplicate": true})
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"duplicate": false})
}

// liveVehicle is one row of the live fleet board.
type liveVehicle struct {
	VehicleReg     string    `json:"vehicle_reg"`
	TripID         int64     `json:"trip_id"`
	Origin         namedRef  `json:"origin"`
	Destination    namedRef  `json:"destination"`
	Latitude       *float64  `json:"latitude"`
	Longitude      *float64  `json:"longitude"`
	HeadingDeg     *float64  `json:"heading_deg"`
	SpeedKmh       *float64  `json:"speed_kmh"`
	RecordedAt     *string   `json:"recorded_at"`
	AgeSeconds     *int64    `json:"age_seconds"`
	Source         *string   `json:"source"`
	ProgressM      *float64  `json:"progress_m"`
	RouteLengthM   *float64  `json:"route_length_m"`
	ProgressPct    *float64  `json:"progress_pct"`
	RoutePolyline  *string   `json:"route_polyline"`
	RouteProvider  *string   `json:"route_provider"`
	Status         string    `json:"status"` // on_route, at_stop, delayed, off_route, breakdown, no_signal
	Freshness      string    `json:"freshness"` // fresh, stale, old, no_position
	Alerts         []string  `json:"alerts"`
	Reason         *string   `json:"reason,omitempty"`
	DriverName     *string   `json:"driver_name,omitempty"`
	DriverLicense  *string   `json:"driver_license,omitempty"`
	CargoWeight    float64   `json:"cargo_weight"`
	CargoType      string    `json:"cargo_type"`
	ETA            string    `json:"eta"`
	MilesRemaining float64   `json:"miles_remaining"`
	DelayMinutes   int       `json:"delay_minutes"`
	FuelPct        *float64  `json:"fuel_pct,omitempty"`
	ReeferTempC    *float64  `json:"reefer_temp_c,omitempty"`
	DrivingMinutes *int      `json:"driving_minutes,omitempty"`
}

type namedRef struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

func strptr(s string) *string { return &s }

func (a *App) handleLiveVehicles(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	ctx := r.Context()
	query := `
		SELECT t.id, t.source_name, t.destination_name, t.vehicle_reg,
			d1.latitude, d1.longitude, d2.latitude, d2.longitude,
			p.latitude, p.longitude, p.heading_deg, p.speed_kmh, p.recorded_at, p.source,
			p.fuel_pct, p.odometer_km, p.reefer_temp_c, p.driving_minutes,
			COALESCE(t.cargo_weight, 0), COALESCE(t.planned_distance, 0),
			COALESCE(v.type, 'Semi'), COALESCE(v.status, 'On Trip'),
			dr.name, dr.license_number
		FROM trips t
		LEFT JOIN destinations d1 ON d1.name = t.source_name
		LEFT JOIN destinations d2 ON d2.name = t.destination_name
		LEFT JOIN vehicles v ON v.registration_number = t.vehicle_reg
		LEFT JOIN drivers dr ON dr.license_number = t.driver_license
		LEFT JOIN LATERAL (
			SELECT latitude, longitude, heading_deg, speed_kmh, recorded_at, source,
				fuel_pct, odometer_km, reefer_temp_c, driving_minutes
			FROM vehicle_positions
			WHERE vehicle_reg = t.vehicle_reg
			ORDER BY recorded_at DESC
			LIMIT 1
		) p ON true
		WHERE t.status = 'Dispatched'`
	var args []any
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []liveVehicle{}})
			return
		}
		query += ` AND t.driver_license = $1`
		args = append(args, user.DriverLicense)
	}
	query += ` ORDER BY t.vehicle_reg ASC`

	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch live vehicles", nil)
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := []liveVehicle{}
	for rows.Next() {
		var (
			tripID              int64
			src, dst, reg       string
			srcLat, srcLon      *float64
			dstLat, dstLon      *float64
			lat, lon, head, spd *float64
			recAt               *time.Time
			source              *string
			fuelPct             *float64
			odometerKm          *float64
			reeferTempC         *float64
			drivingMinutes      *int
			cargoWeight         float64
			plannedDistance     float64
			vType, vStatus      string
			drName, drLic       *string
		)
		if err := rows.Scan(&tripID, &src, &dst, &reg, &srcLat, &srcLon, &dstLat, &dstLon,
			&lat, &lon, &head, &spd, &recAt, &source,
			&fuelPct, &odometerKm, &reeferTempC, &drivingMinutes,
			&cargoWeight, &plannedDistance,
			&vType, &vStatus,
			&drName, &drLic); err != nil {
			continue
		}
		lv := liveVehicle{
			VehicleReg:     reg,
			TripID:         tripID,
			Origin:         namedRef{Name: src, Latitude: srcLat, Longitude: srcLon},
			Destination:    namedRef{Name: dst, Latitude: dstLat, Longitude: dstLon},
			Status:         "no_signal",
			Freshness:      "no_position",
			Alerts:         []string{},
			CargoWeight:    cargoWeight,
			CargoType:      vType,
			FuelPct:        fuelPct,
			ReeferTempC:    reeferTempC,
			DrivingMinutes: drivingMinutes,
		}

		if user.Role != rbac.RoleSafetyOfficer && user.Role != rbac.RoleFinancialAnalyst {
			lv.DriverName = drName
			lv.DriverLicense = drLic
		}

		if recAt != nil {
			ts := recAt.UTC().Format(time.RFC3339)
			age := int64(now.Sub(recAt.UTC()).Seconds())
			if age < 0 {
				age = 0
			}
			lv.Latitude = lat
			lv.Longitude = lon
			lv.HeadingDeg = head
			lv.SpeedKmh = spd
			lv.RecordedAt = &ts
			lv.AgeSeconds = &age
			lv.Source = source
			switch {
			case age < 60:
				lv.Freshness = "fresh"
			case age < 30*60:
				lv.Freshness = "stale"
			default:
				lv.Freshness = "old"
			}
			lv.ProgressM, lv.RouteLengthM, lv.ProgressPct, lv.Reason = nil, nil, nil, nil
			rp := a.progressFor(ctx, src, dst, *lat, *lon)
			lv.ProgressM, lv.RouteLengthM, lv.ProgressPct = rp.progress, rp.length, rp.pct
			lv.RoutePolyline, lv.RouteProvider, lv.Reason = rp.polyline, rp.provider, rp.reason

			var remainingM float64
			if rp.length != nil && rp.progress != nil {
				if *rp.length > *rp.progress {
					remainingM = *rp.length - *rp.progress
				}
			} else if plannedDistance > 0 {
				remainingM = plannedDistance * 1000
			}
			lv.MilesRemaining = remainingM / 1609.34

			speed := 85.0
			if spd != nil && *spd > 15 {
				speed = *spd
			}
			remHours := (remainingM / 1000.0) / speed
			etaTime := now.Add(time.Duration(remHours*3600) * time.Second)
			lv.ETA = etaTime.Format("15:04")

			plannedHours := plannedDistance / 85.0
			if plannedHours <= 0 && rp.length != nil {
				plannedHours = (*rp.length / 1000.0) / 85.0
			}
			if plannedHours > 0 && remHours > plannedHours*0.7 {
				diffMin := int((remHours - plannedHours*0.5) * 60)
				if diffMin > 0 {
					lv.DelayMinutes = diffMin
				}
			}

			if age > 1800 {
				lv.Status = "no_signal"
			} else if vStatus == "In Shop" || (spd != nil && *spd == 0 && fuelPct != nil && *fuelPct <= 0) {
				lv.Status = "breakdown"
			} else if rp.distToRoute != nil && *rp.distToRoute > 500 {
				lv.Status = "off_route"
			} else if spd != nil && *spd < 3.0 && (lv.ProgressPct == nil || *lv.ProgressPct < 2 || *lv.ProgressPct > 98) {
				lv.Status = "at_stop"
			} else if lv.DelayMinutes > 15 {
				lv.Status = "delayed"
			} else {
				lv.Status = "on_route"
			}

			alerts := []string{}
			if lv.DelayMinutes > 30 {
				alerts = append(alerts, "late_arrival")
			}
			if spd != nil && *spd > 105.0 {
				alerts = append(alerts, "speeding")
			}
			if fuelPct != nil && *fuelPct < 15.0 {
				alerts = append(alerts, "low_fuel")
			}
			isReefer := vType == "Refrigerated" || strings.Contains(strings.ToLower(vType), "reefer")
			if isReefer && reeferTempC != nil && *reeferTempC > -15.0 {
				alerts = append(alerts, "reefer_temp_out_of_range")
			}
			if drivingMinutes != nil && *drivingMinutes >= 600 {
				alerts = append(alerts, "hos_near_limit")
			}
			lv.Alerts = alerts
		}
		items = append(items, lv)
	}

	body, _ := json.Marshal(map[string]any{"items": items})
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// routeProgress carries the projection result plus the raw cached geometry so
// the frontend draws the exact backend route without a second request.
type routeProgress struct {
	progress    *float64
	length      *float64
	pct         *float64
	distToRoute *float64
	polyline    *string
	provider    *string
	reason      *string
}

// progressFor projects a position onto the cached route polyline. On any
// geometry/projection failure it returns nil progress with a reason and never
// an error, so the live endpoint cannot fail because of routing data.
func (a *App) progressFor(ctx context.Context, origin, destination string, lat, lon float64) routeProgress {
	var poly, provider *string
	if err := a.db.QueryRow(ctx,
		`SELECT polyline6, provider FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`,
		origin, destination).Scan(&poly, &provider); err != nil || poly == nil || *poly == "" {
		return routeProgress{reason: strptr("no_route_geometry")}
	}
	pts, err := DecodePolyline6(*poly)
	if err != nil || len(pts) < 2 {
		return routeProgress{polyline: poly, provider: provider, reason: strptr("invalid_route_geometry")}
	}
	progress, total, distToRoute, ok := ProjectOntoRouteWithDistance(lat, lon, pts)
	if !ok {
		return routeProgress{polyline: poly, provider: provider, reason: strptr("projection_failed")}
	}
	pct := 0.0
	if total > 0 {
		pct = progress / total * 100
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
	}
	return routeProgress{progress: &progress, length: &total, pct: &pct, distToRoute: &distToRoute, polyline: poly, provider: provider}
}
