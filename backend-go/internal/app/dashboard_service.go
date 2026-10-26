package app

import (
	"math"
	"net/http"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

type VehicleKPI struct {
	RegistrationNumber string   `json:"registration_number"`
	Model              string   `json:"model"`
	Type               string   `json:"type"`
	Status             string   `json:"status"`
	AcquisitionCost    float64  `json:"acquisition_cost"`
	Revenue            float64  `json:"revenue"`
	FuelCost           float64  `json:"fuel_cost"`
	MaintenanceCost    float64  `json:"maintenance_cost"`
	OperationalCost    float64  `json:"operational_cost"`
	FuelLiters         float64  `json:"fuel_liters"`
	Distance           float64  `json:"distance"`
	FuelEfficiency     *float64 `json:"fuel_efficiency"`
	ROI                *float64 `json:"roi"`
}

type DashboardKPIsResponse struct {
	ActiveVehicles        int          `json:"active_vehicles"`
	AvailableVehicles     int          `json:"available_vehicles"`
	InShopVehicles        int          `json:"in_shop_vehicles"`
	RetiredVehicles       int          `json:"retired_vehicles"`
	ActiveTrips           int          `json:"active_trips"`
	PendingTrips          int          `json:"pending_trips"`
	CompletedTrips        int          `json:"completed_trips"`
	DriversOnDuty         int          `json:"drivers_on_duty"`
	AvailableDrivers      int          `json:"available_drivers"`
	FleetUtilization      float64      `json:"fleet_utilization"`
	OverallFuelEfficiency *float64     `json:"overall_fuel_efficiency"`
	TotalOperationalCost  float64      `json:"total_operational_cost"`
	TotalRevenue          float64      `json:"total_revenue"`
	TotalDistance         float64      `json:"total_distance"`
	Vehicles              []VehicleKPI `json:"vehicles"`
}

type DriverDashboardResponse struct {
	DriverLicense       string            `json:"driver_license"`
	TripsByStatus       map[string]int    `json:"trips_by_status"`
	TotalCompletedTrips int               `json:"total_completed_trips"`
	TotalDistance       float64           `json:"total_distance"`
	RecentTrips         []DriverTripEntry `json:"recent_trips"`
}

type DriverTripEntry struct {
	ID              int64   `json:"id"`
	SourceName      string  `json:"source_name"`
	DestinationName string  `json:"destination_name"`
	CargoWeight     float64 `json:"cargo_weight"`
	PlannedDistance float64 `json:"planned_distance"`
	Distance        float64 `json:"distance"`
	CompletedAt     string  `json:"completed_at"`
}

// GET /api/v1/dashboard/kpis
func (a *App) handleDashboardKPIs(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role == rbac.RoleDriver {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Drivers are not permitted to view fleet-wide dashboard KPIs.", nil)
		return
	}

	q := r.URL.Query()
	vType := q.Get("type")
	vStatus := q.Get("status")
	fromDate := q.Get("from")
	toDate := q.Get("to")

	// Validate filters
	if vType != "" {
		validTypes := map[string]bool{
			"Van": true, "Box Truck": true, "Flatbed": true, "Semi": true,
			"Light Truck": true, "Medium Truck": true, "Heavy Truck": true, "Refrigerated": true,
		}
		if !validTypes[vType] {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid vehicle type filter.", nil)
			return
		}
	}
	if vStatus != "" {
		validStatuses := map[string]bool{
			"Available": true, "On Trip": true, "In Shop": true, "Retired": true,
		}
		if !validStatuses[vStatus] {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid vehicle status filter.", nil)
			return
		}
	}
	if fromDate != "" {
		if _, err := time.Parse("2006-01-02", fromDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid from date format. Use YYYY-MM-DD.", nil)
			return
		}
	}
	if toDate != "" {
		if _, err := time.Parse("2006-01-02", toDate); err != nil {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Invalid to date format. Use YYYY-MM-DD.", nil)
			return
		}
	}

	var fromArg, toArg *string
	if fromDate != "" {
		fromArg = &fromDate
	}
	if toDate != "" {
		toArg = &toDate
	}

	var typeArg, statusArg *string
	if vType != "" {
		typeArg = &vType
	}
	if vStatus != "" {
		statusArg = &vStatus
	}

	ctx := r.Context()

	// 1. Fleet & Driver Counts (Single Aggregate Query)
	var activeV, availV, inShopV, retiredV int
	var fleetUtil float64
	var dutyD, availD int

	countsQuery := `
		SELECT
			COALESCE(COUNT(*) FILTER (WHERE status = 'On Trip' AND is_active = true), 0) as active_v,
			COALESCE(COUNT(*) FILTER (WHERE status = 'Available' AND is_active = true), 0) as avail_v,
			COALESCE(COUNT(*) FILTER (WHERE status = 'In Shop' AND is_active = true), 0) as in_shop_v,
			COALESCE(COUNT(*) FILTER (WHERE status = 'Retired' OR is_active = false), 0) as retired_v,
			COALESCE(ROUND((COUNT(*) FILTER (WHERE status = 'On Trip' AND is_active = true) * 100.0) /
				NULLIF(COUNT(*) FILTER (WHERE status != 'Retired' AND is_active = true), 0), 2), 0.0) as util
		FROM vehicles
		WHERE ($1::TEXT IS NULL OR type = $1)
		  AND ($2::TEXT IS NULL OR status = $2)`
	err := a.db.QueryRow(ctx, countsQuery, typeArg, statusArg).Scan(&activeV, &availV, &inShopV, &retiredV, &fleetUtil)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve vehicle counts.", nil)
		return
	}

	driverCountsQuery := `
		SELECT
			COALESCE(COUNT(*) FILTER (WHERE status = 'On Trip' AND is_active = true), 0) as duty_d,
			COALESCE(COUNT(*) FILTER (WHERE status = 'Available' AND is_active = true), 0) as avail_d
		FROM drivers`
	err = a.db.QueryRow(ctx, driverCountsQuery).Scan(&dutyD, &availD)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve driver counts.", nil)
		return
	}

	// 2. Trips Aggregates in Date Range
	var activeT, pendingT, completedT int
	var totalRev, totalDist float64

	tripQuery := `
		SELECT
			COALESCE(COUNT(*) FILTER (WHERE t.status = 'Dispatched'), 0) as active_t,
			COALESCE(COUNT(*) FILTER (WHERE t.status = 'Draft'), 0) as pending_t,
			COALESCE(COUNT(*) FILTER (WHERE t.status = 'Completed'
				AND ($1::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE >= $1::DATE)
				AND ($2::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE <= $2::DATE)
			), 0) as completed_t,
			COALESCE(SUM(CASE
				WHEN t.status = 'Completed'
				AND ($1::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE >= $1::DATE)
				AND ($2::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE <= $2::DATE)
				THEN t.revenue ELSE 0 END), 0) as total_rev,
			COALESCE(SUM(CASE
				WHEN t.status = 'Completed'
				AND ($1::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE >= $1::DATE)
				AND ($2::DATE IS NULL OR (COALESCE(t.completed_at, t.updated_at) AT TIME ZONE 'UTC')::DATE <= $2::DATE)
				AND t.final_odometer IS NOT NULL AND t.start_odometer IS NOT NULL AND t.final_odometer >= t.start_odometer
				THEN (t.final_odometer - t.start_odometer)
				ELSE 0
			END), 0) as total_dist
		FROM trips t
		LEFT JOIN vehicles v ON t.vehicle_reg = v.registration_number
		WHERE ($3::TEXT IS NULL OR v.type = $3)
		  AND ($4::TEXT IS NULL OR v.status = $4)`

	err = a.db.QueryRow(ctx, tripQuery, fromArg, toArg, typeArg, statusArg).Scan(&activeT, &pendingT, &completedT, &totalRev, &totalDist)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve trip statistics.", nil)
		return
	}

	// 3. Operational Costs in Range: Expenses + Closed Maintenance Logs
	var totalExpenses, totalFuelLiters float64
	expQuery := `
		SELECT
			COALESCE(SUM(e.cost), 0),
			COALESCE(SUM(CASE WHEN e.type = 'Fuel' THEN e.liters ELSE 0 END), 0)
		FROM expenses e
		LEFT JOIN vehicles v ON e.vehicle_reg = v.registration_number
		WHERE ($1::DATE IS NULL OR e.date >= $1::DATE)
		  AND ($2::DATE IS NULL OR e.date <= $2::DATE)
		  AND ($3::TEXT IS NULL OR v.type = $3)
		  AND ($4::TEXT IS NULL OR v.status = $4)`
	err = a.db.QueryRow(ctx, expQuery, fromArg, toArg, typeArg, statusArg).Scan(&totalExpenses, &totalFuelLiters)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve expenses.", nil)
		return
	}

	var totalClosedMaint float64
	maintCostQuery := `
		SELECT COALESCE(SUM(m.cost), 0)
		FROM maintenance_logs m
		LEFT JOIN vehicles v ON m.vehicle_reg = v.registration_number
		WHERE m.status = 'Closed'
		  AND ($1::DATE IS NULL OR m.end_date >= $1::DATE)
		  AND ($2::DATE IS NULL OR m.end_date <= $2::DATE)
		  AND ($3::TEXT IS NULL OR v.type = $3)
		  AND ($4::TEXT IS NULL OR v.status = $4)`
	err = a.db.QueryRow(ctx, maintCostQuery, fromArg, toArg, typeArg, statusArg).Scan(&totalClosedMaint)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve maintenance cost.", nil)
		return
	}

	totalOpCost := totalExpenses + totalClosedMaint

	var overallFuelEff *float64
	if totalFuelLiters > 0 {
		eff := math.Round((totalDist/totalFuelLiters)*100) / 100
		overallFuelEff = &eff
	}

	// 4. Per-Vehicle Analytics (Single SQL Query with Aggregations)
	vAnalyticsQuery := `
		SELECT
			v.registration_number,
			v.model,
			v.type,
			v.status,
			v.acquisition_cost,
			COALESCE(t.rev, 0) as revenue,
			COALESCE(t.dist, 0) as distance,
			COALESCE(e.fuel_cost, 0) as fuel_cost,
			COALESCE(e.fuel_liters, 0) as fuel_liters,
			COALESCE(m.maint_cost, 0) + COALESCE(e.maint_exp_cost, 0) as maintenance_cost,
			COALESCE(e.total_exp, 0) + COALESCE(m.maint_cost, 0) as operational_cost
		FROM vehicles v
		LEFT JOIN (
			SELECT
				vehicle_reg,
				SUM(revenue) as rev,
				SUM(CASE
					WHEN final_odometer IS NOT NULL AND start_odometer IS NOT NULL AND final_odometer >= start_odometer
						THEN (final_odometer - start_odometer)
					ELSE 0
				END) as dist
			FROM trips
			WHERE status = 'Completed'
			  AND ($1::DATE IS NULL OR (COALESCE(completed_at, updated_at) AT TIME ZONE 'UTC')::DATE >= $1::DATE)
			  AND ($2::DATE IS NULL OR (COALESCE(completed_at, updated_at) AT TIME ZONE 'UTC')::DATE <= $2::DATE)
			GROUP BY vehicle_reg
		) t ON t.vehicle_reg = v.registration_number
		LEFT JOIN (
			SELECT
				vehicle_reg,
				SUM(CASE WHEN type = 'Fuel' THEN cost ELSE 0 END) as fuel_cost,
				SUM(CASE WHEN type = 'Fuel' THEN liters ELSE 0 END) as fuel_liters,
				SUM(CASE WHEN type IN ('Maintenance', 'Maintenance-Surge') THEN cost ELSE 0 END) as maint_exp_cost,
				SUM(cost) as total_exp
			FROM expenses
			WHERE ($1::DATE IS NULL OR date >= $1::DATE)
			  AND ($2::DATE IS NULL OR date <= $2::DATE)
			GROUP BY vehicle_reg
		) e ON e.vehicle_reg = v.registration_number
		LEFT JOIN (
			SELECT
				vehicle_reg,
				SUM(cost) as maint_cost
			FROM maintenance_logs
			WHERE status = 'Closed'
			  AND ($1::DATE IS NULL OR end_date >= $1::DATE)
			  AND ($2::DATE IS NULL OR end_date <= $2::DATE)
			GROUP BY vehicle_reg
		) m ON m.vehicle_reg = v.registration_number
		WHERE v.is_active = true
		  AND ($3::TEXT IS NULL OR v.type = $3)
		  AND ($4::TEXT IS NULL OR v.status = $4)
		ORDER BY v.registration_number ASC`

	rows, err := a.db.Query(ctx, vAnalyticsQuery, fromArg, toArg, typeArg, statusArg)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to compute vehicle analytics.", nil)
		return
	}
	defer rows.Close()

	var vehicleList []VehicleKPI
	for rows.Next() {
		var item VehicleKPI
		var vDist, vFuelLit float64
		if err := rows.Scan(
			&item.RegistrationNumber,
			&item.Model,
			&item.Type,
			&item.Status,
			&item.AcquisitionCost,
			&item.Revenue,
			&vDist,
			&item.FuelCost,
			&vFuelLit,
			&item.MaintenanceCost,
			&item.OperationalCost,
		); err != nil {
			httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to scan vehicle analytics row.", nil)
			return
		}
		item.Distance = roundToTwoDecimals(vDist)
		item.FuelLiters = roundToTwoDecimals(vFuelLit)
		item.Revenue = roundToTwoDecimals(item.Revenue)
		item.FuelCost = roundToTwoDecimals(item.FuelCost)
		item.MaintenanceCost = roundToTwoDecimals(item.MaintenanceCost)
		item.OperationalCost = roundToTwoDecimals(item.OperationalCost)

		if vFuelLit > 0 {
			eff := roundToTwoDecimals(vDist / vFuelLit)
			item.FuelEfficiency = &eff
		}
		if item.AcquisitionCost > 0 {
			roiVal := roundToTwoDecimals(((item.Revenue - (item.FuelCost + item.MaintenanceCost)) / item.AcquisitionCost) * 100.0)
			item.ROI = &roiVal
		}

		vehicleList = append(vehicleList, item)
	}
	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to iterate vehicle analytics rows.", nil)
		return
	}

	resp := DashboardKPIsResponse{
		ActiveVehicles:        activeV,
		AvailableVehicles:     availV,
		InShopVehicles:        inShopV,
		RetiredVehicles:       retiredV,
		ActiveTrips:           activeT,
		PendingTrips:          pendingT,
		CompletedTrips:        completedT,
		DriversOnDuty:         dutyD,
		AvailableDrivers:      availD,
		FleetUtilization:      fleetUtil,
		OverallFuelEfficiency: overallFuelEff,
		TotalOperationalCost:  roundToTwoDecimals(totalOpCost),
		TotalRevenue:          roundToTwoDecimals(totalRev),
		TotalDistance:         roundToTwoDecimals(totalDist),
		Vehicles:              vehicleList,
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

// GET /api/v1/dashboard/me (Driver-only self view)
func (a *App) handleDriverSelfDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role != rbac.RoleDriver {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only drivers may access the driver self dashboard.", nil)
		return
	}
	if user.DriverLicense == "" {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "User does not have an associated driver license.", nil)
		return
	}

	ctx := r.Context()

	// 1. Trips count by status
	statusCounts := map[string]int{
		"Draft":      0,
		"Dispatched": 0,
		"Completed":  0,
		"Cancelled":  0,
	}
	rowsCounts, err := a.db.Query(ctx, `
		SELECT status, COUNT(*)
		FROM trips
		WHERE driver_license = $1
		GROUP BY status`, user.DriverLicense)
	if err == nil {
		defer rowsCounts.Close()
		for rowsCounts.Next() {
			var st string
			var cnt int
			if err := rowsCounts.Scan(&st, &cnt); err == nil {
				statusCounts[st] = cnt
			}
		}
	}

	// 2. Total completed distance
	var totalDist float64
	_ = a.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(
			CASE
				WHEN final_odometer IS NOT NULL AND start_odometer IS NOT NULL AND final_odometer >= start_odometer
					THEN (final_odometer - start_odometer)
				ELSE planned_distance
			END
		), 0)
		FROM trips
		WHERE driver_license = $1 AND status = 'Completed'`, user.DriverLicense).Scan(&totalDist)

	// 3. Last 20 completed trips
	tripRows, err := a.db.Query(ctx, `
		SELECT
			id,
			source_name,
			destination_name,
			cargo_weight,
			planned_distance,
			CASE
				WHEN final_odometer IS NOT NULL AND start_odometer IS NOT NULL AND final_odometer >= start_odometer
					THEN (final_odometer - start_odometer)
				ELSE planned_distance
			END as actual_dist,
			COALESCE(completed_at, updated_at) as comp_at
		FROM trips
		WHERE driver_license = $1 AND status = 'Completed'
		ORDER BY COALESCE(completed_at, updated_at) DESC, id DESC
		LIMIT 20`, user.DriverLicense)

	var recentTrips []DriverTripEntry
	if err == nil {
		defer tripRows.Close()
		for tripRows.Next() {
			var entry DriverTripEntry
			var compAt time.Time
			if err := tripRows.Scan(&entry.ID, &entry.SourceName, &entry.DestinationName, &entry.CargoWeight, &entry.PlannedDistance, &entry.Distance, &compAt); err == nil {
				entry.CompletedAt = compAt.Format(time.RFC3339)
				entry.Distance = roundToTwoDecimals(entry.Distance)
				entry.PlannedDistance = roundToTwoDecimals(entry.PlannedDistance)
				entry.CargoWeight = roundToTwoDecimals(entry.CargoWeight)
				recentTrips = append(recentTrips, entry)
			}
		}
	}

	resp := DriverDashboardResponse{
		DriverLicense:       user.DriverLicense,
		TripsByStatus:       statusCounts,
		TotalCompletedTrips: statusCounts["Completed"],
		TotalDistance:       roundToTwoDecimals(totalDist),
		RecentTrips:         recentTrips,
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}
