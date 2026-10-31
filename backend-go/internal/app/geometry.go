package app

import (
	"fmt"
	"math"
	"strings"
)

// DecodePolyline6 decodes a precision-6 encoded polyline into [lat, lon] pairs.
func DecodePolyline6(s string) ([][2]float64, error) {
	var pts [][2]float64
	var lat, lon int64
	i := 0
	read := func() (int64, error) {
		var result int64
		var shift uint
		for {
			if i >= len(s) {
				return 0, fmt.Errorf("truncated polyline")
			}
			b := int64(s[i]) - 63
			i++
			result |= (b & 0x1f) << shift
			shift += 5
			if b < 0x20 {
				break
			}
		}
		if result&1 != 0 {
			result = ^(result >> 1)
		} else {
			result >>= 1
		}
		return result, nil
	}
	for i < len(s) {
		dLat, err := read()
		if err != nil {
			return nil, err
		}
		dLon, err := read()
		if err != nil {
			return nil, err
		}
		lat += dLat
		lon += dLon
		pts = append(pts, [2]float64{float64(lat) / 1e6, float64(lon) / 1e6})
	}
	return pts, nil
}

// EncodePolyline6 encodes [lat, lon] pairs with precision 6.
func EncodePolyline6(pts [][2]float64) string {
	var sb strings.Builder
	var prevLat, prevLon int64
	write := func(v int64) {
		if v < 0 {
			v = ^(v << 1)
		} else {
			v <<= 1
		}
		for v >= 0x20 {
			sb.WriteByte(byte((0x20 | (v & 0x1f)) + 63))
			v >>= 5
		}
		sb.WriteByte(byte(v + 63))
	}
	for _, p := range pts {
		lat := int64(math.Round(p[0] * 1e6))
		lon := int64(math.Round(p[1] * 1e6))
		write(lat - prevLat)
		write(lon - prevLon)
		prevLat, prevLon = lat, lon
	}
	return sb.String()
}

// ---------- Geo math ----------

const earthRadiusM = 6371000.0

func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(a))
}

// RouteLengthM sums haversine distances along decoded points.
func RouteLengthM(pts [][2]float64) float64 {
	var total float64
	for i := 1; i < len(pts); i++ {
		total += haversineM(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1])
	}
	return total
}

// ProjectOntoRoute finds the nearest point on the polyline to (lat, lon)
// using equirectangular projection per segment. Returns cumulative
// progress in meters and total route length. ok=false when pts < 2.
func ProjectOntoRoute(lat, lon float64, pts [][2]float64) (progressM, totalM float64, ok bool) {
	if len(pts) < 2 {
		return 0, 0, false
	}
	totalM = RouteLengthM(pts)
	meanLat := (lat + pts[0][0]) / 2 * math.Pi / 180
	px := lon * math.Cos(meanLat)
	py := lat
	best := math.MaxFloat64
	var cumulative float64
	for i := 1; i < len(pts); i++ {
		ax := pts[i-1][1] * math.Cos(meanLat)
		ay := pts[i-1][0]
		bx := pts[i][1] * math.Cos(meanLat)
		by := pts[i][0]
		dx := bx - ax
		dy := by - ay
		segLen := haversineM(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1])
		t := 0.0
		if denom := dx*dx + dy*dy; denom > 0 {
			t = ((px-ax)*dx + (py-ay)*dy) / denom
			t = math.Max(0, math.Min(1, t))
		}
		cx := ax + t*dx
		cy := ay + t*dy
		d := math.Hypot(px-cx, py-cy) * math.Pi / 180 * earthRadiusM
		if d < best {
			best = d
			progressM = cumulative + t*segLen
		}
		cumulative += segLen
	}
	return progressM, totalM, true
}
