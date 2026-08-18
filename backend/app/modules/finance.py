# backend/app/modules/finance.py
from fastapi import APIRouter, Depends, HTTPException, status
from typing import List

from app.db.session import db
from app.core.dependencies import RoleChecker, get_current_user
from app.core.realtime import fleet_events
from app.modules.schemas import ExpenseCreate, ExpenseResponse

router = APIRouter(prefix="/api/finance", tags=["Finance & Expenses"])

ALLOWED_EXPENSE_TYPES = {"Fuel", "Toll", "Maintenance-Surge", "Other"}


@router.post("/expenses", response_model=ExpenseResponse, status_code=status.HTTP_201_CREATED)
async def create_expense(
    payload: ExpenseCreate,
    current_user: dict = Depends(RoleChecker(["Financial Analyst", "Fleet Manager"])),
):
    if payload.type not in ALLOWED_EXPENSE_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid expense type. Must be one of: {sorted(ALLOWED_EXPENSE_TYPES)}",
        )

    trips = db.get_all_trips()
    trip = next((t for t in trips if t["id"] == payload.trip_id), None)
    if not trip:
        raise HTTPException(status_code=404, detail="Trip not found for expense logging.")

    if payload.type == "Fuel" and (payload.liters is None or payload.liters <= 0):
        raise HTTPException(status_code=400, detail="Fuel expenses require liters > 0.")

    created = db.add_expense(payload.model_dump())
    await fleet_events.broadcast(
        "expense.created",
        {
            "expense_id": created["id"],
            "trip_id": created["trip_id"],
            "type": created["type"],
            "cost": created["cost"],
        },
    )
    return created


@router.get("/expenses/{trip_id}", response_model=List[ExpenseResponse])
def list_expenses_for_trip(
    trip_id: int,
    current_user: dict = Depends(get_current_user),
):
    trips = db.get_all_trips()
    trip = next((t for t in trips if t["id"] == trip_id), None)
    if not trip:
        raise HTTPException(status_code=404, detail="Trip not found.")
    return db.get_expenses_by_trip(trip_id)
