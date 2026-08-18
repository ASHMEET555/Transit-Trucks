from fastapi.testclient import TestClient


def _auth_header(token: str):
    return {"Authorization": f"Bearer {token}"}


def test_suggest_trip_assignment(client: TestClient, manager_token: str):
    response = client.post(
        "/api/trips/suggest",
        headers=_auth_header(manager_token),
        json={
            "source_name": "Warehouse Alpha",
            "destination_name": "Distribution Hub Beta",
            "vehicle_reg": "placeholder",
            "driver_license": "placeholder",
            "cargo_weight": 500,
            "planned_distance": 80,
            "revenue": 600,
        },
    )
    assert response.status_code == 200
    body = response.json()
    assert body["suggested_vehicle_reg"] in {"VAN-01", "VAN-02"}
    assert body["suggested_driver_license"] in {"DL-100001", "DL-100002"}


def test_dispatch_then_complete_trip(client: TestClient, manager_token: str):
    dispatch = client.post(
        "/api/trips/",
        headers=_auth_header(manager_token),
        json={
            "source_name": "Warehouse Alpha",
            "destination_name": "Distribution Hub Beta",
            "vehicle_reg": "VAN-01",
            "driver_license": "DL-100001",
            "cargo_weight": 450,
            "planned_distance": 110,
            "revenue": 1000,
        },
    )
    assert dispatch.status_code == 201
    trip_id = dispatch.json()["id"]

    complete = client.put(
        f"/api/trips/{trip_id}/complete?final_odometer=10300",
        headers=_auth_header(manager_token),
    )
    assert complete.status_code == 200


def test_overload_rejected(client: TestClient, manager_token: str):
    response = client.post(
        "/api/trips/",
        headers=_auth_header(manager_token),
        json={
            "source_name": "Warehouse Alpha",
            "destination_name": "Distribution Hub Beta",
            "vehicle_reg": "VAN-02",
            "driver_license": "DL-100002",
            "cargo_weight": 5000,
            "planned_distance": 90,
            "revenue": 700,
        },
    )
    assert response.status_code == 400


def test_trip_websocket_event_on_dispatch(client: TestClient, manager_token: str):
    with client.websocket_connect("/ws/fleet") as ws:
        response = client.post(
            "/api/trips/",
            headers=_auth_header(manager_token),
            json={
                "source_name": "Warehouse Alpha",
                "destination_name": "Distribution Hub Beta",
                "vehicle_reg": "VAN-01",
                "driver_license": "DL-100001",
                "cargo_weight": 300,
                "planned_distance": 70,
                "revenue": 500,
            },
        )
        assert response.status_code == 201
        message = ws.receive_json()
        assert message["event"] == "trip.dispatched"
