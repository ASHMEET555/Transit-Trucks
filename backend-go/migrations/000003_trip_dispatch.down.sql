-- Revert Migration 000003
DROP INDEX IF EXISTS idx_idempotency_keys_expires_at;
ALTER TABLE idempotency_keys DROP COLUMN IF EXISTS expires_at;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_fuel;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_odometer;
ALTER TABLE trips DROP COLUMN IF EXISTS fuel_cost;
ALTER TABLE trips DROP COLUMN IF EXISTS fuel_liters;
ALTER TABLE trips DROP COLUMN IF EXISTS final_odometer;
ALTER TABLE trips DROP COLUMN IF EXISTS start_odometer;
