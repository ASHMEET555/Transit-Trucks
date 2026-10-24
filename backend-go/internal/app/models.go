package app

// These mirror the JSON contracts consumed by the existing Next.js frontend and API spec.
type vehicle struct {
	RegistrationNumber string  `json:"registration_number"`
	Model              string  `json:"model"`
	Type               string  `json:"type"`
	MaxLoadCapacity    float64 `json:"max_load_capacity"`
	Odometer           float64 `json:"odometer"`
	AcquisitionCost    float64 `json:"acquisition_cost"`
	Status             string  `json:"status"`
	IsActive           bool    `json:"is_active"`
}

type updateVehicleRequest struct {
	Model           *string  `json:"model"`
	Type            *string  `json:"type"`
	MaxLoadCapacity *float64 `json:"max_load_capacity"`
	Odometer        *float64 `json:"odometer"`
	AcquisitionCost *float64 `json:"acquisition_cost"`
}

type driver struct {
	LicenseNumber     string  `json:"license_number"`
	Name              string  `json:"name"`
	LicenseCategory   string  `json:"license_category"`
	LicenseExpiryDate string  `json:"license_expiry_date"`
	ContactNumber     string  `json:"contact_number"`
	SafetyScore       float64 `json:"safety_score"`
	Status            string  `json:"status"`
	IsActive          bool    `json:"is_active"`
}

type updateDriverRequest struct {
	Name              *string  `json:"name"`
	LicenseCategory   *string  `json:"license_category"`
	LicenseExpiryDate *string  `json:"license_expiry_date"`
	ContactNumber     *string  `json:"contact_number"`
	SafetyScore       *float64 `json:"safety_score"`
}

type destination struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

type PaginatedResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

type trip struct {
	ID              int     `json:"id"`
	SourceName      string  `json:"source_name"`
	DestinationName string  `json:"destination_name"`
	VehicleReg      string  `json:"vehicle_reg"`
	DriverLicense   string  `json:"driver_license"`
	CargoWeight     float64 `json:"cargo_weight"`
	PlannedDistance float64 `json:"planned_distance"`
	Revenue         float64 `json:"revenue"`
	Status          string  `json:"status"`
}

type maintenance struct {
	ID          int     `json:"id"`
	VehicleReg  string  `json:"vehicle_reg"`
	Description string  `json:"description"`
	Cost        float64 `json:"cost"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Status      string  `json:"status"`
}

type expenseRow struct {
	ID     int      `json:"id"`
	TripID int      `json:"trip_id"`
	Type   string   `json:"type"`
	Liters *float64 `json:"liters"`
	Cost   float64  `json:"cost"`
	Date   string   `json:"date"`
}
