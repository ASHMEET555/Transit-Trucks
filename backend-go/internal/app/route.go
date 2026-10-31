package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const defaultOSRMURL = "http://localhost:5000"

// osrmBaseURL returns the OSRM endpoint base, defaulting to local dev.
func osrmBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("OSRM_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultOSRMURL
}

// ---------- OSRM client ----------

// RouteResult is one computed route between two hubs.
type RouteResult struct {
	Polyline6 string
	DistanceM float64
	DurationS float64
	Provider  string
}

// FetchRoute queries OSRM (lon,lat order) for a driving route.
func FetchRoute(ctx context.Context, baseURL string, fromLat, fromLon, toLat, toLon float64) (RouteResult, error) {
	url := fmt.Sprintf("%s/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=polyline6",
		strings.TrimRight(baseURL, "/"), fromLon, fromLat, toLon, toLat)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RouteResult{}, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return RouteResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RouteResult{}, fmt.Errorf("osrm status %d", resp.StatusCode)
	}
	var body struct {
		Code   string `json:"code"`
		Routes []struct {
			Geometry string  `json:"geometry"`
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return RouteResult{}, err
	}
	if body.Code != "Ok" || len(body.Routes) == 0 {
		return RouteResult{}, fmt.Errorf("osrm no route: %s", body.Code)
	}
	r := body.Routes[0]
	if r.Geometry == "" {
		return RouteResult{}, fmt.Errorf("osrm empty geometry")
	}
	if _, err := DecodePolyline6(r.Geometry); err != nil {
		return RouteResult{}, fmt.Errorf("osrm bad geometry: %w", err)
	}
	return RouteResult{Polyline6: r.Geometry, DistanceM: r.Distance, DurationS: r.Duration, Provider: "osrm"}, nil
}

// ensureRouteBestEffort computes and caches the route for a hub pair in its
// own short transaction, after the dispatch commit. OSRM latency therefore
// never delays dispatch events; failures are logged and ignored.
func (a *App) ensureRouteBestEffort(origin, destination string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if _, err := EnsureRoute(ctx, tx, origin, destination); err != nil {
		if a.log != nil {
			a.log.Warn("route ensure failed", "error", err)
		}
		return
	}
	if err := tx.Commit(ctx); err != nil && a.log != nil {
		a.log.Warn("route ensure commit failed", "error", err)
	}
}

// EnsureRoute returns the cached route for an (origin, destination) pair, or
// computes it via OSRM and caches it. When OSRM is unreachable (or hubs lack
// coordinates), it caches a straight-line 2-point route with
// provider='approximate' so the UI can render it dashed. The second dispatch
// of the same pair is always a cache hit with no OSRM call.
//
// Returns cached=true when the row already existed.
func EnsureRoute(ctx context.Context, tx pgx.Tx, origin, destination string) (bool, error) {
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM route_geometries WHERE origin_id = $1 AND destination_id = $2`,
		origin, destination).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}

	result, ok := computeRouteForPair(ctx, tx, origin, destination)
	if !ok {
		result = RouteResult{Polyline6: "", DistanceM: 0, DurationS: 0, Provider: "approximate"}
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO route_geometries (origin_id, destination_id, polyline6, distance_m, duration_s, provider)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (origin_id, destination_id) DO NOTHING`,
		origin, destination, result.Polyline6, result.DistanceM, result.DurationS, result.Provider)
	if err != nil {
		return false, err
	}
	return false, nil
}

func computeRouteForPair(ctx context.Context, tx pgx.Tx, origin, destination string) (RouteResult, bool) {
	var oLat, oLon, dLat, dLon *float64
	if err := tx.QueryRow(ctx,
		`SELECT latitude, longitude FROM destinations WHERE name = $1`, origin).Scan(&oLat, &oLon); err != nil {
		return RouteResult{}, false
	}
	if err := tx.QueryRow(ctx,
		`SELECT latitude, longitude FROM destinations WHERE name = $1`, destination).Scan(&dLat, &dLon); err != nil {
		return RouteResult{}, false
	}
	if oLat == nil || oLon == nil || dLat == nil || dLon == nil {
		return RouteResult{}, false
	}
	res, err := FetchRoute(ctx, osrmBaseURL(), *oLat, *oLon, *dLat, *dLon)
	if err != nil {
		approx := EncodePolyline6([][2]float64{{*oLat, *oLon}, {*dLat, *dLon}})
		dist := haversineM(*oLat, *oLon, *dLat, *dLon)
		return RouteResult{Polyline6: approx, DistanceM: dist, DurationS: 0, Provider: "approximate"}, true
	}
	return res, true
}
