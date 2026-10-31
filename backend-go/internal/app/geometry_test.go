package app

import (
	"math"
	"testing"
)

func TestPolyline6Roundtrip(t *testing.T) {
	pts := [][2]float64{
		{41.8781, -87.6298},
		{41.0, -88.5},
		{40.2, -90.1},
		{39.7392, -104.9903},
	}
	enc := EncodePolyline6(pts)
	if enc == "" {
		t.Fatal("expected non-empty encoding")
	}
	dec, err := DecodePolyline6(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) != len(pts) {
		t.Fatalf("expected %d points, got %d", len(pts), len(dec))
	}
	for i := range pts {
		if math.Abs(dec[i][0]-pts[i][0]) > 2e-6 || math.Abs(dec[i][1]-pts[i][1]) > 2e-6 {
			t.Fatalf("point %d drifted: want %v got %v", i, pts[i], dec[i])
		}
	}
	if _, err := DecodePolyline6("???"); err == nil {
		t.Fatal("expected error for truncated polyline")
	}
	if _, err := DecodePolyline6(""); err != nil {
		// empty string decodes to zero points without error
		t.Fatalf("empty string should decode cleanly: %v", err)
	}
}

func TestProjection50mOff(t *testing.T) {
	// Straight 10km east-west segment at the equator: 1 degree ~ 111.32km.
	pts := [][2]float64{{0, 0}, {0, 0.0898315}}
	total := RouteLengthM(pts)
	if math.Abs(total-10000) > 25 {
		t.Fatalf("expected ~10km route, got %f", total)
	}
	// Point 50m north of the midpoint.
	offLat := 50.0 / 111320.0
	progress, length, ok := ProjectOntoRoute(offLat, 0.04491575, pts)
	if !ok {
		t.Fatal("expected projection to succeed")
	}
	if math.Abs(length-total) > 1e-6 {
		t.Fatalf("length mismatch: %f vs %f", length, total)
	}
	if math.Abs(progress-total/2) > 5 {
		t.Fatalf("expected progress ~half of %f, got %f", total, progress)
	}
	if _, _, ok := ProjectOntoRoute(0, 0, [][2]float64{{0, 0}}); ok {
		t.Fatal("expected ok=false for degenerate route")
	}
}
