package app

import (
	"math"
	"net/http"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

// AssumedAverageSpeedKMH is the constant speed used to estimate live trip
// progress. There is no GPS or telemetry source; see docs/LIVE_MAP.md.
const AssumedAverageSpeedKMH = 80.0

type LiveTripCoord struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type LiveTrip struct {
	ID                int64         `json:"id"`
	SourceName        string        `json:"source_name"`
	DestinationName   string        `json:"destination_name"`
	VehicleReg        string        `json:"vehicle_reg"`
	DriverLicense     *string       `json:"driver_license,omitempty"`
	Status            string        `json:"status"`
	DispatchedAt      string        `json:"dispatched_at"`
	PlannedDistance   float64       `json:"planned_distance"`
	EstimatedProgress float64       `json:"estimated_progress"`
	Origin            LiveTripCoord `json:"origin"`
	Destination       LiveTripCoord `json:"destination"`
}

type LiveTripsResponse struct {
	Items            []LiveTrip `json:"items"`
	AssumedSpeedKmh  float64    `json:"assumed_speed_kmh"`
	EstimatedWarning string     `json:"estimated_warning"`
}

// GET /api/v1/trips/live — active trips with map coordinates and server-side
// estimated progress. Trips whose source or destination lacks coordinates are
// excluded (they remain visible in trip lists). No driver names are returned.
// Safety Officer and Financial Analyst payloads omit driver_license.
func (a *App) handleLiveTrips(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}

	stripLicense := user.Role == rbac.RoleSafetyOfficer || user.Role == rbac.RoleFinancialAnalyst

	var licArg *string
	if user.Role == rbac.RoleDriver {
		if user.DriverLicense == "" {
			httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "User does not have an associated driver license.", nil)
			return
		}
		licArg = &user.DriverLicense
	}

	query := `
		SELECT t.id, t.source_name, t.destination_name, t.vehicle_reg, t.driver_license,
			t.planned_distance, COALESCE(t.dispatched_at, t.updated_at),
			s.latitude, s.longitude, d.latitude, d.longitude
		FROM trips t
		JOIN destinations s ON s.name = t.source_name
		JOIN destinations d ON d.name = t.destination_name
		WHERE t.status = 'Dispatched'
		  AND s.latitude IS NOT NULL AND d.latitude IS NOT NULL
		  AND ($1::TEXT IS NULL OR t.driver_license = $1)
		ORDER BY t.id ASC`

	rs, err := a.db.Query(r.Context(), query, licArg)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch live trips.", nil)
		return
	}
	defer rs.Close()

	now := time.Now()
	items := []LiveTrip{}
	for rs.Next() {
		var item LiveTrip
		var lic string
		var dispatchedAt time.Time
		var oLat, oLng, dLat, dLng float64
		if err := rs.Scan(&item.ID, &item.SourceName, &item.DestinationName,
			&item.VehicleReg, &lic, &item.PlannedDistance, &dispatchedAt,
			&oLat, &oLng, &dLat, &dLng); err != nil {
			continue
		}
		if !stripLicense {
			l := lic
			item.DriverLicense = &l
		}
		item.Status = "Dispatched"
		item.DispatchedAt = dispatchedAt.Format(time.RFC3339)
		item.Origin = LiveTripCoord{Latitude: oLat, Longitude: oLng}
		item.Destination = LiveTripCoord{Latitude: dLat, Longitude: dLng}
		item.PlannedDistance = roundToTwoDecimals(item.PlannedDistance)
		item.EstimatedProgress = estimateProgress(dispatchedAt, now, item.PlannedDistance)
		items = append(items, item)
	}

	httpx.WriteJSON(w, http.StatusOK, LiveTripsResponse{
		Items:            items,
		AssumedSpeedKmh:  AssumedAverageSpeedKMH,
		EstimatedWarning: "Estimated position, no GPS",
	})
}

func estimateProgress(dispatchedAt, now time.Time, plannedDistance float64) float64 {
	if plannedDistance <= 0 || !now.After(dispatchedAt) {
		return 0
	}
	elapsedHours := now.Sub(dispatchedAt).Hours()
	progress := (elapsedHours * AssumedAverageSpeedKMH) / plannedDistance
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return math.Round(progress*10000) / 10000
}
