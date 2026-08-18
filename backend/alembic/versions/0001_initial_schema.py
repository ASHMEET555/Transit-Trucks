"""initial schema

Revision ID: 0001_initial_schema
Revises:
Create Date: 2026-08-04
"""

from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = "0001_initial_schema"
down_revision: Union[str, Sequence[str], None] = None
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        "users",
        sa.Column("email", sa.Text(), primary_key=True),
        sa.Column("password_hash", sa.Text(), nullable=False),
        sa.Column("role", sa.Text(), nullable=False),
    )

    op.create_table(
        "destinations",
        sa.Column("name", sa.Text(), primary_key=True),
        sa.Column("region", sa.Text(), nullable=False),
    )

    op.create_table(
        "vehicles",
        sa.Column("registration_number", sa.Text(), primary_key=True),
        sa.Column("model", sa.Text(), nullable=False),
        sa.Column("type", sa.Text(), nullable=False),
        sa.Column("max_load_capacity", sa.Float(), nullable=False),
        sa.Column("odometer", sa.Float(), nullable=False),
        sa.Column("acquisition_cost", sa.Float(), nullable=False),
        sa.Column("status", sa.Text(), nullable=False, server_default="Available"),
    )

    op.create_table(
        "drivers",
        sa.Column("license_number", sa.Text(), primary_key=True),
        sa.Column("name", sa.Text(), nullable=False),
        sa.Column("license_category", sa.Text(), nullable=False),
        sa.Column("license_expiry_date", sa.Text(), nullable=False),
        sa.Column("contact_number", sa.Text(), nullable=False),
        sa.Column("safety_score", sa.Float(), nullable=False, server_default="100"),
        sa.Column("status", sa.Text(), nullable=False, server_default="Available"),
    )

    op.create_table(
        "trips",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("source_name", sa.Text(), sa.ForeignKey("destinations.name"), nullable=False),
        sa.Column("destination_name", sa.Text(), sa.ForeignKey("destinations.name"), nullable=False),
        sa.Column("vehicle_reg", sa.Text(), sa.ForeignKey("vehicles.registration_number"), nullable=False),
        sa.Column("driver_license", sa.Text(), sa.ForeignKey("drivers.license_number"), nullable=False),
        sa.Column("cargo_weight", sa.Float(), nullable=False),
        sa.Column("planned_distance", sa.Float(), nullable=False),
        sa.Column("revenue", sa.Float(), nullable=False, server_default="0"),
        sa.Column("status", sa.Text(), nullable=False, server_default="Draft"),
    )

    op.create_table(
        "maintenance_logs",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("vehicle_reg", sa.Text(), sa.ForeignKey("vehicles.registration_number"), nullable=False),
        sa.Column("description", sa.Text(), nullable=False),
        sa.Column("cost", sa.Float(), nullable=False),
        sa.Column("start_date", sa.Text(), nullable=False),
        sa.Column("end_date", sa.Text(), nullable=True),
        sa.Column("status", sa.Text(), nullable=False, server_default="Open"),
    )

    op.create_table(
        "expenses",
        sa.Column("id", sa.Integer(), primary_key=True, autoincrement=True),
        sa.Column("trip_id", sa.Integer(), sa.ForeignKey("trips.id", ondelete="CASCADE"), nullable=False),
        sa.Column("type", sa.Text(), nullable=False),
        sa.Column("liters", sa.Float(), nullable=True),
        sa.Column("cost", sa.Float(), nullable=False),
        sa.Column("date", sa.Text(), nullable=False),
    )


def downgrade() -> None:
    op.drop_table("expenses")
    op.drop_table("maintenance_logs")
    op.drop_table("trips")
    op.drop_table("drivers")
    op.drop_table("vehicles")
    op.drop_table("destinations")
    op.drop_table("users")
