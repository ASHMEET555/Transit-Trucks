import importlib
import os
import sqlite3
import sys
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


def _seed_minimal_dataset(db_path: Path) -> None:
    conn = sqlite3.connect(str(db_path))
    cur = conn.cursor()
    cur.execute(
        "INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)",
        ("manager@test.com", "password123", "Fleet Manager"),
    )
    cur.execute(
        "INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)",
        ("finance@test.com", "password123", "Financial Analyst"),
    )
    cur.execute(
        "INSERT INTO destinations (name, region) VALUES (?, ?)",
        ("Warehouse Alpha", "North"),
    )
    cur.execute(
        "INSERT INTO destinations (name, region) VALUES (?, ?)",
        ("Distribution Hub Beta", "East"),
    )
    cur.execute(
        """
        INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        ("VAN-01", "Ford Transit", "Van", 1000.0, 10000.0, 30000.0, "Available"),
    )
    cur.execute(
        """
        INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        ("VAN-02", "Ram ProMaster", "Van", 700.0, 12000.0, 29000.0, "Available"),
    )
    cur.execute(
        """
        INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        ("DL-100001", "Alice Driver", "Commercial", "2029-12-01", "+15550001", 96.0, "Available"),
    )
    cur.execute(
        """
        INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        ("DL-100002", "Bob Driver", "Commercial", "2029-11-20", "+15550002", 90.0, "Available"),
    )
    conn.commit()
    conn.close()


@pytest.fixture()
def client(tmp_path, monkeypatch):
    db_path = tmp_path / "test_transitops.db"
    monkeypatch.setenv("DB_BACKEND", "sqlite")
    monkeypatch.setenv("DB_PATH", str(db_path))
    monkeypatch.setenv("SECRET_KEY", "test-secret")

    import app.db.sqlite_repo as sqlite_repo_module
    import app.db.session as session_module
    import app.db.init.init_db as init_db_module
    import app.core.security as security_module
    import app.main as main_module

    importlib.reload(sqlite_repo_module)
    importlib.reload(session_module)
    importlib.reload(init_db_module)
    importlib.reload(security_module)
    importlib.reload(main_module)

    init_db_module.build_schema()
    _seed_minimal_dataset(db_path)

    with TestClient(main_module.app) as test_client:
        yield test_client


@pytest.fixture()
def manager_token(client: TestClient) -> str:
    response = client.post(
        "/api/auth/login",
        data={"username": "manager@test.com", "password": "password123"},
    )
    assert response.status_code == 200
    return response.json()["access_token"]


@pytest.fixture()
def finance_token(client: TestClient) -> str:
    response = client.post(
        "/api/auth/login",
        data={"username": "finance@test.com", "password": "password123"},
    )
    assert response.status_code == 200
    return response.json()["access_token"]
