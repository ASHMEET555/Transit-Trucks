package app

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	Secret string
	Logger *slog.Logger
}
type App struct {
	db      *pgxpool.Pool
	secret  string
	log     *slog.Logger
	mux     *http.ServeMux
	clients map[*websocket.Conn]bool
	cmu     sync.Mutex
}

func New(db *pgxpool.Pool, c Config) *App {
	a := &App{db: db, secret: c.Secret, log: c.Logger, mux: http.NewServeMux(), clients: map[*websocket.Conn]bool{}}
	a.routes()
	return a
}
func (a *App) ListenAndServe(addr string) error { return http.ListenAndServe(addr, a.cors(a.mux)) }
func (a *App) routes() {
	a.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		out(w, 200, map[string]any{"status": "online", "system": "TransitOps Backend Architecture Operational"})
	})
	a.mux.HandleFunc("GET /ws/fleet", a.ws)
	a.mux.HandleFunc("POST /api/auth/login", a.login)
	a.mux.HandleFunc("POST /api/auth/register", a.register)
	a.mux.HandleFunc("GET /api/fleet/vehicles", a.auth(a.vehicles))
	a.mux.HandleFunc("POST /api/fleet/vehicles", a.role([]string{"Fleet Manager"}, a.createVehicle))
	a.mux.HandleFunc("GET /api/fleet/vehicles/{reg}", a.auth(a.vehicle))
	a.mux.HandleFunc("PUT /api/fleet/vehicles/{reg}", a.role([]string{"Fleet Manager"}, a.updateVehicle))
	a.mux.HandleFunc("DELETE /api/fleet/vehicles/{reg}", a.role([]string{"Fleet Manager"}, a.deleteVehicle))
	a.mux.HandleFunc("GET /api/fleet/drivers", a.auth(a.drivers))
	a.mux.HandleFunc("POST /api/fleet/drivers", a.role([]string{"Fleet Manager", "Safety Officer"}, a.createDriver))
	a.mux.HandleFunc("GET /api/fleet/drivers/{license}", a.auth(a.driver))
	a.mux.HandleFunc("PUT /api/fleet/drivers/{license}", a.role([]string{"Fleet Manager", "Safety Officer"}, a.updateDriver))
	a.mux.HandleFunc("DELETE /api/fleet/drivers/{license}", a.role([]string{"Fleet Manager"}, a.deleteDriver))
	a.mux.HandleFunc("GET /api/destinations/", a.auth(a.destinations))
	a.mux.HandleFunc("POST /api/destinations/", a.role([]string{"Fleet Manager"}, a.createDestination))
	a.mux.HandleFunc("DELETE /api/destinations/{name}", a.role([]string{"Fleet Manager"}, a.deleteDestination))
	a.mux.HandleFunc("GET /api/trips/", a.auth(a.trips))
	a.mux.HandleFunc("POST /api/trips/suggest", a.role([]string{"Fleet Manager", "Driver"}, a.suggest))
	a.mux.HandleFunc("POST /api/trips/", a.role([]string{"Fleet Manager", "Driver"}, a.dispatch))
	a.mux.HandleFunc("PUT /api/trips/{id}/complete", a.auth(a.complete))
	a.mux.HandleFunc("PUT /api/trips/{id}/cancel", a.auth(a.cancel))
	a.mux.HandleFunc("GET /api/maintenance/", a.auth(a.maintenance))
	a.mux.HandleFunc("POST /api/maintenance/", a.role([]string{"Fleet Manager"}, a.openMaintenance))
	a.mux.HandleFunc("PUT /api/maintenance/{id}/close", a.role([]string{"Fleet Manager"}, a.closeMaintenance))
	a.mux.HandleFunc("POST /api/finance/expenses", a.role([]string{"Financial Analyst", "Fleet Manager"}, a.expense))
	a.mux.HandleFunc("GET /api/finance/expenses/{id}", a.auth(a.expenses))
	a.mux.HandleFunc("GET /api/dashboard/kpis", a.auth(a.kpis))
}
func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func out(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, s int, d string) { out(w, s, map[string]any{"detail": d}) }
func decode(r *http.Request, v any) bool          { return json.NewDecoder(r.Body).Decode(v) == nil }

