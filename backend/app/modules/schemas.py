# backend/app/modules/schemas.py
from pydantic import BaseModel, EmailStr, Field
from typing import Optional, List

# --- Auth ---
class UserLogin(BaseModel):
    email: EmailStr
    password: str

class Token(BaseModel):
    access_token: str
    token_type: str
    role: str

class UserCreate(BaseModel):
    email: EmailStr
    password: str = Field(..., min_length=6)
    role: str = Field(..., description="Must be one of: 'Fleet Manager', 'Driver', 'Safety Officer', 'Financial Analyst'")

# --- Destination ---
class DestinationCreate(BaseModel):
    name: str = Field(..., min_length=1)
    region: str = Field(..., min_length=1)

class DestinationResponse(DestinationCreate):
    pass

# --- Vehicle ---
class VehicleCreate(BaseModel):
    registration_number: str = Field(..., min_length=2)
    model: str
    type: str
    max_load_capacity: float = Field(..., gt=0)
    odometer: float = Field(..., gte=0)
    acquisition_cost: float = Field(..., gt=0)
    status: Optional[str] = "Available"

class VehicleUpdate(BaseModel):
    model: str
    type: str
    max_load_capacity: float
    odometer: float
    acquisition_cost: float
    status: str

class VehicleResponse(VehicleCreate):
    pass

# --- Driver ---
class DriverCreate(BaseModel):
    license_number: str = Field(..., min_length=2)
    name: str
    license_category: str
    license_expiry_date: str  # YYYY-MM-DD
    contact_number: str
    safety_score: Optional[float] = 100.0
    status: Optional[str] = "Available"

class DriverUpdate(BaseModel):
    name: str
    license_category: str
    license_expiry_date: str
    contact_number: str
    safety_score: float
    status: str

class DriverResponse(DriverCreate):
    pass

# --- Trips ---
class TripCreate(BaseModel):
    source_name: str
    destination_name: str
    vehicle_reg: str
    driver_license: str
    cargo_weight: float = Field(..., gt=0)
    planned_distance: float = Field(..., gt=0)
    revenue: float = Field(..., gte=0)

class TripResponse(TripCreate):
    id: int
    status: str


class TripSuggestionResponse(BaseModel):
    source_name: str
    destination_name: str
    cargo_weight: float
    suggested_vehicle_reg: str
    suggested_driver_license: str
    suggested_driver_name: str
    capacity_utilization: float
    heuristic_score: float

# --- Maintenance ---
class MaintenanceCreate(BaseModel):
    vehicle_reg: str
    description: str = Field(..., min_length=5)
    cost: float = Field(..., gte=0)
    start_date: str  # YYYY-MM-DD

class MaintenanceClose(BaseModel):
    end_date: str  # YYYY-MM-DD

class MaintenanceResponse(MaintenanceCreate):
    id: int
    status: str
    end_date: Optional[str] = None

# --- Expenses / Finance ---
class ExpenseCreate(BaseModel):
    trip_id: int
    type: str = Field(..., description="Fuel | Toll | Maintenance-Surge | Other")
    liters: Optional[float] = None
    cost: float = Field(..., gte=0)
    date: str  # YYYY-MM-DD

class ExpenseResponse(ExpenseCreate):
    id: int

# --- Dashboard Analytics ---
class VehicleAnalytics(BaseModel):
    registration_number: str
    model: str
    type: str
    status: str
    acquisition_cost: float
    revenue: float
    fuel_cost: float
    maintenance_cost: float
    operational_cost: float
    fuel_liters: float
    distance: float
    fuel_efficiency: Optional[float] = None
    roi: Optional[float] = None

class DashboardKPIs(BaseModel):
    active_vehicles: int
    available_vehicles: int
    in_shop_vehicles: int
    retired_vehicles: int
    active_trips: int
    pending_trips: int
    completed_trips: int
    drivers_on_duty: int
    available_drivers: int
    fleet_utilization: float
    overall_fuel_efficiency: Optional[float] = None
    total_operational_cost: float
    total_revenue: float
    vehicles: List[VehicleAnalytics]