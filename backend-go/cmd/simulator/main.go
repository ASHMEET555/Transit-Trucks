// Command simulator drives virtual trucks along cached routes and posts
// positions to POST /api/v1/telemetry/positions as a service login.
//
// Approach: the simulator reads Dispatched trips plus the route_geometries
// cache directly from Postgres (poll), but writes positions only through the
// same public HTTP endpoint every other client uses. It logs in with the
// seed admin credentials from SIM_ADMIN_EMAIL / SIM_ADMIN_PASSWORD and sends
// source=simulator reports, which the server accepts only when SIM_ENABLED=1.
// It never completes trips: arrived trucks dwell at the destination forever.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
)

type polledTruck struct {
	tripID                int64
	vehicleReg            string
	origin, destination   string
	polyline              string
	distanceM, durationS   float64
	provider              string
}

func pollDispatched(ctx context.Context, pool *pgxpool.Pool) ([]polledTruck, error) {
	rows, err := pool.Query(ctx, `
		SELECT t.id, t.vehicle_reg, t.source_name, t.destination_name,
			COALESCE(r.polyline6, ''), COALESCE(r.distance_m, 0), COALESCE(r.duration_s, 0), COALESCE(r.provider, '')
		FROM trips t
		LEFT JOIN route_geometries r ON r.origin_id = t.source_name AND r.destination_id = t.destination_name
		WHERE t.status = 'Dispatched'
		ORDER BY t.vehicle_reg ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []polledTruck
	for rows.Next() {
		var tr polledTruck
		if err := rows.Scan(&tr.tripID, &tr.vehicleReg, &tr.origin, &tr.destination,
			&tr.polyline, &tr.distanceM, &tr.durationS, &tr.provider); err != nil {
			continue
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func login(ctx context.Context, backendURL, email, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, backendURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &loginError{status: resp.StatusCode}
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Token, nil
}

type loginError struct{ status int }

func (e *loginError) Error() string { return "login failed" }

func postPosition(ctx context.Context, backendURL, token string, v *VehicleState, r *SimRoute) int {
	lat, lon, heading := PositionAt(v, r)
	body, _ := json.Marshal(map[string]any{
		"vehicle_reg": v.VehicleReg,
		"latitude":    lat,
		"longitude":   lon,
		"heading_deg": heading,
		"speed_kmh":   v.SpeedMps * 3.6,
		"recorded_at": time.Now().UTC().Format(time.RFC3339),
		"source":      "simulator",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, backendURL+"/api/v1/telemetry/positions", bytes.NewReader(body))
	if err != nil {
		return -1
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := LoadSimConfig()
	if cfg.DSN == "" {
		logger.Error("DATABASE_URL or POSTGRES_DSN is required")
		os.Exit(1)
	}
	if cfg.AdminPass == "" {
		logger.Error("SIM_ADMIN_PASSWORD is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	token, err := login(ctx, cfg.BackendURL, cfg.AdminEmail, cfg.AdminPass)
	if err != nil {
		logger.Error("simulator login failed", "error", err)
		os.Exit(1)
	}

	states := map[string]*VehicleState{}
	routes := map[string]*SimRoute{}

	refresh := func() {
		trucks, err := pollDispatched(ctx, pool)
		if err != nil {
			logger.Warn("poll failed", "error", err)
			return
		}
		seen := map[string]bool{}
		for _, tr := range trucks {
			seen[tr.vehicleReg] = true
			if tr.polyline == "" {
				logger.Warn("no route geometry yet, skipping", "vehicle", tr.vehicleReg)
				continue
			}
			if _, ok := routes[tr.vehicleReg]; !ok {
				pts, err := app.DecodePolyline6(tr.polyline)
				if err != nil || len(pts) < 2 {
					logger.Warn("bad route geometry, skipping", "vehicle", tr.vehicleReg)
					continue
				}
				r := NewSimRoute(pts, tr.distanceM, tr.durationS, tr.provider)
				routes[tr.vehicleReg] = &r
				off := SeedOffset(cfg.Seed, tr.vehicleReg)
				if off >= r.TotalM {
					off = 0
				}
				states[tr.vehicleReg] = &VehicleState{VehicleReg: tr.vehicleReg, TripID: tr.tripID, DistM: off}
				logger.Info("tracking vehicle", "vehicle", tr.vehicleReg, "offset_m", off)
			}
		}
		for reg := range states {
			if !seen[reg] {
				delete(states, reg)
				delete(routes, reg)
			}
		}
	}
	refresh()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	poll := time.NewTicker(10 * time.Second)
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			refresh()
		case <-tick.C:
			dtSim := cfg.TimeScale // physics tick 1s x SIM_TIME_SCALE
			for reg, v := range states {
				r := routes[reg]
				TickVehicle(v, r, dtSim)
				if code := postPosition(ctx, cfg.BackendURL, token, v, r); code == http.StatusUnauthorized {
					if tok, err := login(ctx, cfg.BackendURL, cfg.AdminEmail, cfg.AdminPass); err == nil {
						token = tok
					}
				}
			}
		}
	}
}
