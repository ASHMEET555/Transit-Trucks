package app

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

// GET /api/v1/dashboard/export.csv
func (a *App) handleDashboardExportCSV(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role == rbac.RoleDriver || user.Role == rbac.RoleSafetyOfficer {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions to export dashboard data.", nil)
		return
	}

	q := r.URL.Query()
	fromDate := q.Get("from")
	toDate := q.Get("to")
	vehicleReg := q.Get("vehicle_reg")

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

	ctx := r.Context()

	var fromArg, toArg, regArg *string
	if fromDate != "" {
		fromArg = &fromDate
	}
	if toDate != "" {
		toArg = &toDate
	}
	if vehicleReg != "" {
		regArg = &vehicleReg
	}

	exportQuery := `
		SELECT
			v.registration_number,
			v.model,
			v.type,
			v.status,
			v.acquisition_cost,
			COALESCE(t.rev, 0) as revenue,
			COALESCE(e.fuel_cost, 0) as fuel_cost,
			COALESCE(m.maint_cost, 0) + COALESCE(e.maint_exp_cost, 0) as maintenance_cost,
			COALESCE(e.total_exp, 0) + COALESCE(m.maint_cost, 0) as operational_cost,
			COALESCE(e.fuel_liters, 0) as fuel_liters,
			COALESCE(t.dist, 0) as distance
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
		  AND ($3::TEXT IS NULL OR v.registration_number = $3)
		ORDER BY v.registration_number ASC`

	rows, err := a.db.Query(ctx, exportQuery, fromArg, toArg, regArg)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query export data.", nil)
		return
	}
	defer rows.Close()

	today := time.Now().UTC().Format("2006-01-02")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"dashboard-%s.csv\"", today))

	flusher, canFlush := w.(http.Flusher)

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	header := []string{
		"Registration Number",
		"Model",
		"Type",
		"Status",
		"Acquisition Cost",
		"Revenue",
		"Fuel Cost",
		"Maintenance Cost",
		"Operational Cost",
		"Fuel Liters",
		"Distance (km)",
		"Fuel Efficiency (km/L)",
		"ROI (%)",
	}
	_ = csvWriter.Write(header)
	csvWriter.Flush()
	if canFlush {
		flusher.Flush()
	}

	for rows.Next() {
		var reg, model, vType, status string
		var acqCost, rev, fCost, mCost, opCost, fLiters, dist float64
		if err := rows.Scan(&reg, &model, &vType, &status, &acqCost, &rev, &fCost, &mCost, &opCost, &fLiters, &dist); err != nil {
			continue
		}

		effStr := "N/A"
		if fLiters > 0 {
			effStr = fmt.Sprintf("%.2f", dist/fLiters)
		}

		roiStr := "N/A"
		if acqCost > 0 {
			roiStr = fmt.Sprintf("%.2f", ((rev-opCost)/acqCost)*100.0)
		}

		row := []string{
			sanitizeCSVCell(reg),
			sanitizeCSVCell(model),
			sanitizeCSVCell(vType),
			sanitizeCSVCell(status),
			fmt.Sprintf("%.2f", acqCost),
			fmt.Sprintf("%.2f", rev),
			fmt.Sprintf("%.2f", fCost),
			fmt.Sprintf("%.2f", mCost),
			fmt.Sprintf("%.2f", opCost),
			fmt.Sprintf("%.2f", fLiters),
			fmt.Sprintf("%.2f", dist),
			effStr,
			roiStr,
		}
		_ = csvWriter.Write(row)
		csvWriter.Flush()
		if canFlush {
			flusher.Flush()
		}
	}
}

// sanitizeCSVCell protects against CSV formula injection by prefixing '=', '+', '-', '@', '\t', '\r' with single quote
func sanitizeCSVCell(cell string) string {
	if len(cell) > 0 {
		switch cell[0] {
		case '=', '+', '-', '@', '\t', '\r':
			return "'" + cell
		}
	}
	return cell
}
