-- Migration 000008 down: live extended telemetry

ALTER TABLE vehicle_positions DROP COLUMN IF EXISTS fuel_pct;
ALTER TABLE vehicle_positions DROP COLUMN IF EXISTS odometer_km;
ALTER TABLE vehicle_positions DROP COLUMN IF EXISTS reefer_temp_c;
ALTER TABLE vehicle_positions DROP COLUMN IF EXISTS driving_minutes;
