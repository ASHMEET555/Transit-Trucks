package app

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

type suggestRequest struct {
	CargoWeight     float64 `json:"cargo_weight"`
	Type            string  `json:"type,omitempty"`
	SourceName      string  `json:"source_name,omitempty"`
	DestinationName string  `json:"destination_name,omitempty"`
}

type RankedPair struct {
	VehicleReg          string  `json:"vehicle_reg"`
	VehicleModel        string  `json:"vehicle_model"`
	VehicleType         string  `json:"vehicle_type"`
	DriverLicense       string  `json:"driver_license"`
	DriverName          string  `json:"driver_name"`
	CapacityUtilization float64 `json:"capacity_utilization"`
	Score               float64 `json:"score"`
}

type IneligibleCandidate struct {
	Type   string `json:"type"`   // "vehicle" or "driver"
	ID     string `json:"id"`     // registration_number or license_number
	Reason string `json:"reason"` // OVERLOADED, LICENSE_EXPIRED, DRIVER_UNAVAILABLE, VEHICLE_UNAVAILABLE, LICENSE_CATEGORY_MISMATCH
}

type suggestResponse struct {
	SourceName             string                `json:"source_name,omitempty"`
	DestinationName        string                `json:"destination_name,omitempty"`
	CargoWeight            float64               `json:"cargo_weight"`
	RankedPairs            []RankedPair          `json:"ranked_pairs"`
	Ineligible             []IneligibleCandidate `json:"ineligible"`
	SuggestedVehicleReg    string                `json:"suggested_vehicle_reg"`
	SuggestedDriverLicense string                `json:"suggested_driver_license"`
	SuggestedDriverName    string                `json:"suggested_driver_name"`
	CapacityUtilization    float64               `json:"capacity_utilization"`
	HeuristicScore         float64               `json:"heuristic_score"`
}

// ScoreCandidate calculates a deterministic recommendation score for an eligible pair.
// Formula:
//   utilization = cargoWeight / vehicleCapacity
//   score = (utilization * 100.0) + (driverSafetyScore * 0.6) - (vehicleOdometer / 100000.0)
func ScoreCandidate(cargoWeight, vehicleCapacity, driverSafetyScore, vehicleOdometer float64) float64 {
	if vehicleCapacity <= 0 {
		return -1.0
	}
	util := cargoWeight / vehicleCapacity
	score := (util * 100.0) + (driverSafetyScore * 0.6) - (vehicleOdometer / 100000.0)
	return math.Round(score*100) / 100
}

// handleSuggest handles POST /api/v1/trips/suggest (pure read-only scoring and matching)
func (a *App) handleSuggest(w http.ResponseWriter, r *http.Request) {
	var req suggestRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	if req.CargoWeight <= 0 {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Cargo weight must be greater than zero.", nil)
		return
	}

	if req.SourceName != "" && req.DestinationName != "" && req.SourceName == req.DestinationName {
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "SAME_ENDPOINTS", "Source and destination hubs must be different.", nil)
		return
	}

	ctx := r.Context()

	// 1. Fetch vehicles (read-only)
	rowsV, err := a.db.Query(ctx, `SELECT registration_number, model, type, max_load_capacity, odometer, status, is_active FROM vehicles`)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	defer rowsV.Close()

	type vItem struct {
		reg      string
		model    string
		vType    string
		capacity float64
		odometer float64
		status   string
		isActive bool
	}

	var allVehicles []vItem
	var eligibleVehicles []vItem
	var ineligible []IneligibleCandidate

	for rowsV.Next() {
		var v vItem
		if err := rowsV.Scan(&v.reg, &v.model, &v.vType, &v.capacity, &v.odometer, &v.status, &v.isActive); err != nil {
			continue
		}
		allVehicles = append(allVehicles, v)

		if req.Type != "" && !strings.EqualFold(v.vType, req.Type) {
			continue
		}

		if !v.isActive || v.status != "Available" {
			ineligible = append(ineligible, IneligibleCandidate{
				Type:   "vehicle",
				ID:     v.reg,
				Reason: "VEHICLE_UNAVAILABLE",
			})
			continue
		}

		if v.capacity < req.CargoWeight {
			ineligible = append(ineligible, IneligibleCandidate{
				Type:   "vehicle",
				ID:     v.reg,
				Reason: "OVERLOADED",
			})
			continue
		}

		eligibleVehicles = append(eligibleVehicles, v)
	}

	// 2. Fetch drivers (read-only)
	rowsD, err := a.db.Query(ctx, `SELECT license_number, name, license_category, license_expiry_date::text, safety_score, status, is_active FROM drivers`)
	if err != nil {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return
	}
	defer rowsD.Close()

	type dItem struct {
		license     string
		name        string
		category    string
		expiryDate  string
		safetyScore float64
		status      string
		isActive    bool
	}

	var allDrivers []dItem
	var eligibleDrivers []dItem
	today := time.Now().UTC()

	for rowsD.Next() {
		var d dItem
		if err := rowsD.Scan(&d.license, &d.name, &d.category, &d.expiryDate, &d.safetyScore, &d.status, &d.isActive); err != nil {
			continue
		}
		allDrivers = append(allDrivers, d)

		if !d.isActive || d.status != "Available" {
			ineligible = append(ineligible, IneligibleCandidate{
				Type:   "driver",
				ID:     d.license,
				Reason: "DRIVER_UNAVAILABLE",
			})
			continue
		}

		if IsDriverLicenseExpired(d.expiryDate, today) {
			ineligible = append(ineligible, IneligibleCandidate{
				Type:   "driver",
				ID:     d.license,
				Reason: "LICENSE_EXPIRED",
			})
			continue
		}

		eligibleDrivers = append(eligibleDrivers, d)
	}

	// 3. Match and score eligible candidates
	var ranked []RankedPair

	for _, v := range eligibleVehicles {
		for _, d := range eligibleDrivers {
			if !IsLicenseCategoryCompatible(d.category, v.vType) {
				continue
			}

			score := ScoreCandidate(req.CargoWeight, v.capacity, d.safetyScore, v.odometer)
			util := math.Round((req.CargoWeight/v.capacity)*10000) / 10000

			ranked = append(ranked, RankedPair{
				VehicleReg:          v.reg,
				VehicleModel:        v.model,
				VehicleType:         v.vType,
				DriverLicense:       d.license,
				DriverName:          d.name,
				CapacityUtilization: util,
				Score:               score,
			})
		}
	}

	// Sort ranked candidates: Score DESC, then VehicleReg ASC, then DriverLicense ASC
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if ranked[i].VehicleReg != ranked[j].VehicleReg {
			return ranked[i].VehicleReg < ranked[j].VehicleReg
		}
		return ranked[i].DriverLicense < ranked[j].DriverLicense
	})

	resp := suggestResponse{
		SourceName:      req.SourceName,
		DestinationName: req.DestinationName,
		CargoWeight:     req.CargoWeight,
		RankedPairs:     ranked,
		Ineligible:      ineligible,
	}

	if len(ranked) > 0 {
		top := ranked[0]
		resp.SuggestedVehicleReg = top.VehicleReg
		resp.SuggestedDriverLicense = top.DriverLicense
		resp.SuggestedDriverName = top.DriverName
		resp.CapacityUtilization = top.CapacityUtilization
		resp.HeuristicScore = top.Score
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}
