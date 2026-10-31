package main

import (
	"hash/fnv"
	"math"
	"os"
	"strconv"
	"strings"
)

// VehicleState is the in-memory kinematic state of one simulated truck.
type VehicleState struct {
	VehicleReg string
	TripID     int64
	DistM      float64 // distance travelled along the route
	SpeedMps   float64
	Arrived    bool
	DwellS     float64 // remaining dwell at destination (sim seconds)
}

// SimRoute is a decoded route with cumulative distances and an average speed.
type SimRoute struct {
	Pts      [][2]float64
	Cum      []float64
	TotalM   float64
	AvgMps   float64
	Provider string
}

// haversineM is duplicated here so the simulator stays dependency-light.
func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * R * math.Asin(math.Sqrt(a))
}

// bearingDeg returns the initial bearing from a to b in degrees [0,360).
func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dLon) * math.Cos(lat2*math.Pi/180)
	x := math.Cos(lat1*math.Pi/180)*math.Sin(lat2*math.Pi/180) -
		math.Sin(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Cos(dLon)
	b := math.Atan2(y, x) * 180 / math.Pi
	for b < 0 {
		b += 360
	}
	for b >= 360 {
		b -= 360
	}
	return b
}

func turnAngle(a, b float64) float64 {
	d := math.Abs(a - b)
	for d >= 360 {
		d -= 360
	}
	if d > 180 {
		d = 360 - d
	}
	return d
}

// NewSimRoute builds cumulative distances; average speed comes from the
// cached route duration, falling back to 80 km/h when unknown.
func NewSimRoute(pts [][2]float64, distanceM, durationS float64, provider string) SimRoute {
	r := SimRoute{Pts: pts, Provider: provider, AvgMps: 80.0 / 3.6}
	if len(pts) == 0 {
		return r
	}
	r.Cum = make([]float64, len(pts))
	for i := 1; i < len(pts); i++ {
		r.Cum[i] = r.Cum[i-1] + haversineM(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1])
	}
	r.TotalM = r.Cum[len(pts)-1]
	if durationS > 0 && distanceM > 0 {
		r.AvgMps = distanceM / durationS
	}
	return r
}

// SpeedLimitAt returns the route average speed, reduced to 60% when the
// sharpest turn within the next 80m exceeds 25 degrees (curve slowdown).
func SpeedLimitAt(r *SimRoute, distM float64) float64 {
	limit := r.AvgMps
	if len(r.Pts) < 3 {
		return limit
	}
	// Segment index containing distM.
	seg := 0
	for seg < len(r.Cum)-2 && r.Cum[seg+1] < distM {
		seg++
	}
	base := bearingDeg(r.Pts[seg][0], r.Pts[seg][1], r.Pts[seg+1][0], r.Pts[seg+1][1])
	ahead := distM + 80
	for i := seg + 1; i+1 < len(r.Pts) && r.Cum[i] <= ahead; i++ {
		b := bearingDeg(r.Pts[i][0], r.Pts[i][1], r.Pts[i+1][0], r.Pts[i+1][1])
		if turnAngle(base, b) > 25 {
			return limit * 0.6
		}
	}
	return limit
}

const (
	maxAccelMps2 = 1.5 // acceleration cap
	maxBrakeMps2 = 2.5 // braking cap
	dwellSimS    = 15 * 60
)

// TickVehicle advances one vehicle by dtSim simulated seconds: accelerate
// toward the speed limit (cap 1.5 m/s^2), brake toward it (cap 2.5 m/s^2),
// stop exactly at the destination, then dwell 15 sim-minutes. It never
// completes trips: arrived vehicles hold position indefinitely.
func TickVehicle(v *VehicleState, r *SimRoute, dtSim float64) {
	if v.Arrived {
		v.SpeedMps = 0
		v.DwellS -= dtSim
		return
	}
	limit := SpeedLimitAt(r, v.DistM)
	if v.SpeedMps < limit {
		v.SpeedMps = math.Min(limit, v.SpeedMps+maxAccelMps2*dtSim)
	} else {
		v.SpeedMps = math.Max(limit, v.SpeedMps-maxBrakeMps2*dtSim)
	}
	v.DistM += v.SpeedMps * dtSim
	if v.DistM >= r.TotalM {
		v.DistM = r.TotalM
		v.SpeedMps = 0
		v.Arrived = true
		v.DwellS = dwellSimS
	}
}

// PositionAt interpolates lat/lon and heading at the vehicle's distance.
func PositionAt(v *VehicleState, r *SimRoute) (lat, lon, heading float64) {
	if len(r.Pts) == 0 {
		return 0, 0, 0
	}
	if len(r.Pts) == 1 || v.DistM <= 0 {
		if len(r.Pts) > 1 {
			return r.Pts[0][0], r.Pts[0][1], bearingDeg(r.Pts[0][0], r.Pts[0][1], r.Pts[1][0], r.Pts[1][1])
		}
		return r.Pts[0][0], r.Pts[0][1], 0
	}
	if v.DistM >= r.TotalM {
		n := len(r.Pts)
		return r.Pts[n-1][0], r.Pts[n-1][1], bearingDeg(r.Pts[n-2][0], r.Pts[n-2][1], r.Pts[n-1][0], r.Pts[n-1][1])
	}
	i := 0
	for i < len(r.Cum)-2 && r.Cum[i+1] < v.DistM {
		i++
	}
	segLen := r.Cum[i+1] - r.Cum[i]
	t := 0.0
	if segLen > 0 {
		t = (v.DistM - r.Cum[i]) / segLen
	}
	lat = r.Pts[i][0] + t*(r.Pts[i+1][0]-r.Pts[i][0])
	lon = r.Pts[i][1] + t*(r.Pts[i+1][1]-r.Pts[i][1])
	heading = bearingDeg(r.Pts[i][0], r.Pts[i][1], r.Pts[i+1][0], r.Pts[i+1][1])
	return lat, lon, heading
}

// SeedOffset returns a deterministic per-vehicle start offset in meters
// (0..500) derived only from SIM_SEED and the vehicle registration.
func SeedOffset(seed int64, vehicleReg string) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strconv.FormatInt(seed, 10) + "|" + vehicleReg))
	return float64(h.Sum32()%5000) / 10.0
}

// SimConfig is parsed from the environment; everything is optional.
type SimConfig struct {
	BackendURL string
	DSN        string
	AdminEmail string
	AdminPass  string
	TimeScale  float64
	Seed       int64
}

// LoadSimConfig reads simulator env with safe defaults.
func LoadSimConfig() SimConfig {
	c := SimConfig{
		BackendURL: strings.TrimSpace(os.Getenv("BACKEND_URL")),
		AdminEmail: strings.TrimSpace(os.Getenv("SIM_ADMIN_EMAIL")),
		AdminPass:  os.Getenv("SIM_ADMIN_PASSWORD"),
		TimeScale:  20,
	}
	if c.BackendURL == "" {
		c.BackendURL = "http://localhost:8000"
	}
	c.BackendURL = strings.TrimRight(c.BackendURL, "/")
	if c.AdminEmail == "" {
		c.AdminEmail = "admin@transitops.com"
	}
	if v := strings.TrimSpace(os.Getenv("SIM_TIME_SCALE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			c.TimeScale = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("SIM_SEED")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.Seed = n
		}
	}
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		c.DSN = dsn
	} else {
		c.DSN = os.Getenv("POSTGRES_DSN")
	}
	return c
}
