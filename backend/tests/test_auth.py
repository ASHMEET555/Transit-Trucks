from fastapi.testclient import TestClient


def test_register_user_success(client: TestClient):
    response = client.post(
        "/api/auth/register",
        json={
            "email": "new.user@test.com",
            "password": "password123",
            "role": "Driver",
        },
    )
    assert response.status_code == 201
    assert response.json()["email"] == "new.user@test.com"


def test_login_success(client: TestClient):
    response = client.post(
        "/api/auth/login",
        data={"username": "manager@test.com", "password": "password123"},
    )
    assert response.status_code == 200
    payload = response.json()
    assert payload["token_type"] == "bearer"
    assert payload["role"] == "Fleet Manager"
