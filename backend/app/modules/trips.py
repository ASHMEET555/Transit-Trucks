# backend/app/modules/trips.py
from fastapi import APIRouter, Depends, HTTPException, status
import datetime
from app.db.session import db
from app.core.dependencies import get_current_user, RoleChecker
from app.core.realtime import fleet_events
from app.modules.schemas import TripCreate, TripResponse, TripSuggestionResponse
from typing import List

router = APIRouter(prefix="/api/trips", tags=["Trip Dispatch Management"])

@router.get("/", response_model=List[TripResponse])
def list_all_trips(current_user: dict = Depends(get_current_user)):
    return db.get_all_trips()


@router.post("/suggest", response_model=TripSuggestionResponse)
def suggest_trip_assignment(
    payload: TripCreate,
    current_user: dict = Depends(RoleChecker(["Fleet Manager", "Driver"])),
):
    if payload.source_name == payload.destination_name:
        raise HTTPException(status_code=400, detail="Source and destination must be different.")

    today = datetime.date.today().isoformat()
    vehicles = [
        v
        for v in db.get_all_vehicles()
        if v["status"] == "Available" and v["max_load_capacity"] >= payload.cargo_weight
    ]
    if not vehicles:
        raise HTTPException(status_code=404, detail="No available vehicle can carry this cargo.")

    drivers = [
        d
        for d in db.get_all_drivers()
        if d["status"] == "Available" and d["license_expiry_date"] > today
    ]
    if not drivers:
        raise HTTPException(status_code=404, detail="No available and compliant driver found.")

    # Heuristic:
    # - Prefer tighter capacity fit (less unused capacity)
    # - Prefer higher safety score
    # - Prefer lower odometer to balance fleet usage
    best = None
    for vehicle in vehicles:
        unused_capacity = vehicle["max_load_capacity"] - payload.cargo_weight
        utilization = payload.cargo_weight / vehicle["max_load_capacity"]
        for driver in drivers:
            score = (
                utilization * 100
                + float(driver.get("safety_score", 0)) * 0.6
                - float(vehicle.get("odometer", 0)) / 100000.0
                - unused_capacity / 1000.0
            )
            if best is None or score > best["score"]:
                best = {"vehicle": vehicle, "driver": driver, "score": score, "utilization": utilization}

    if best is None:
        raise HTTPException(status_code=404, detail="No feasible assignment found.")

    return TripSuggestionResponse(
        source_name=payload.source_name,
        destination_name=payload.destination_name,
        cargo_weight=payload.cargo_weight,
        suggested_vehicle_reg=best["vehicle"]["registration_number"],
        suggested_driver_license=best["driver"]["license_number"],
        suggested_driver_name=best["driver"]["name"],
        capacity_utilization=round(best["utilization"], 4),
        heuristic_score=round(best["score"], 2),
    )


