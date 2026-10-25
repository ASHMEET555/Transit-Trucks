package app

import (
	"strings"
	"time"
)

// IsLicenseCategoryCompatible implements the driver license vs vehicle type compatibility table:
// - Category 'Heavy': compatible with all vehicles (Van, Box Truck, Flatbed, Semi, Light Truck, Medium Truck, Heavy Truck, Refrigerated)
// - Category 'Medium': compatible with Van, Light Truck, Medium Truck, Box Truck
// - Category 'Van' / 'Light': compatible with Van, Light Truck
func IsLicenseCategoryCompatible(driverCategory, vehicleType string) bool {
	cat := strings.TrimSpace(driverCategory)
	vType := strings.TrimSpace(vehicleType)

	switch cat {
	case "Heavy":
		return true
	case "Medium":
		switch vType {
		case "Van", "Light Truck", "Medium Truck", "Box Truck":
			return true
		default:
			return false
		}
	case "Van", "Light":
		switch vType {
		case "Van", "Light Truck":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// IsDriverLicenseExpired checks if license_expiry_date is before today (UTC).
func IsDriverLicenseExpired(expiryDate string, dispatchDate time.Time) bool {
	t, err := time.Parse("2006-01-02", expiryDate)
	if err != nil {
		return true
	}
	d := time.Date(dispatchDate.Year(), dispatchDate.Month(), dispatchDate.Day(), 0, 0, 0, 0, time.UTC)
	return t.Before(d)
}
