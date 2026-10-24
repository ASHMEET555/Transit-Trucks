package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		panic("POSTGRES_DSN is not set")
	}
	p := os.Getenv("JSON_PATH")
	if p == "" {
		p = "dummy_data.json"
	}
	f, e := os.Open(p)
	if e != nil {
		fmt.Printf("Error: Could not locate dummy JSON framework file at %s\n", p)
		return
	}
	defer f.Close()
	var d map[string][]map[string]any
	if json.NewDecoder(f).Decode(&d) != nil {
		panic("invalid seed json")
	}
	db, e := pgxpool.New(context.Background(), dsn)
	if e != nil {
		panic(e)
	}
	defer db.Close()
	ctx := context.Background()
	tx, e := db.Begin(ctx)
	if e != nil {
		panic(e)
	}
	defer tx.Rollback(ctx)
	run := func(q string, fields []string, items []map[string]any) {
		for _, x := range items {
			a := make([]any, len(fields))
			for i, k := range fields {
				a[i] = x[k]
			}
			if _, e := tx.Exec(ctx, q, a...); e != nil {
				panic(e)
			}
		}
	}
	run("INSERT INTO users(email,password_hash,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", []string{"email", "password_hash", "role"}, d["users"])
	run("INSERT INTO destinations(name,region) VALUES($1,$2) ON CONFLICT DO NOTHING", []string{"name", "region"}, d["destinations"])
	run("INSERT INTO vehicles(registration_number,model,type,max_load_capacity,odometer,acquisition_cost,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"registration_number", "model", "type", "max_load_capacity", "odometer", "acquisition_cost", "status"}, d["vehicles"])
	run("INSERT INTO drivers(license_number,name,license_category,license_expiry_date,contact_number,safety_score,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"license_number", "name", "license_category", "license_expiry_date", "contact_number", "safety_score", "status"}, d["drivers"])
	run("INSERT INTO trips(id,source_name,destination_name,vehicle_reg,driver_license,cargo_weight,planned_distance,revenue,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING", []string{"id", "source_name", "destination_name", "vehicle_reg", "driver_license", "cargo_weight", "planned_distance", "revenue", "status"}, d["trips"])
	run("INSERT INTO maintenance_logs(id,vehicle_reg,description,cost,start_date,end_date,status) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING", []string{"id", "vehicle_reg", "description", "cost", "start_date", "end_date", "status"}, d["maintenance_logs"])
	run("INSERT INTO expenses(id,trip_id,type,liters,cost,date) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", []string{"id", "trip_id", "type", "liters", "cost", "date"}, d["fuel_expenses"])
	for _, s := range []string{"trips", "maintenance_logs", "expenses"} {
		if _, e := tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence($1, 'id'), COALESCE((SELECT MAX(id) FROM "+s+"), 1), true)", s); e != nil {
			panic(e)
		}
	}
	if e := tx.Commit(ctx); e != nil {
		panic(e)
	}
	fmt.Println("Database seeding pipelines terminated cleanly.")
}