@router.post("/", response_model=TripResponse, status_code=status.HTTP_201_CREATED)
async def dispatch_new_trip(payload: TripCreate, current_user: dict = Depends(RoleChecker(["Fleet Manager", "Driver"]))):
    # 1. Fetch Target Asset Contexts
    vehicle = db.get_vehicle_by_reg(payload.vehicle_reg)
    driver = db.get_driver_by_license(payload.driver_license)
    
    if not vehicle or not driver:
        raise HTTPException(status_code=404, detail="Assigned Vehicle or Driver profile not found.")
        
    # 2. Strict Business Rule Validation
    # Rule A: Cargo Weight Limit Check
    if payload.cargo_weight > vehicle["max_load_capacity"]:
        raise HTTPException(
            status_code=400, 
            detail=f"Overload Error: Cargo ({payload.cargo_weight}kg) exceeds max vehicle capacity ({vehicle['max_load_capacity']}kg)[cite: 1]."
        )
        
    # Rule B: Asset Availability Checks[cite: 1]
    if vehicle["status"] in ('In Shop', 'Retired'):
        raise HTTPException(status_code=400, detail=f"Vehicle cannot be dispatched. Current Status: {vehicle['status']}[cite: 1].")
    if vehicle["status"] == 'On Trip':
        raise HTTPException(status_code=400, detail="Conflict: Vehicle is already actively deployed on another trip[cite: 1].")
        
    # Rule C: Driver Eligibility & License Validation[cite: 1]
    today = datetime.date.today().isoformat()
    if driver["license_expiry_date"] <= today:
        raise HTTPException(status_code=400, detail="Compliance Failure: Driver license has expired[cite: 1].")
    if driver["status"] == 'Suspended':
        raise HTTPException(status_code=400, detail="Compliance Failure: Assigned driver is currently suspended[cite: 1].")
    if driver["status"] == 'On Trip':
        raise HTTPException(status_code=400, detail="Conflict: Driver is already assigned to an active trip[cite: 1].")

    # 3. Create the Base Trip Entry
    trip_data = payload.model_dump()
    trip_data["status"] = "Dispatched"  # Auto-dispatch on successful validation[cite: 1]
    created_trip = db.create_trip(trip_data)
    
    # 4. State Cascade: Move Assets to 'On Trip'[cite: 1]
    db.update_vehicle_status(payload.vehicle_reg, "On Trip")
    db.update_driver_status(payload.driver_license, "On Trip")
    
    await fleet_events.broadcast(
        "trip.dispatched",
        {
            "trip_id": created_trip["id"],
            "vehicle_reg": payload.vehicle_reg,
            "driver_license": payload.driver_license,
            "status": "Dispatched",
        },
    )
    return created_trip

@router.put("/{trip_id}/complete")
async def complete_active_transit(trip_id: int, final_odometer: float, current_user: dict = Depends(get_current_user)):
    # Find all active trips matching this ID
    trips = db.get_all_trips()
    target_trip = next((t for t in trips if t["id"] == trip_id), None)
    
    if not target_trip:
        raise HTTPException(status_code=404, detail="Trip record not found.")
    if target_trip["status"] != "Dispatched":
        raise HTTPException(status_code=400, detail="Only actively dispatched transits can be finalized.")
        
    vehicle = db.get_vehicle_by_reg(target_trip["vehicle_reg"])
    if final_odometer <= vehicle["odometer"]:
        raise HTTPException(status_code=400, detail=f"Odometer reading must be higher than starting values ({vehicle['odometer']}).")
        
    # 1. Update Lifecycle State[cite: 1]
    db.update_trip_status(trip_id, "Completed")
    
    # 2. Update Odometer Log Metrics
    vehicle["odometer"] = final_odometer
    vehicle["status"] = "Available"
    db.update_vehicle(target_trip["vehicle_reg"], vehicle)
    
    # 3. Release Driver[cite: 1]
    db.update_driver_status(target_trip["driver_license"], "Available")
    
    await fleet_events.broadcast(
        "trip.completed",
        {
            "trip_id": trip_id,
            "vehicle_reg": target_trip["vehicle_reg"],
            "driver_license": target_trip["driver_license"],
            "status": "Completed",
            "final_odometer": final_odometer,
        },
    )
    return {"message": "Trip completed successfully. Resources released back to active pools[cite: 1]."}


@router.put("/{trip_id}/cancel")
async def cancel_active_transit(trip_id: int, current_user: dict = Depends(get_current_user)):
    trips = db.get_all_trips()
    target_trip = next((t for t in trips if t["id"] == trip_id), None)

    if not target_trip:
        raise HTTPException(status_code=404, detail="Trip record not found.")
    if target_trip["status"] != "Dispatched":
        raise HTTPException(
            status_code=400,
            detail="Only actively dispatched transits can be cancelled.",
        )

    db.update_trip_status(trip_id, "Cancelled")
    db.update_vehicle_status(target_trip["vehicle_reg"], "Available")
    db.update_driver_status(target_trip["driver_license"], "Available")

    await fleet_events.broadcast(
        "trip.cancelled",
        {
            "trip_id": trip_id,
            "vehicle_reg": target_trip["vehicle_reg"],
            "driver_license": target_trip["driver_license"],
            "status": "Cancelled",
        },
    )
    return {"message": "Trip cancelled. Vehicle and driver restored to Available."}
