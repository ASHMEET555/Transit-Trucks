from fastapi.testclient import TestClient


def _auth_header(token: str):
    return {"Authorization": f"Bearer {token}"}


def _create_trip(client: TestClient, token: str) -> int:
    response = client.post(
        "/api/trips/",
        headers=_auth_header(token),
        json={
            "source_name": "Warehouse Alpha",
            "destination_name": "Distribution Hub Beta",
            "vehicle_reg": "VAN-01",
            "driver_license": "DL-100001",
            "cargo_weight": 350,
            "planned_distance": 60,
            "revenue": 550,
        },
    )
    assert response.status_code == 201
    return response.json()["id"]


def test_open_and_close_maintenance(client: TestClient, manager_token: str):
    open_resp = client.post(
        "/api/maintenance/",
        headers=_auth_header(manager_token),
        json={
            "vehicle_reg": "VAN-02",
            "description": "Brake pads replacement",
            "cost": 120.0,
            "start_date": "2026-08-01",
        },
    )
    assert open_resp.status_code == 200
    log_id = open_resp.json()["id"]

    close_resp = client.put(
        f"/api/maintenance/{log_id}/close",
        headers=_auth_header(manager_token),
        json={"end_date": "2026-08-03"},
    )
    assert close_resp.status_code == 200


def test_fuel_expense_requires_liters(
    client: TestClient,
    manager_token: str,
    finance_token: str,
):
    trip_id = _create_trip(client, manager_token)
    bad = client.post(
        "/api/finance/expenses",
        headers=_auth_header(finance_token),
        json={
            "trip_id": trip_id,
            "type": "Fuel",
            "liters": 0,
            "cost": 100,
            "date": "2026-08-03",
        },
    )
    assert bad.status_code == 400

    good = client.post(
        "/api/finance/expenses",
        headers=_auth_header(finance_token),
        json={
            "trip_id": trip_id,
            "type": "Fuel",
            "liters": 42.5,
            "cost": 88,
            "date": "2026-08-03",
        },
    )
    assert good.status_code == 201