func (a *App) vehicles(w http.ResponseWriter, r *http.Request) {
	v, e := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles")
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, v)
}
func (a *App) vehicle(w http.ResponseWriter, r *http.Request) {
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", r.PathValue("reg"))
	if e != nil {
		fail(w, 404, "Vehicle not found")
		return
	}
	out(w, 200, v)
}
func (a *App) createVehicle(w http.ResponseWriter, r *http.Request) {
	var v vehicle
	if !decode(r, &v) {
		fail(w, 422, "Invalid request body")
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM vehicles WHERE registration_number=$1", v.RegistrationNumber).Scan(&n)
	if n > 0 {
		fail(w, 400, "Vehicle Registration Number must be unique.")
		return
	}
	if v.Status == "" {
		v.Status = "Available"
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO vehicles VALUES($1,$2,$3,$4,$5,$6,$7)", v.RegistrationNumber, v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 201, v)
}
func (a *App) updateVehicle(w http.ResponseWriter, r *http.Request) {
	var v vehicle
	if !decode(r, &v) {
		fail(w, 422, "Invalid request body")
		return
	}
	ct, e := a.db.Exec(r.Context(), "UPDATE vehicles SET model=$1,type=$2,max_load_capacity=$3,odometer=$4,acquisition_cost=$5,status=$6 WHERE registration_number=$7", v.Model, v.Type, v.MaxLoadCapacity, v.Odometer, v.AcquisitionCost, v.Status, r.PathValue("reg"))
	if e != nil || ct.RowsAffected() == 0 {
		fail(w, 404, "Vehicle not found or modification failed")
		return
	}
	out(w, 200, map[string]string{"message": "Vehicle updated successfully"})
}
func (a *App) deleteVehicle(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM vehicles WHERE registration_number=$1", r.PathValue("reg"))
	if e != nil || ct.RowsAffected() == 0 {
		fail(w, 404, "Vehicle not found")
		return
	}
	out(w, 200, map[string]string{"message": "Vehicle deleted successfully"})
}
func (a *App) drivers(w http.ResponseWriter, r *http.Request) {
	v, e := rows[driver](r.Context(), a.db, "SELECT * FROM drivers")
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, v)
}
func (a *App) driver(w http.ResponseWriter, r *http.Request) {
	v, e := one[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE license_number=$1", r.PathValue("license"))
	if e != nil {
		fail(w, 404, "Driver profile not found")
		return
	}
	out(w, 200, v)
}
func (a *App) createDriver(w http.ResponseWriter, r *http.Request) {
	var v driver
	if !decode(r, &v) {
		fail(w, 422, "Invalid request body")
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM drivers WHERE license_number=$1", v.LicenseNumber).Scan(&n)
	if n > 0 {
		fail(w, 400, "Driver License Number already registered.")
		return
	}
	if v.Status == "" {
		v.Status = "Available"
	}
	if v.SafetyScore == 0 {
		v.SafetyScore = 100
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO drivers VALUES($1,$2,$3,$4,$5,$6,$7)", v.LicenseNumber, v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 201, v)
}
func (a *App) updateDriver(w http.ResponseWriter, r *http.Request) {
	var v driver
	if !decode(r, &v) {
		fail(w, 422, "Invalid request body")
		return
	}
	ct, e := a.db.Exec(r.Context(), "UPDATE drivers SET name=$1,license_category=$2,license_expiry_date=$3,contact_number=$4,safety_score=$5,status=$6 WHERE license_number=$7", v.Name, v.LicenseCategory, v.LicenseExpiryDate, v.ContactNumber, v.SafetyScore, v.Status, r.PathValue("license"))
	if e != nil || ct.RowsAffected() == 0 {
		fail(w, 404, "Driver profile not found or modification failed")
		return
	}
	out(w, 200, map[string]string{"message": "Driver updated successfully"})
}
func (a *App) deleteDriver(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM drivers WHERE license_number=$1", r.PathValue("license"))
	if e != nil || ct.RowsAffected() == 0 {
		fail(w, 404, "Driver not found")
		return
	}
	out(w, 200, map[string]string{"message": "Driver profile purged successfully"})
}

func (a *App) destinations(w http.ResponseWriter, r *http.Request) {
	x, e := rows[destination](r.Context(), a.db, "SELECT * FROM destinations")
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, x)
}
func (a *App) createDestination(w http.ResponseWriter, r *http.Request) {
	var x destination
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	_, e := a.db.Exec(r.Context(), "INSERT INTO destinations VALUES($1,$2)", x.Name, x.Region)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, x)
}
func (a *App) deleteDestination(w http.ResponseWriter, r *http.Request) {
	ct, e := a.db.Exec(r.Context(), "DELETE FROM destinations WHERE name=$1", r.PathValue("name"))
	if e != nil || ct.RowsAffected() == 0 {
		fail(w, 404, "Destination hub not found")
		return
	}
	out(w, 200, map[string]string{"message": "Destination deleted successfully"})
}

func (a *App) trips(w http.ResponseWriter, r *http.Request) {
	x, e := rows[trip](r.Context(), a.db, "SELECT * FROM trips")
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, x)
}
func (a *App) suggest(w http.ResponseWriter, r *http.Request) {
	var x trip
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	if x.SourceName == x.DestinationName {
		fail(w, 400, "Source and destination must be different.")
		return
	}
	vs, _ := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE status='Available' AND max_load_capacity >= $1", x.CargoWeight)
	if len(vs) == 0 {
		fail(w, 404, "No available vehicle can carry this cargo.")
		return
	}
	ds, _ := rows[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE status='Available' AND license_expiry_date > $1", time.Now().Format("2006-01-02"))
	if len(ds) == 0 {
		fail(w, 404, "No available and compliant driver found.")
		return
	}
	var bv vehicle
	var bd driver
	best := -math.MaxFloat64
	var util float64
	for _, v := range vs {
		u := x.CargoWeight / v.MaxLoadCapacity
		for _, d := range ds {
			s := u*100 + d.SafetyScore*.6 - v.Odometer/100000 - (v.MaxLoadCapacity-x.CargoWeight)/1000
			if s > best {
				best = s
				bv = v
				bd = d
				util = u
			}
		}
	}
	out(w, 200, map[string]any{"source_name": x.SourceName, "destination_name": x.DestinationName, "cargo_weight": x.CargoWeight, "suggested_vehicle_reg": bv.RegistrationNumber, "suggested_driver_license": bd.LicenseNumber, "suggested_driver_name": bd.Name, "capacity_utilization": math.Round(util*10000) / 10000, "heuristic_score": math.Round(best*100) / 100})
}
func (a *App) dispatch(w http.ResponseWriter, r *http.Request) {
	var x trip
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	d, de := one[driver](r.Context(), a.db, "SELECT * FROM drivers WHERE license_number=$1", x.DriverLicense)
	if e != nil || de != nil {
		fail(w, 404, "Assigned Vehicle or Driver profile not found.")
		return
	}
	if x.CargoWeight > v.MaxLoadCapacity {
		fail(w, 400, fmt.Sprintf("Overload Error: Cargo (%vkg) exceeds max vehicle capacity (%vkg)[cite: 1].", x.CargoWeight, v.MaxLoadCapacity))
		return
	}
	if v.Status == "In Shop" || v.Status == "Retired" {
		fail(w, 400, fmt.Sprintf("Vehicle cannot be dispatched. Current Status: %s[cite: 1].", v.Status))
		return
	}
	if v.Status == "On Trip" {
		fail(w, 400, "Conflict: Vehicle is already actively deployed on another trip[cite: 1].")
		return
	}
	if d.LicenseExpiryDate <= time.Now().Format("2006-01-02") {
		fail(w, 400, "Compliance Failure: Driver license has expired[cite: 1].")
		return
	}
	if d.Status == "Suspended" {
		fail(w, 400, "Compliance Failure: Assigned driver is currently suspended[cite: 1].")
		return
	}
	if d.Status == "On Trip" {
		fail(w, 400, "Conflict: Driver is already assigned to an active trip[cite: 1].")
		return
	}
	x.Status = "Dispatched"
	e = a.db.QueryRow(r.Context(), "INSERT INTO trips(source_name,destination_name,vehicle_reg,driver_license,cargo_weight,planned_distance,revenue,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id", x.SourceName, x.DestinationName, x.VehicleReg, x.DriverLicense, x.CargoWeight, x.PlannedDistance, x.Revenue, x.Status).Scan(&x.ID)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='On Trip' WHERE registration_number=$1", x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='On Trip' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.dispatched", map[string]any{"trip_id": x.ID, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Dispatched"})
	out(w, 201, x)
}
func (a *App) complete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[trip](r.Context(), a.db, "SELECT * FROM trips WHERE id=$1", id)
	if e != nil {
		fail(w, 404, "Trip record not found.")
		return
	}
	if x.Status != "Dispatched" {
		fail(w, 400, "Only actively dispatched transits can be finalized.")
		return
	}
	n, _ := strconv.ParseFloat(r.URL.Query().Get("final_odometer"), 64)
	v, _ := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	if n <= v.Odometer {
		fail(w, 400, fmt.Sprintf("Odometer reading must be higher than starting values (%v).", v.Odometer))
		return
	}
	a.db.Exec(r.Context(), "UPDATE trips SET status='Completed' WHERE id=$1", id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET odometer=$1,status='Available' WHERE registration_number=$2", n, x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='Available' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.completed", map[string]any{"trip_id": id, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Completed", "final_odometer": n})
	out(w, 200, map[string]string{"message": "Trip completed successfully. Resources released back to active pools[cite: 1]."})
}
func (a *App) cancel(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[trip](r.Context(), a.db, "SELECT * FROM trips WHERE id=$1", id)
	if e != nil {
		fail(w, 404, "Trip record not found.")
		return
	}
	if x.Status != "Dispatched" {
		fail(w, 400, "Only actively dispatched transits can be cancelled.")
		return
	}
	a.db.Exec(r.Context(), "UPDATE trips SET status='Cancelled' WHERE id=$1", id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='Available' WHERE registration_number=$1", x.VehicleReg)
	a.db.Exec(r.Context(), "UPDATE drivers SET status='Available' WHERE license_number=$1", x.DriverLicense)
	a.broadcast("trip.cancelled", map[string]any{"trip_id": id, "vehicle_reg": x.VehicleReg, "driver_license": x.DriverLicense, "status": "Cancelled"})
	out(w, 200, map[string]string{"message": "Trip cancelled. Vehicle and driver restored to Available."})
}

func (a *App) maintenance(w http.ResponseWriter, r *http.Request) {
	x, e := rows[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs")
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, x)
}
func (a *App) openMaintenance(w http.ResponseWriter, r *http.Request) {
	var x maintenance
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	v, e := one[vehicle](r.Context(), a.db, "SELECT * FROM vehicles WHERE registration_number=$1", x.VehicleReg)
	if e != nil {
		fail(w, 404, "Vehicle not found.")
		return
	}
	if v.Status == "On Trip" {
		fail(w, 400, "Cannot pull an active vehicle off transit into maintenance.")
		return
	}
	x.Status = "Open"
	e = a.db.QueryRow(r.Context(), "INSERT INTO maintenance_logs(vehicle_reg,description,cost,start_date,status) VALUES($1,$2,$3,$4,'Open') RETURNING id", x.VehicleReg, x.Description, x.Cost, x.StartDate).Scan(&x.ID)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='In Shop' WHERE registration_number=$1", x.VehicleReg)
	a.broadcast("maintenance.opened", map[string]any{"log_id": x.ID, "vehicle_reg": x.VehicleReg, "status": "Open"})
	out(w, 200, x)
}
func (a *App) closeMaintenance(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	x, e := one[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs WHERE id=$1", id)
	if e != nil {
		fail(w, 404, "Maintenance log index not found.")
		return
	}
	if x.Status == "Closed" {
		fail(w, 400, "Work order is already closed.")
		return
	}
	var body struct {
		EndDate string `json:"end_date"`
	}
	if !decode(r, &body) {
		fail(w, 422, "Invalid request body")
		return
	}
	a.db.Exec(r.Context(), "UPDATE maintenance_logs SET status='Closed',end_date=$1 WHERE id=$2", body.EndDate, id)
	a.db.Exec(r.Context(), "UPDATE vehicles SET status='Available' WHERE registration_number=$1", x.VehicleReg)
	a.broadcast("maintenance.closed", map[string]any{"log_id": id, "vehicle_reg": x.VehicleReg, "status": "Closed"})
	out(w, 200, map[string]string{"message": "Workshop operations finished. Vehicle returned to fleet."})
}

func (a *App) expense(w http.ResponseWriter, r *http.Request) {
	var x expenseRow
	if !decode(r, &x) {
		fail(w, 422, "Invalid request body")
		return
	}
	ok := map[string]bool{"Fuel": true, "Toll": true, "Maintenance-Surge": true, "Other": true}
	if !ok[x.Type] {
		fail(w, 400, "Invalid expense type. Must be one of: [Fuel Maintenance-Surge Other Toll]")
		return
	}
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM trips WHERE id=$1", x.TripID).Scan(&n)
	if n == 0 {
		fail(w, 404, "Trip not found for expense logging.")
		return
	}
	if x.Type == "Fuel" && (x.Liters == nil || *x.Liters <= 0) {
		fail(w, 400, "Fuel expenses require liters > 0.")
		return
	}
	e := a.db.QueryRow(r.Context(), "INSERT INTO expenses(trip_id,type,liters,cost,date) VALUES($1,$2,$3,$4,$5) RETURNING id", x.TripID, x.Type, x.Liters, x.Cost, x.Date).Scan(&x.ID)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	a.broadcast("expense.created", map[string]any{"expense_id": x.ID, "trip_id": x.TripID, "type": x.Type, "cost": x.Cost})
	out(w, 201, x)
}
func (a *App) expenses(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var n int
	a.db.QueryRow(r.Context(), "SELECT count(*) FROM trips WHERE id=$1", id).Scan(&n)
	if n == 0 {
		fail(w, 404, "Trip not found.")
		return
	}
	x, e := rows[expenseRow](r.Context(), a.db, "SELECT * FROM expenses WHERE trip_id=$1", id)
	if e != nil {
		fail(w, 500, "Internal Server Error")
		return
	}
	out(w, 200, x)
}
func (a *App) kpis(w http.ResponseWriter, r *http.Request) {
	vs, _ := rows[vehicle](r.Context(), a.db, "SELECT * FROM vehicles")
	ds, _ := rows[driver](r.Context(), a.db, "SELECT * FROM drivers")
	ts, _ := rows[trip](r.Context(), a.db, "SELECT * FROM trips")
	ms, _ := rows[maintenance](r.Context(), a.db, "SELECT * FROM maintenance_logs")
	active, available, shop, retired := 0, 0, 0, 0
	for _, v := range vs {
		switch v.Status {
		case "On Trip":
			active++
		case "Available":
			available++
		case "In Shop":
			shop++
		case "Retired":
			retired++
		}
	}
	at, pt, ct, duty, ad := 0, 0, 0, 0, 0
	for _, t := range ts {
		if t.Status == "Dispatched" {
			at++
		}
		if t.Status == "Draft" {
			pt++
		}
		if t.Status == "Completed" {
			ct++
		}
	}
	for _, d := range ds {
		if d.Status == "On Trip" {
			duty++
		}
		if d.Status == "Available" {
			ad++
		}
	}
	var tf, td, tc, tr float64
	analytics := []map[string]any{}
	for _, v := range vs {
		var rev, dist, fc, fl, mc float64
		for _, t := range ts {
			if t.VehicleReg == v.RegistrationNumber {
				rev += t.Revenue
				dist += t.PlannedDistance
				es, _ := rows[expenseRow](r.Context(), a.db, "SELECT * FROM expenses WHERE trip_id=$1", t.ID)
				for _, e := range es {
					if e.Type == "Fuel" {
						fc += e.Cost
						if e.Liters != nil {
							fl += *e.Liters
						}
					}
				}
			}
		}
		for _, m := range ms {
			if m.VehicleReg == v.RegistrationNumber {
				mc += m.Cost
			}
		}
		op := fc + mc
		var roi any = nil
		var eff any = nil
		if v.AcquisitionCost > 0 {
			roi = (rev - op) / v.AcquisitionCost
		}
		if fl > 0 {
			eff = dist / fl
		}
		tf += fl
		td += dist
		tc += op
		tr += rev
		analytics = append(analytics, map[string]any{"registration_number": v.RegistrationNumber, "model": v.Model, "type": v.Type, "status": v.Status, "acquisition_cost": v.AcquisitionCost, "revenue": rev, "fuel_cost": fc, "maintenance_cost": mc, "operational_cost": op, "fuel_liters": fl, "distance": dist, "fuel_efficiency": eff, "roi": roi})
	}
	var util float64
	if len(vs)-retired > 0 {
		util = math.Round(float64(active)/float64(len(vs)-retired)*10000) / 100
	}
	var oe any = nil
	if tf > 0 {
		oe = td / tf
	}
	out(w, 200, map[string]any{"active_vehicles": active, "available_vehicles": available, "in_shop_vehicles": shop, "retired_vehicles": retired, "active_trips": at, "pending_trips": pt, "completed_trips": ct, "drivers_on_duty": duty, "available_drivers": ad, "fleet_utilization": util, "overall_fuel_efficiency": oe, "total_operational_cost": tc, "total_revenue": tr, "vehicles": analytics})
}
func (a *App) ws(w http.ResponseWriter, r *http.Request) {
	c, e := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(w, r, nil)
	if e != nil {
		return
	}
	a.cmu.Lock()
	a.clients[c] = true
	a.cmu.Unlock()
	defer func() { a.cmu.Lock(); delete(a.clients, c); a.cmu.Unlock(); c.Close() }()
	for {
		if _, _, e := c.ReadMessage(); e != nil {
			return
		}
	}
}
func (a *App) broadcast(event string, payload any) {
	b, _ := json.Marshal(map[string]any{"event": event, "payload": payload})
	a.cmu.Lock()
	defer a.cmu.Unlock()
	for c := range a.clients {
		if e := c.WriteMessage(websocket.TextMessage, b); e != nil {
			delete(a.clients, c)
			c.Close()
		}
	}
}
