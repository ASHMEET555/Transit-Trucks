package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type destRecord struct {
	name   string
	region string
	lat    float64
	lon    float64
}

type simRouteSpec struct {
	origin      string
	destination string
	name        string
}

func main() {
	envVal := strings.ToLower(os.Getenv("ENV"))
	if envVal == "production" {
		fmt.Fprintln(os.Stderr, "Error: seeding is strictly refused when ENV=production (zero rows written)")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "Error: DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}

	seedVal := int64(42)
	if s := os.Getenv("SEED_RANDOM_SEED"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			seedVal = v
		}
	}
	rng := rand.New(rand.NewSource(seedVal))

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	tx, err := db.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: transaction begin failed: %v\n", err)
		os.Exit(1)
	}
	defer tx.Rollback(ctx)

	seedPassEnv := strings.TrimSpace(os.Getenv("SEED_PASSWORD"))
	if seedPassEnv == "" {
		seedPassEnv = strings.TrimSpace(os.Getenv("INITIAL_ADMIN_PASSWORD"))
	}
	if seedPassEnv == "" {
		seedPassEnv = "TransitOps2026!"
	}

	passHash, err := auth.HashPassword(seedPassEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: password hash error: %v\n", err)
		os.Exit(1)
	}

	now := time.Now().UTC()

	// 1. Hubs / Depots: 8 continental depots + 5 endpoint hubs
	hubs := []destRecord{
		{"Chicago Logistics Hub", "Midwest", 41.8781, -87.6298},
		{"Dallas Gateway Hub", "South", 32.7767, -96.7970},
		{"Atlanta Distribution Hub", "Southeast", 33.7490, -84.3880},
		{"Denver Intermodal Hub", "Mountain", 39.7392, -104.9903},
		{"Kansas City Terminal", "Midwest", 39.0997, -94.5786},
		{"Los Angeles Freight Hub", "West", 34.0522, -118.2437},
		{"Reno Hub", "West", 39.5296, -119.8138},
		{"Seattle Terminal", "Northwest", 47.6062, -122.3321},
		// Endpoint destinations for continental routes:
		{"Salt Lake City Hub", "West", 40.7608, -111.8910},
		{"Houston Freight Depot", "South", 29.7604, -95.3698},
		{"Portland Logistics Hub", "Northwest", 45.5152, -122.6784},
		{"New York Central Hub", "Northeast", 40.7128, -74.0060},
		{"Phoenix Transshipment Depot", "Southwest", 33.4484, -112.0740},
		{"Miami Port Hub", "Southeast", 25.7617, -80.1918},
	}

	hubMap := make(map[string]destRecord)
	for _, h := range hubs {
		hubMap[h.name] = h
		_, err = tx.Exec(ctx,
			`INSERT INTO destinations (name, region, latitude, longitude, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $5)
			 ON CONFLICT (name) DO UPDATE SET region = EXCLUDED.region,
			   latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude`,
			h.name, h.region, h.lat, h.lon, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting destination %s: %v\n", h.name, err)
			os.Exit(1)
		}
	}

	// 2. Continental Long-Haul Routes (10 routes from spec)
	simRoutes := []simRouteSpec{
		{"Reno Hub", "Salt Lake City Hub", "I-80 East Corridor"},
		{"Denver Intermodal Hub", "Los Angeles Freight Hub", "I-70 / I-15 Southwest Transit"},
		{"Chicago Logistics Hub", "Atlanta Distribution Hub", "Midwest to Southeast Freight"},
		{"Dallas Gateway Hub", "Houston Freight Depot", "I-45 South Trunk"},
		{"Seattle Terminal", "Portland Logistics Hub", "I-5 Northwest Corridor"},
		{"Kansas City Terminal", "Dallas Gateway Hub", "I-35 Central Corridor"},
		{"Chicago Logistics Hub", "New York Central Hub", "I-80 / I-90 East Coast Trunk"},
		{"Denver Intermodal Hub", "Kansas City Terminal", "I-70 Plains Express"},
		{"Atlanta Distribution Hub", "Dallas Gateway Hub", "I-20 Southern Link"},
		{"Los Angeles Freight Hub", "Phoenix Transshipment Depot", "I-10 Desert Corridor"},
	}

	// Seed / pre-cache OSRM road geometries for all 10 routes
	osrmBase := strings.TrimRight(strings.TrimSpace(os.Getenv("OSRM_URL")), "/")
	if osrmBase == "" {
		osrmBase = "http://localhost:5000"
	}
	for _, sr := range simRoutes {
		orig := hubMap[sr.origin]
		dest := hubMap[sr.destination]

		// Check if already cached
		var exists int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`, sr.origin, sr.destination).Scan(&exists)
		if exists > 0 {
			continue
		}

		res, err := app.FetchRoute(ctx, osrmBase, orig.lat, orig.lon, dest.lat, dest.lon)
		if err != nil {
			// Fallback to public OSRM router
			res, err = app.FetchRoute(ctx, "https://router.project-osrm.org", orig.lat, orig.lon, dest.lat, dest.lon)
		}
		if err != nil {
			// Fallback approximate straight-line
			poly := app.EncodePolyline6([][2]float64{{orig.lat, orig.lon}, {dest.lat, dest.lon}})
			res = app.RouteResult{
				Polyline6: poly,
				DistanceM: 800000,
				DurationS: 36000,
				Provider:  "approximate",
			}
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO route_geometries (origin_id, destination_id, polyline6, distance_m, duration_s, provider)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (origin_id, destination_id) DO UPDATE SET
			   polyline6 = EXCLUDED.polyline6, distance_m = EXCLUDED.distance_m,
			   duration_s = EXCLUDED.duration_s, provider = EXCLUDED.provider`,
			sr.origin, sr.destination, res.Polyline6, res.DistanceM, res.DurationS, res.Provider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error caching route %s -> %s: %v\n", sr.origin, sr.destination, err)
		}
	}

	// 3. Vehicles (50 trucks for simulator demo)
	vehicleTypes := []string{"Semi", "Refrigerated", "Box Truck", "Semi", "Refrigerated"}
	vehicleModels := []string{"Volvo VNL 860", "Freightliner Cascadia", "Peterbilt 579", "Kenworth T680", "Mack Anthem"}

	vehicleRegs := make([]string, 50)
	for i := 1; i <= 50; i++ {
		reg := fmt.Sprintf("VH-%04d", 1000+i)
		vehicleRegs[i-1] = reg

		vType := vehicleTypes[rng.Intn(len(vehicleTypes))]
		model := vehicleModels[rng.Intn(len(vehicleModels))]
		maxLoad := float64(18000 + rng.Intn(12)*1000)
		odometer := float64(45000 + rng.Intn(220000))
		cost := float64(120000 + rng.Intn(70000))
		status := "Available"

		_, err = tx.Exec(ctx,
			`INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, $8)
			 ON CONFLICT (registration_number) DO UPDATE SET
			   model = EXCLUDED.model, type = EXCLUDED.type, max_load_capacity = EXCLUDED.max_load_capacity,
			   odometer = EXCLUDED.odometer, acquisition_cost = EXCLUDED.acquisition_cost, status = EXCLUDED.status,
			   is_active = true`,
			reg, model, vType, maxLoad, odometer, cost, status, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting vehicle %s: %v\n", reg, err)
			os.Exit(1)
		}
	}

	// 4. Drivers (40 drivers)
	firstNames := []string{"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda", "William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas"}

	driverLicenses := make([]string, 40)
	for i := 1; i <= 40; i++ {
		lic := fmt.Sprintf("DL-%04d", 1000+i)
		driverLicenses[i-1] = lic

		name := fmt.Sprintf("%s %s", firstNames[rng.Intn(len(firstNames))], lastNames[rng.Intn(len(lastNames))])
		cat := "Heavy"
		phone := fmt.Sprintf("+1-555-%04d", 1000+i)
		safety := float64(820+rng.Intn(180)) / 10.0
		expiryDate := now.AddDate(2, rng.Intn(12), rng.Intn(28)).Format(domain.DateFormat)

		_, err = tx.Exec(ctx,
			`INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'Available', true, $7, $7)
			 ON CONFLICT (license_number) DO UPDATE SET
			   name = EXCLUDED.name, license_category = EXCLUDED.license_category,
			   license_expiry_date = EXCLUDED.license_expiry_date, contact_number = EXCLUDED.contact_number,
			   safety_score = EXCLUDED.safety_score, status = EXCLUDED.status, is_active = true`,
			lic, name, cat, expiryDate, phone, safety, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting driver %s: %v\n", lic, err)
			os.Exit(1)
		}
	}

	// 5. Seed Users
	systemUsers := []struct {
		email   string
		role    string
		license *string
	}{
		{"admin@transitops.com", "Admin", nil},
		{"fleet@transitops.com", "Fleet Manager", nil},
		{"safety@transitops.com", "Safety Officer", nil},
		{"finance@transitops.com", "Financial Analyst", nil},
		{"driver1@transitops.com", "Driver", &driverLicenses[0]},
		{"driver2@transitops.com", "Driver", &driverLicenses[1]},
		{"driver3@transitops.com", "Driver", &driverLicenses[2]},
	}

	for _, u := range systemUsers {
		userId := uuid.NewString()
		_, err = tx.Exec(ctx,
			`INSERT INTO users (id, email, password_hash, role, is_active, driver_license, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, true, $5, $6, $6)
			 ON CONFLICT (email) DO UPDATE SET
			   password_hash = EXCLUDED.password_hash,
			   role = EXCLUDED.role,
			   driver_license = EXCLUDED.driver_license,
			   is_active = true,
			   updated_at = EXCLUDED.updated_at`,
			userId, u.email, passHash, u.role, u.license, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting user %s: %v\n", u.email, err)
			os.Exit(1)
		}
	}

	// 6. Reset dependent tables
	if _, err := tx.Exec(ctx, `DELETE FROM expenses`); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing expenses: %v\n", err)
		os.Exit(1)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM maintenance_logs`); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing maintenance_logs: %v\n", err)
		os.Exit(1)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trips`); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing trips: %v\n", err)
		os.Exit(1)
	}

	// 7. Seed Active Dispatched Trips on Continental Routes
	// Put 20 trucks on Dispatched trips across the 10 continental routes
	for i := 0; i < 20; i++ {
		sr := simRoutes[i%len(simRoutes)]
		vReg := vehicleRegs[i]
		dLic := driverLicenses[i]

		cargo := float64(8000 + rng.Intn(14000))
		dist := float64(600 + rng.Intn(1200))
		rev := dist * float64(3+rng.Intn(3))

		var tripId int64
		err = tx.QueryRow(ctx,
			`INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 'Dispatched', $8, $8)
			 RETURNING id`,
			sr.origin, sr.destination, vReg, dLic, cargo, dist, rev, now).Scan(&tripId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting dispatched trip: %v\n", err)
			os.Exit(1)
		}

		_, _ = tx.Exec(ctx, `UPDATE vehicles SET status = 'On Trip' WHERE registration_number = $1`, vReg)
		_, _ = tx.Exec(ctx, `UPDATE drivers SET status = 'On Trip' WHERE license_number = $1`, dLic)
	}

	// 8. Historical Completed & Draft Trips
	for i := 20; i < 60; i++ {
		sr := simRoutes[rng.Intn(len(simRoutes))]
		vReg := vehicleRegs[rng.Intn(len(vehicleRegs))]
		dLic := driverLicenses[rng.Intn(len(driverLicenses))]

		cargo := float64(5000 + rng.Intn(16000))
		dist := float64(400 + rng.Intn(1400))
		rev := dist * float64(3+rng.Intn(2))
		status := "Completed"
		if i%5 == 0 {
			status = "Draft"
		}

		var tripId int64
		err = tx.QueryRow(ctx,
			`INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			 RETURNING id`,
			sr.origin, sr.destination, vReg, dLic, cargo, dist, rev, status, now).Scan(&tripId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting trip %d: %v\n", i, err)
			os.Exit(1)
		}

		if status == "Completed" {
			liters := dist / 1.3
			cost := liters * 1.55
			_, _ = tx.Exec(ctx,
				`INSERT INTO expenses (trip_id, type, liters, cost, date, created_at, updated_at)
				 VALUES ($1, 'Fuel', $2, $3, $4, $5, $5)`,
				tripId, liters, cost, now.Format(domain.DateFormat), now)
		}
	}

	// 9. Maintenance Logs
	for i := 1; i <= 15; i++ {
		vReg := vehicleRegs[rng.Intn(len(vehicleRegs))]
		cost := float64(450 + rng.Intn(3500))
		mStatus := "Closed"
		daysAgo := 10 + rng.Intn(60)
		startDate := now.AddDate(0, 0, -daysAgo).Format(domain.DateFormat)
		ed := now.AddDate(0, 0, -daysAgo+2+rng.Intn(5)).Format(domain.DateFormat)

		_, err = tx.Exec(ctx,
			`INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, end_date, status, created_at, updated_at)
			 VALUES ($1, 'Scheduled Overhaul & Brake Service', $2, $3, $4, $5, $6, $6)`,
			vReg, cost, startDate, ed, mStatus, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting maintenance log: %v\n", err)
			os.Exit(1)
		}
	}

	for _, s := range []string{"trips", "maintenance_logs", "expenses"} {
		if _, e := tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence($1, 'id'), COALESCE((SELECT MAX(id) FROM "+s+"), 1), true)", s); e != nil {
			fmt.Fprintf(os.Stderr, "Error resetting sequence: %v\n", e)
			os.Exit(1)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error committing seed transaction: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Seed v2 generated successfully:")
	fmt.Printf(" - Vehicles: %d (50 continental trucks)\n", len(vehicleRegs))
	fmt.Printf(" - Drivers: %d\n", len(driverLicenses))
	fmt.Printf(" - Hubs: %d\n", len(hubs))
	fmt.Printf(" - Continental Routes: %d\n", len(simRoutes))
	fmt.Println(" - Password for all seeded users:", seedPassEnv)
}
