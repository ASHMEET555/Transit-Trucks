DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS audit_log;

DROP INDEX IF EXISTS idx_maintenance_vehicle_reg;
DROP INDEX IF EXISTS idx_expenses_trip_id;
DROP INDEX IF EXISTS idx_trips_status_created_at;
DROP INDEX IF EXISTS idx_trips_driver_license;
DROP INDEX IF EXISTS idx_trips_vehicle_reg;

DROP INDEX IF EXISTS idx_maintenance_vehicle_open;
DROP INDEX IF EXISTS idx_trips_driver_dispatched;
DROP INDEX IF EXISTS idx_trips_vehicle_dispatched;

-- Revert expenses constraints
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS check_expenses_liters;
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS check_expenses_cost;
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS check_expenses_type;
ALTER TABLE expenses ALTER COLUMN date TYPE TEXT USING date::TEXT;
ALTER TABLE expenses ALTER COLUMN liters TYPE DOUBLE PRECISION;
ALTER TABLE expenses ALTER COLUMN cost TYPE DOUBLE PRECISION;
ALTER TABLE expenses DROP COLUMN IF EXISTS updated_at;
ALTER TABLE expenses DROP COLUMN IF EXISTS created_at;

-- Revert maintenance_logs constraints
ALTER TABLE maintenance_logs DROP CONSTRAINT IF EXISTS check_maintenance_dates;
ALTER TABLE maintenance_logs DROP CONSTRAINT IF EXISTS check_maintenance_cost;
ALTER TABLE maintenance_logs DROP CONSTRAINT IF EXISTS check_maintenance_status;
ALTER TABLE maintenance_logs ALTER COLUMN end_date TYPE TEXT USING end_date::TEXT;
ALTER TABLE maintenance_logs ALTER COLUMN start_date TYPE TEXT USING start_date::TEXT;
ALTER TABLE maintenance_logs ALTER COLUMN cost TYPE DOUBLE PRECISION;
ALTER TABLE maintenance_logs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE maintenance_logs DROP COLUMN IF EXISTS created_at;

-- Revert trips constraints
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_source_dest;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_revenue;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_planned_distance;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_cargo_weight;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_status;
ALTER TABLE trips ALTER COLUMN revenue TYPE DOUBLE PRECISION;
ALTER TABLE trips ALTER COLUMN planned_distance TYPE DOUBLE PRECISION;
ALTER TABLE trips ALTER COLUMN cargo_weight TYPE DOUBLE PRECISION;
ALTER TABLE trips DROP COLUMN IF EXISTS updated_at;
ALTER TABLE trips DROP COLUMN IF EXISTS created_at;

-- Revert drivers constraints
ALTER TABLE drivers DROP CONSTRAINT IF EXISTS check_drivers_safety_score;
ALTER TABLE drivers DROP CONSTRAINT IF EXISTS check_drivers_status;
ALTER TABLE drivers ALTER COLUMN safety_score TYPE DOUBLE PRECISION;
ALTER TABLE drivers ALTER COLUMN license_expiry_date TYPE TEXT USING license_expiry_date::TEXT;
ALTER TABLE drivers DROP COLUMN IF EXISTS updated_at;
ALTER TABLE drivers DROP COLUMN IF EXISTS created_at;
ALTER TABLE drivers DROP COLUMN IF EXISTS is_active;

-- Revert vehicles constraints
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS check_vehicles_cost;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS check_vehicles_odometer;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS check_vehicles_max_load;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS check_vehicles_status;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS check_vehicles_type;
ALTER TABLE vehicles ALTER COLUMN acquisition_cost TYPE DOUBLE PRECISION;
ALTER TABLE vehicles ALTER COLUMN odometer TYPE DOUBLE PRECISION;
ALTER TABLE vehicles ALTER COLUMN max_load_capacity TYPE DOUBLE PRECISION;
ALTER TABLE vehicles DROP COLUMN IF EXISTS updated_at;
ALTER TABLE vehicles DROP COLUMN IF EXISTS created_at;
ALTER TABLE vehicles DROP COLUMN IF EXISTS is_active;

-- Revert destinations
ALTER TABLE destinations DROP COLUMN IF EXISTS updated_at;
ALTER TABLE destinations DROP COLUMN IF EXISTS created_at;

-- Revert Auth & Sessions
DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS sessions;
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_users_role;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_id_unique;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS created_at;
ALTER TABLE users DROP COLUMN IF EXISTS driver_license;
ALTER TABLE users DROP COLUMN IF EXISTS is_active;
ALTER TABLE users DROP COLUMN IF EXISTS id;
