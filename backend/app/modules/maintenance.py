# backend/app/modules/maintenance.py
from fastapi import APIRouter, Depends, HTTPException, status
import datetime
from app.db.session import db
from app.core.dependencies import get_current_user, RoleChecker
from app.core.realtime import fleet_events
from app.modules.schemas import MaintenanceCreate, MaintenanceClose, MaintenanceResponse
from typing import List

router = APIRouter(prefix="/api/maintenance", tags=["Fleet Maintenance Operations"])

@router.get("/", response_model=List[MaintenanceResponse])
def list_maintenance_orders(current_user: dict = Depends(get_current_user)):
    return db.get_all_maintenance_logs()

@router.post("/", response_model=MaintenanceResponse)
async def open_workshop_work_order(payload: MaintenanceCreate, current_user: dict = Depends(RoleChecker(["Fleet Manager"]))):
    vehicle = db.get_vehicle_by_reg(payload.vehicle_reg)
    if not vehicle:
        raise HTTPException(status_code=404, detail="Vehicle not found.")
    if vehicle["status"] == "On Trip":
        raise HTTPException(status_code=400, detail="Cannot pull an active vehicle off transit into maintenance.")
        
    # 1. Log Maintenance File Entry
    created_log = db.create_maintenance_log(payload.model_dump())
    
    # 2. Cascade Status Change to "In Shop"[cite: 1]
    db.update_vehicle_status(payload.vehicle_reg, "In Shop")
    
    await fleet_events.broadcast(
        "maintenance.opened",
        {"log_id": created_log["id"], "vehicle_reg": payload.vehicle_reg, "status": "Open"},
    )
    return created_log

@router.put("/{log_id}/close")
async def close_workshop_work_order(log_id: int, payload: MaintenanceClose, current_user: dict = Depends(RoleChecker(["Fleet Manager"]))):
    logs = db.get_all_maintenance_logs()
    target_log = next((l for l in logs if l["id"] == log_id), None)
    
    if not target_log:
        raise HTTPException(status_code=404, detail="Maintenance log index not found.")
    if target_log["status"] == "Closed":
        raise HTTPException(status_code=400, detail="Work order is already closed.")
        
    # 1. Close out work order entry
    target_log["status"] = "Closed"
    target_log["end_date"] = payload.end_date
    db.update_maintenance_log(log_id, target_log)
    
    # 2. Release Vehicle back to Available Pool[cite: 1]
    db.update_vehicle_status(target_log["vehicle_reg"], "Available")
    
    await fleet_events.broadcast(
        "maintenance.closed",
        {"log_id": log_id, "vehicle_reg": target_log["vehicle_reg"], "status": "Closed"},
    )
    return {"message": "Workshop operations finished. Vehicle returned to fleet."}