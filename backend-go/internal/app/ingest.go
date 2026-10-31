package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PositionInput is the validated payload for a single vehicle position report.
type PositionInput struct {
	VehicleReg  string
	Latitude    float64
	Longitude   float64
	HeadingDeg  float64
	SpeedKmh    float64
	RecordedAt  time.Time
	Source      string
	AllowDevice bool // service-level test flag; the HTTP layer never sets this
}

// IngestError is a validation/gating failure with a stable API error code.
type IngestError struct {
	Code    string
	Message string
	Field   string
}

func (e *IngestError) Error() string { return e.Code + ": " + e.Message }

// IngestPosition validates and stores one position report inside tx.
// Duplicate (vehicle_reg, recorded_at) reports insert no new row and return
// duplicate=true. Rows older than 48h are purged on every insert.
func IngestPosition(ctx context.Context, tx pgx.Tx, input PositionInput) (bool, error) {
	if input.Latitude < -90 || input.Latitude > 90 {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "latitude must be between -90 and 90", Field: "latitude"}
	}
	if input.Longitude < -180 || input.Longitude > 180 {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "longitude must be between -180 and 180", Field: "longitude"}
	}
	if input.HeadingDeg < 0 || input.HeadingDeg > 360 {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "heading_deg must be between 0 and 360", Field: "heading_deg"}
	}
	if input.SpeedKmh < 0 || input.SpeedKmh > 200 {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "speed_kmh must be between 0 and 200", Field: "speed_kmh"}
	}
	if input.Source != "simulator" && input.Source != "device" {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "source must be simulator or device", Field: "source"}
	}
	if input.Source == "device" && !input.AllowDevice {
		return false, &IngestError{Code: "DEVICE_INGEST_NOT_ENABLED", Message: "device ingest is not enabled", Field: "source"}
	}
	if input.RecordedAt.IsZero() {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "recorded_at is required", Field: "recorded_at"}
	}
	now := time.Now().UTC()
	if input.RecordedAt.Before(now.Add(-10*time.Minute)) {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "recorded_at is older than 10 minutes", Field: "recorded_at"}
	}
	if input.RecordedAt.After(now.Add(60 * time.Second)) {
		return false, &IngestError{Code: "VALIDATION_FAILED", Message: "recorded_at is more than 60 seconds in the future", Field: "recorded_at"}
	}

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM vehicle_positions WHERE vehicle_reg = $1 AND recorded_at = $2)`,
		input.VehicleReg, input.RecordedAt.UTC()).Scan(&exists); err != nil {
		return false, fmt.Errorf("duplicate check failed: %w", err)
	}
	if exists {
		return true, nil
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO vehicle_positions (vehicle_reg, latitude, longitude, heading_deg, speed_kmh, recorded_at, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		input.VehicleReg, input.Latitude, input.Longitude, input.HeadingDeg, input.SpeedKmh,
		input.RecordedAt.UTC(), input.Source); err != nil {
		return false, fmt.Errorf("insert position failed: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM vehicle_positions WHERE recorded_at < now() - INTERVAL '48 hours'`); err != nil {
		return false, fmt.Errorf("purge old positions failed: %w", err)
	}

	return false, nil
}
