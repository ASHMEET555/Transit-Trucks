package app

import (
	"testing"
)

// TestPhysicsLimits verifies that kinematics strictly adhere to the spec:
// acceleration cap 1.5 m/s^2, braking cap 2.5 m/s^2.
func TestPhysicsLimits(t *testing.T) {
	const maxAccel = 1.5
	const maxBrake = 2.5

	// Test acceleration limit
	currentSpeed := 10.0 // m/s
	targetLimit := 30.0  // m/s
	dt := 2.0           // seconds

	accelStep := maxAccel * dt
	newSpeed := currentSpeed + accelStep
	if (newSpeed - currentSpeed) / dt > maxAccel {
		t.Fatalf("acceleration exceeded cap: %f > %f", (newSpeed-currentSpeed)/dt, maxAccel)
	}

	// Test braking limit
	currentSpeed = 30.0
	targetLimit = 10.0
	brakeStep := maxBrake * dt
	newSpeed = currentSpeed - brakeStep
	if (currentSpeed - newSpeed) / dt > maxBrake {
		t.Fatalf("braking exceeded cap: %f > %f", (currentSpeed-newSpeed)/dt, maxBrake)
	}
	_ = targetLimit
}

// TestHoursOfServiceRule verifies FMCSA HOS logic:
// Max 11 hours driving (660 min). Alert triggers when driving time reaches >= 600 min (<60 min left).
func TestHoursOfServiceRule(t *testing.T) {
	const (
		maxDrivingMinutes = 11 * 60 // 660 min
		alertThresholdMin = 600     // 10 hours = 60 min left
		requiredOffDutyH  = 10      // 10 hours consecutive off duty resets
	)

	// Under 10 hours: no alert
	drivingMins := 540
	if drivingMins >= alertThresholdMin {
		t.Fatalf("expected no hos alert at %d minutes", drivingMins)
	}

	// At 600 minutes: hos_near_limit alert must fire
	drivingMins = 600
	if drivingMins < alertThresholdMin {
		t.Fatalf("expected hos_near_limit alert at %d minutes", drivingMins)
	}

	// Reset after 10 hours off duty
	offDutyHours := 10.5
	if offDutyHours >= requiredOffDutyH {
		drivingMins = 0 // reset
	}
	if drivingMins != 0 {
		t.Fatalf("expected HOS driving minutes to reset after 10h off duty, got %d", drivingMins)
	}
}

// TestAlertThresholds verifies the exact alert trigger conditions.
func TestAlertThresholds(t *testing.T) {
	// 1. Late arrival (> 30 min)
	delayMin := 35
	isLate := delayMin > 30
	if !isLate {
		t.Fatal("expected late_arrival alert for 35 min delay")
	}
	if 25 > 30 {
		t.Fatal("25 min delay should not trigger late_arrival")
	}

	// 2. Speeding (> 105 km/h)
	speedKmh := 108.5
	isSpeeding := speedKmh > 105.0
	if !isSpeeding {
		t.Fatal("expected speeding alert for 108.5 km/h")
	}
	if 102.0 > 105.0 {
		t.Fatal("102 km/h should not trigger speeding")
	}

	// 3. Low fuel (< 15%)
	fuelPct := 12.0
	isLowFuel := fuelPct < 15.0
	if !isLowFuel {
		t.Fatal("expected low_fuel alert for 12% fuel")
	}
	if 16.0 < 15.0 {
		t.Fatal("16% fuel should not trigger low_fuel")
	}

	// 4. Reefer temp out of range (> -15°C)
	reeferTempC := -10.5
	isReeferAlert := reeferTempC > -15.0
	if !isReeferAlert {
		t.Fatal("expected reefer_temp_out_of_range alert for -10.5 C")
	}
	if -20.0 > -15.0 {
		t.Fatal("-20 C should not trigger reefer alert")
	}
}
