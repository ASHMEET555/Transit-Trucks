package main

import (
	"testing"
)

func straightRoute() SimRoute {
	pts := [][2]float64{{0, 0}, {0, 0.0898315}} // ~10km
	return NewSimRoute(pts, 10000, 400, "osrm") // avg 25 m/s
}

func TestPhysicsAccelCap(t *testing.T) {
	r := straightRoute()
	v := &VehicleState{SpeedMps: 0}
	TickVehicle(v, &r, 1.0)
	if v.SpeedMps != 1.5 {
		t.Fatalf("expected accel-capped speed 1.5, got %f", v.SpeedMps)
	}
	for i := 0; i < 30; i++ {
		TickVehicle(v, &r, 1.0)
	}
	if v.SpeedMps > 25.000001 {
		t.Fatalf("speed must not exceed route average 25, got %f", v.SpeedMps)
	}
}

func TestPhysicsBrakeCap(t *testing.T) {
	r := straightRoute()
	v := &VehicleState{SpeedMps: 30} // above limit
	TickVehicle(v, &r, 1.0)
	if v.SpeedMps != 27.5 {
		t.Fatalf("expected brake-capped speed 27.5, got %f", v.SpeedMps)
	}
}

func TestCurveSlowdown(t *testing.T) {
	// L-shape with ~55m legs: 90-degree turn inside the 80m lookahead.
	pts := [][2]float64{{0, 0}, {0, 0.0005}, {0.0005, 0.0005}}
	r := NewSimRoute(pts, 111, 4.44, "osrm") // avg ~25 m/s
	if got, want := SpeedLimitAt(&r, 0), r.AvgMps*0.6; (got-want) > 1e-9 || (want-got) > 1e-9 {
		t.Fatalf("expected 0.6x slowdown in curve, got %f want %f", got, want)
	}
	// Past the turn the limit recovers.
	if got := SpeedLimitAt(&r, 60); (got-r.AvgMps) > 1e-9 || (r.AvgMps-got) > 1e-9 {
		t.Fatalf("expected full speed past curve, got %f", got)
	}
}

func TestStopAtDestinationThenDwell(t *testing.T) {
	pts := [][2]float64{{0, 0}, {0, 0.0009}} // ~100m
	r := NewSimRoute(pts, 100, 10, "osrm")   // avg 10 m/s
	v := &VehicleState{SpeedMps: 10}
	for i := 0; i < 200 && !v.Arrived; i++ {
		TickVehicle(v, &r, 1.0)
	}
	if !v.Arrived {
		t.Fatal("expected arrival")
	}
	if v.DistM != r.TotalM {
		t.Fatalf("must stop exactly at destination: %f vs %f", v.DistM, r.TotalM)
	}
	if v.SpeedMps != 0 {
		t.Fatalf("speed must be 0 at destination, got %f", v.SpeedMps)
	}
	if v.DwellS != 15*60 {
		t.Fatalf("expected 15 sim-min dwell, got %f", v.DwellS)
	}
}

func TestNeverCompletesTrips(t *testing.T) {
	pts := [][2]float64{{0, 0}, {0, 0.0009}}
	r := NewSimRoute(pts, 100, 10, "osrm")
	v := &VehicleState{SpeedMps: 10}
	// Tick far beyond arrival + dwell: vehicle must hold, never reset/complete.
	for i := 0; i < 2000; i++ {
		TickVehicle(v, &r, 1.0)
	}
	if !v.Arrived || v.DistM != r.TotalM || v.SpeedMps != 0 {
		t.Fatalf("arrived vehicle must hold position forever: %+v", v)
	}
}

func TestSeedOffsetDeterministic(t *testing.T) {
	a := SeedOffset(7, "VH-0001")
	b := SeedOffset(7, "VH-0001")
	if a != b {
		t.Fatal("same SIM_SEED must give same offsets")
	}
	if a < 0 || a >= 500 {
		t.Fatalf("offset out of range: %f", a)
	}
	seen := map[float64]bool{a: true}
	for _, reg := range []string{"VH-0002", "VH-0003", "VH-0004", "VH-0005"} {
		seen[SeedOffset(7, reg)] = true
	}
	if len(seen) < 3 {
		t.Fatal("offsets should vary across vehicles")
	}
}

func TestLoadSimConfigDefaults(t *testing.T) {
	t.Setenv("BACKEND_URL", "")
	t.Setenv("SIM_ADMIN_EMAIL", "")
	t.Setenv("SIM_TIME_SCALE", "")
	t.Setenv("SIM_SEED", "")
	c := LoadSimConfig()
	if c.TimeScale != 20 {
		t.Fatalf("default SIM_TIME_SCALE must be 20, got %f", c.TimeScale)
	}
	if c.BackendURL != "http://localhost:8000" {
		t.Fatalf("unexpected backend default %q", c.BackendURL)
	}
	if c.Seed != 0 {
		t.Fatalf("default seed must be 0, got %d", c.Seed)
	}

	t.Setenv("SIM_TIME_SCALE", "bogus")
	t.Setenv("SIM_SEED", "42")
	c = LoadSimConfig()
	if c.TimeScale != 20 {
		t.Fatalf("invalid SIM_TIME_SCALE must fall back to 20, got %f", c.TimeScale)
	}
	if c.Seed != 42 {
		t.Fatalf("expected seed 42, got %d", c.Seed)
	}
}

func TestTotalTimeWithin5Pct(t *testing.T) {
	// Straight 10km at avg 25 m/s: OSRM duration 400 s. At 1 s sim ticks the
	// trip must take within 5% of 400 s (ramp-up is the only overhead).
	r := straightRoute()
	v := &VehicleState{}
	steps := 0
	for !v.Arrived && steps < 10000 {
		TickVehicle(v, &r, 1.0)
		steps++
	}
	if !v.Arrived {
		t.Fatal("expected arrival")
	}
	got, want := float64(steps), 400.0
	if diff := got - want; diff < -0.05*want || diff > 0.05*want {
		t.Fatalf("trip took %d s, want within 5%% of %.0f s", steps, want)
	}
}

func TestSpeedNeverExceedsLimit(t *testing.T) {
	r := straightRoute()
	v := &VehicleState{}
	for i := 0; i < 600 && !v.Arrived; i++ {
		TickVehicle(v, &r, 1.0)
		if v.SpeedMps > r.AvgMps+1e-9 {
			t.Fatalf("speed %f exceeded limit %f at step %d", v.SpeedMps, r.AvgMps, i)
		}
	}
	if !v.Arrived {
		t.Fatal("expected arrival")
	}
	if rem := r.TotalM - v.DistM; rem != 0 {
		t.Fatalf("must stop within 2 m of destination, remainder %f", rem)
	}
}
