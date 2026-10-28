package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	// 1. Hubs / Destinations (>= 8 hubs across regions)
	type destRecord struct {
		name   string
		region string
	}
	hubs := []destRecord{
		{"New York Central Hub", "Northeast"},
		{"Chicago Logistics Hub", "Midwest"},
		{"Los Angeles Freight Hub", "West"},
		{"Dallas Gateway Hub", "South"},
		{"Atlanta Distribution Hub", "Southeast"},
		{"Seattle Terminal", "Northwest"},
		{"Denver Intermodal Hub", "Mountain"},
		{"Miami Port Hub", "Southeast"},
	}

	for _, h := range hubs {
		_, err = tx.Exec(ctx,
			`INSERT INTO destinations (name, region, created_at, updated_at)
			 VALUES ($1, $2, $3, $3)
			 ON CONFLICT (name) DO UPDATE SET region = EXCLUDED.region`,
			h.name, h.region, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting destination %s: %v\n", h.name, err)
			os.Exit(1)
		}
	}

	// 2. Vehicles (>= 160 vehicles)
	vehicleTypes := []string{"Van", "Box Truck", "Flatbed", "Semi", "Light Truck", "Medium Truck", "Heavy Truck", "Refrigerated"}
	vehicleModels := []string{"Ford Transit", "Freightliner Cascadia", "Volvo VNL 860", "Isuzu NPR-HD", "Peterbilt 579", "Kenworth T680", "Mercedes Sprinter", "Mack Anthem"}

	vehicleRegs := make([]string, 160)
	for i := 1; i <= 160; i++ {
		reg := fmt.Sprintf("VH-%04d", 1000+i)
		vehicleRegs[i-1] = reg

		vType := vehicleTypes[rng.Intn(len(vehicleTypes))]
		model := vehicleModels[rng.Intn(len(vehicleModels))]
		maxLoad := float64(1000 + rng.Intn(24)*1000)
		odometer := float64(5000 + rng.Intn(180000))
		cost := float64(25000 + rng.Intn(95000))

		status := "Available"
		roll := rng.Float64()
		if roll < 0.10 {
			status = "In Shop"
		} else if roll < 0.20 {
			status = "Retired"
		}

		isActive := true
		if status == "Retired" && rng.Float64() < 0.5 {
			isActive = false
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			 ON CONFLICT (registration_number) DO UPDATE SET
			   model = EXCLUDED.model, type = EXCLUDED.type, max_load_capacity = EXCLUDED.max_load_capacity,
			   odometer = EXCLUDED.odometer, acquisition_cost = EXCLUDED.acquisition_cost, status = EXCLUDED.status,
			   is_active = EXCLUDED.is_active`,
			reg, model, vType, maxLoad, odometer, cost, status, isActive, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting vehicle %s: %v\n", reg, err)
			os.Exit(1)
		}
	}

	// 3. Drivers (>= 160 drivers)
	firstNames := []string{"James", "Mary", "John", "Patricia", "Robert", "Jennifer", "Michael", "Linda", "William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas"}

	driverLicenses := make([]string, 160)
	for i := 1; i <= 160; i++ {
		lic := fmt.Sprintf("DL-%04d", 1000+i)
		driverLicenses[i-1] = lic

		name := fmt.Sprintf("%s %s", firstNames[rng.Intn(len(firstNames))], lastNames[rng.Intn(len(lastNames))])
		cat := "Heavy"
		if i%3 == 0 {
			cat = "Medium"
		} else if i%3 == 1 {
			cat = "Van"
		}

		phone := fmt.Sprintf("+1-555-%04d", 1000+i)
		safety := float64(700+rng.Intn(300)) / 10.0

		// Status and expiry variations:
		status := "Available"
		roll := rng.Float64()
		var expiryDate string
		if roll < 0.10 {
			status = "Suspended"
			expiryDate = now.AddDate(-1, 0, rng.Intn(300)).Format(domain.DateFormat) // expired
		} else if roll < 0.20 {
			status = "Off Duty"
			expiryDate = now.AddDate(0, 0, rng.Intn(25)).Format(domain.DateFormat) // expiring soon (<30 days)
		} else {
			expiryDate = now.AddDate(1+rng.Intn(4), rng.Intn(12), rng.Intn(28)).Format(domain.DateFormat) // future
		}

		isActive := true
		if status == "Suspended" && rng.Float64() < 0.4 {
			isActive = false
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			 ON CONFLICT (license_number) DO UPDATE SET
			   name = EXCLUDED.name, license_category = EXCLUDED.license_category,
			   license_expiry_date = EXCLUDED.license_expiry_date, contact_number = EXCLUDED.contact_number,
			   safety_score = EXCLUDED.safety_score, status = EXCLUDED.status, is_active = EXCLUDED.is_active`,
			lic, name, cat, expiryDate, phone, safety, status, isActive, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting driver %s: %v\n", lic, err)
			os.Exit(1)
		}
	}

	// 4. Seed Standard Users
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

	// 5. Historical Trips (>= 200 historical trips)
	// Clear existing dependent records so seeding is idempotent
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

	dispatchedVehicles := make(map[string]bool)
	dispatchedDrivers := make(map[string]bool)

	for i := 1; i <= 220; i++ {
		srcIdx := rng.Intn(len(hubs))
		destIdx := (srcIdx + 1 + rng.Intn(len(hubs)-1)) % len(hubs) // guarantee source <> destination

		vIdx := rng.Intn(len(vehicleRegs))
		dIdx := rng.Intn(len(driverLicenses))

		vReg := vehicleRegs[vIdx]
		dLic := driverLicenses[dIdx]

		cargo := float64(500 + rng.Intn(15000))
		dist := float64(80 + rng.Intn(1200))
		rev := dist * float64(2+rng.Intn(4))

		status := "Completed"
		roll := rng.Float64()
		if roll < 0.15 {
			status = "Cancelled"
		} else if roll < 0.25 {
			status = "Draft"
		} else if roll < 0.30 {
			// At most ONE Dispatched trip per vehicle and driver
			if !dispatchedVehicles[vReg] && !dispatchedDrivers[dLic] {
				status = "Dispatched"
				dispatchedVehicles[vReg] = true
				dispatchedDrivers[dLic] = true
			}
		}

		var tripId int64
		err = tx.QueryRow(ctx,
			`INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			 RETURNING id`,
			hubs[srcIdx].name, hubs[destIdx].name, vReg, dLic, cargo, dist, rev, status, now).Scan(&tripId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting trip %d: %v\n", i, err)
			os.Exit(1)
		}

		// Insert Fuel/Toll expense for completed trips
		if status == "Completed" {
			liters := float64(30 + rng.Intn(200))
			fuelCost := liters * 1.55
			_, _ = tx.Exec(ctx,
				`INSERT INTO expenses (trip_id, type, liters, cost, date, created_at, updated_at)
				 VALUES ($1, 'Fuel', $2, $3, $4, $5, $5)`,
				tripId, liters, fuelCost, now.Format(domain.DateFormat), now)
		} else if status == "Dispatched" {
			_, _ = tx.Exec(ctx, `UPDATE vehicles SET status = 'On Trip' WHERE registration_number = $1`, vReg)
			_, _ = tx.Exec(ctx, `UPDATE drivers SET status = 'On Trip' WHERE license_number = $1`, dLic)
		}
	}

	// 6. Maintenance Logs
	openMaintVehicles := make(map[string]bool)
	for i := 1; i <= 35; i++ {
		vReg := vehicleRegs[rng.Intn(len(vehicleRegs))]
		cost := float64(250 + rng.Intn(4500))
		mStatus := "Closed"
		startDate := now.AddDate(0, 0, -rng.Intn(120)).Format(domain.DateFormat)
		var endDate *string

		if rng.Float64() < 0.25 && !openMaintVehicles[vReg] {
			mStatus = "Open"
			openMaintVehicles[vReg] = true
			endDate = nil
		} else {
			ed := now.AddDate(0, 0, -rng.Intn(10)).Format(domain.DateFormat)
			endDate = &ed
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, end_date, status, created_at, updated_at)
			 VALUES ($1, 'Scheduled Inspection and Overhaul', $2, $3, $4, $5, $6, $6)`,
			vReg, cost, startDate, endDate, mStatus, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting maintenance log %d: %v\n", i, err)
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
	fmt.Printf(" - Vehicles: %d\n", len(vehicleRegs))
	fmt.Printf(" - Drivers: %d\n", len(driverLicenses))
	fmt.Printf(" - Hubs: %d\n", len(hubs))
	fmt.Printf(" - Random Seed: %d\n", seedVal)
	fmt.Println(" - Password for all seeded users:", seedPassEnv)
}
