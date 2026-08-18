import os
from typing import Any, Dict, List, Optional

import psycopg2
import psycopg2.extras
from dotenv import load_dotenv

from .base import BaseRepository

load_dotenv()

POSTGRES_DSN = os.getenv("POSTGRES_DSN")


class PostgresRepository(BaseRepository):
    def _get_connection(self):
        if not POSTGRES_DSN:
            raise RuntimeError("POSTGRES_DSN is not set")
        return psycopg2.connect(POSTGRES_DSN)

    def _fetch_one(self, query: str, params: tuple[Any, ...]) -> Optional[Dict[str, Any]]:
        with self._get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cursor:
                cursor.execute(query, params)
                row = cursor.fetchone()
                return dict(row) if row else None

    def _fetch_all(self, query: str, params: tuple[Any, ...] = ()) -> List[Dict[str, Any]]:
        with self._get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cursor:
                cursor.execute(query, params)
                return [dict(row) for row in cursor.fetchall()]

    def create_driver(self, driver_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        driver_data["license_number"],
                        driver_data["name"],
                        driver_data["license_category"],
                        driver_data["license_expiry_date"],
                        driver_data["contact_number"],
                        driver_data.get("safety_score", 100.0),
                        driver_data.get("status", "Available"),
                    ),
                )
            conn.commit()
        return driver_data

    def get_driver_by_license(self, license_number: str) -> Optional[Dict[str, Any]]:
        return self._fetch_one("SELECT * FROM drivers WHERE license_number = %s", (license_number,))

    def update_driver(self, license_number: str, driver_data: Dict[str, Any]) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE drivers
                    SET name=%s, license_category=%s, license_expiry_date=%s, contact_number=%s, safety_score=%s, status=%s
                    WHERE license_number=%s
                    """,
                    (
                        driver_data["name"],
                        driver_data["license_category"],
                        driver_data["license_expiry_date"],
                        driver_data["contact_number"],
                        driver_data["safety_score"],
                        driver_data["status"],
                        license_number,
                    ),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def delete_driver(self, license_number: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute("DELETE FROM drivers WHERE license_number = %s", (license_number,))
                deleted = cursor.rowcount > 0
            conn.commit()
        return deleted

    def create_vehicle(self, vehicle_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        vehicle_data["registration_number"],
                        vehicle_data["model"],
                        vehicle_data["type"],
                        vehicle_data["max_load_capacity"],
                        vehicle_data["odometer"],
                        vehicle_data["acquisition_cost"],
                        vehicle_data.get("status", "Available"),
                    ),
                )
            conn.commit()
        return vehicle_data

    def get_vehicle_by_reg(self, registration_number: str) -> Optional[Dict[str, Any]]:
        return self._fetch_one("SELECT * FROM vehicles WHERE registration_number = %s", (registration_number,))

    def update_vehicle(self, registration_number: str, vehicle_data: Dict[str, Any]) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE vehicles
                    SET model=%s, type=%s, max_load_capacity=%s, odometer=%s, acquisition_cost=%s, status=%s
                    WHERE registration_number=%s
                    """,
                    (
                        vehicle_data["model"],
                        vehicle_data["type"],
                        vehicle_data["max_load_capacity"],
                        vehicle_data["odometer"],
                        vehicle_data["acquisition_cost"],
                        vehicle_data["status"],
                        registration_number,
                    ),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def delete_vehicle(self, registration_number: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute("DELETE FROM vehicles WHERE registration_number = %s", (registration_number,))
                deleted = cursor.rowcount > 0
            conn.commit()
        return deleted

    def create_user(self, user_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    "INSERT INTO users (email, password_hash, role) VALUES (%s, %s, %s)",
                    (user_data["email"], user_data["password_hash"], user_data["role"]),
                )
            conn.commit()
        return user_data

    def get_user_by_email(self, email: str) -> Optional[Dict[str, Any]]:
        return self._fetch_one("SELECT * FROM users WHERE email = %s", (email,))

    def update_user(self, email: str, user_data: Dict[str, Any]) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    "UPDATE users SET password_hash=%s, role=%s WHERE email=%s",
                    (user_data["password_hash"], user_data["role"], email),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def delete_user(self, email: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute("DELETE FROM users WHERE email = %s", (email,))
                deleted = cursor.rowcount > 0
            conn.commit()
        return deleted

    def add_expense(self, expense_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO expenses (trip_id, type, liters, cost, date)
                    VALUES (%s, %s, %s, %s, %s)
                    RETURNING id
                    """,
                    (
                        expense_data["trip_id"],
                        expense_data["type"],
                        expense_data.get("liters"),
                        expense_data["cost"],
                        expense_data["date"],
                    ),
                )
                expense_data["id"] = cursor.fetchone()[0]
            conn.commit()
        return expense_data

    def get_expenses_by_trip(self, trip_id: int) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM expenses WHERE trip_id = %s", (trip_id,))

    def create_destination(self, dest_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    "INSERT INTO destinations (name, region) VALUES (%s, %s)",
                    (dest_data["name"], dest_data["region"]),
                )
            conn.commit()
        return dest_data

    def get_all_destinations(self) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM destinations")

    def delete_destination(self, name: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute("DELETE FROM destinations WHERE name = %s", (name,))
                deleted = cursor.rowcount > 0
            conn.commit()
        return deleted

    def create_maintenance_log(self, log_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO maintenance_logs (vehicle_reg, description, cost, start_date, end_date, status)
                    VALUES (%s, %s, %s, %s, %s, 'Open')
                    RETURNING id
                    """,
                    (
                        log_data["vehicle_reg"],
                        log_data["description"],
                        log_data["cost"],
                        log_data["start_date"],
                        log_data.get("end_date"),
                    ),
                )
                log_data["id"] = cursor.fetchone()[0]
            conn.commit()
        log_data["status"] = "Open"
        if "end_date" not in log_data:
            log_data["end_date"] = None
        return log_data

    def update_maintenance_log(self, log_id: int, log_data: Dict[str, Any]) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE maintenance_logs
                    SET description=%s, cost=%s, start_date=%s, end_date=%s, status=%s
                    WHERE id=%s
                    """,
                    (
                        log_data["description"],
                        log_data["cost"],
                        log_data["start_date"],
                        log_data.get("end_date"),
                        log_data["status"],
                        log_id,
                    ),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def create_trip(self, trip_data: Dict[str, Any]) -> Dict[str, Any]:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                    RETURNING id
                    """,
                    (
                        trip_data["source_name"],
                        trip_data["destination_name"],
                        trip_data["vehicle_reg"],
                        trip_data["driver_license"],
                        trip_data["cargo_weight"],
                        trip_data["planned_distance"],
                        trip_data.get("revenue", 0.0),
                        trip_data.get("status", "Draft"),
                    ),
                )
                trip_data["id"] = cursor.fetchone()[0]
            conn.commit()
        return trip_data

    def update_trip_status(self, trip_id: int, status: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute("UPDATE trips SET status = %s WHERE id = %s", (status, trip_id))
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def get_all_vehicles(self) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM vehicles")

    def get_all_drivers(self) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM drivers")

    def get_all_trips(self) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM trips")

    def get_trips_by_driver(self, license_number: str) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM trips WHERE driver_license = %s", (license_number,))

    def get_trips_by_vehicle(self, registration_number: str) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM trips WHERE vehicle_reg = %s", (registration_number,))

    def get_trips_by_source(self, source_name: str) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM trips WHERE source_name = %s", (source_name,))

    def get_trips_by_destination(self, destination_name: str) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM trips WHERE destination_name = %s", (destination_name,))

    def get_all_maintenance_logs(self) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM maintenance_logs")

    def get_maintenance_logs_by_vehicle(self, registration_number: str) -> List[Dict[str, Any]]:
        return self._fetch_all("SELECT * FROM maintenance_logs WHERE vehicle_reg = %s", (registration_number,))

    def update_vehicle_status(self, registration_number: str, status: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    "UPDATE vehicles SET status = %s WHERE registration_number = %s",
                    (status, registration_number),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated

    def update_driver_status(self, license_number: str, status: str) -> bool:
        with self._get_connection() as conn:
            with conn.cursor() as cursor:
                cursor.execute(
                    "UPDATE drivers SET status = %s WHERE license_number = %s",
                    (status, license_number),
                )
                updated = cursor.rowcount > 0
            conn.commit()
        return updated
