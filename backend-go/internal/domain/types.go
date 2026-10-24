package domain

import (
	"errors"
	"fmt"
	"time"
)

type VehicleStatus string

const (
	VehicleStatusAvailable VehicleStatus = "Available"
	VehicleStatusOnTrip    VehicleStatus = "On Trip"
	VehicleStatusInShop    VehicleStatus = "In Shop"
	VehicleStatusRetired   VehicleStatus = "Retired"
)

var ValidVehicleStatuses = map[VehicleStatus]bool{
	VehicleStatusAvailable: true,
	VehicleStatusOnTrip:    true,
	VehicleStatusInShop:    true,
	VehicleStatusRetired:   true,
}

func (s VehicleStatus) IsValid() bool {
	return ValidVehicleStatuses[s]
}

type VehicleType string

const (
	VehicleTypeVan          VehicleType = "Van"
	VehicleTypeBoxTruck     VehicleType = "Box Truck"
	VehicleTypeFlatbed      VehicleType = "Flatbed"
	VehicleTypeSemi         VehicleType = "Semi"
	VehicleTypeLightTruck   VehicleType = "Light Truck"
	VehicleTypeMediumTruck  VehicleType = "Medium Truck"
	VehicleTypeHeavyTruck   VehicleType = "Heavy Truck"
	VehicleTypeRefrigerated VehicleType = "Refrigerated"
)

var ValidVehicleTypes = map[VehicleType]bool{
	VehicleTypeVan:          true,
	VehicleTypeBoxTruck:     true,
	VehicleTypeFlatbed:      true,
	VehicleTypeSemi:         true,
	VehicleTypeLightTruck:   true,
	VehicleTypeMediumTruck:  true,
	VehicleTypeHeavyTruck:   true,
	VehicleTypeRefrigerated: true,
}

func (t VehicleType) IsValid() bool {
	return ValidVehicleTypes[t]
}

type DriverStatus string

const (
	DriverStatusAvailable DriverStatus = "Available"
	DriverStatusOnTrip    DriverStatus = "On Trip"
	DriverStatusSuspended DriverStatus = "Suspended"
	DriverStatusOffDuty   DriverStatus = "Off Duty"
	DriverStatusActive    DriverStatus = "Active"
)

var ValidDriverStatuses = map[DriverStatus]bool{
	DriverStatusAvailable: true,
	DriverStatusOnTrip:    true,
	DriverStatusSuspended: true,
	DriverStatusOffDuty:   true,
	DriverStatusActive:    true,
}

func (s DriverStatus) IsValid() bool {
	return ValidDriverStatuses[s]
}

type ExpenseType string

const (
	ExpenseTypeFuel        ExpenseType = "Fuel"
	ExpenseTypeToll        ExpenseType = "Toll"
	ExpenseTypeMaintenance ExpenseType = "Maintenance"
	ExpenseTypeOther       ExpenseType = "Other"
)

var ValidExpenseTypes = map[ExpenseType]bool{
	ExpenseTypeFuel:        true,
	ExpenseTypeToll:        true,
	ExpenseTypeMaintenance: true,
	ExpenseTypeOther:       true,
}

func (e ExpenseType) IsValid() bool {
	return ValidExpenseTypes[e]
}

type TripStatus string

const (
	TripStatusDraft      TripStatus = "Draft"
	TripStatusDispatched TripStatus = "Dispatched"
	TripStatusCompleted  TripStatus = "Completed"
	TripStatusCancelled  TripStatus = "Cancelled"
)

type MaintenanceStatus string

const (
	MaintenanceStatusOpen    MaintenanceStatus = "Open"
	MaintenanceStatusClosed  MaintenanceStatus = "Closed"
	MaintenanceStatusPending MaintenanceStatus = "Pending"
)

const DateFormat = "2006-01-02"

func ParseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("date is required")
	}
	t, err := time.Parse(DateFormat, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD", s)
	}
	return t, nil
}

func ValidateNonNegative(val float64, fieldName string) error {
	if val < 0 {
		return fmt.Errorf("%s must be non-negative", fieldName)
	}
	return nil
}

func ValidatePositive(val float64, fieldName string) error {
	if val <= 0 {
		return fmt.Errorf("%s must be greater than zero", fieldName)
	}
	return nil
}

func ValidateSafetyScore(val float64) error {
	if val < 0 || val > 100 {
		return errors.New("safety_score must be between 0 and 100")
	}
	return nil
}
