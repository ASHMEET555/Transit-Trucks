-- Phase 1 Auth & Sessions
ALTER TABLE users ADD COLUMN IF NOT EXISTS id TEXT;
UPDATE users SET id = email WHERE id IS NULL;
ALTER TABLE users ALTER COLUMN id SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'users_id_unique'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT users_id_unique UNIQUE (id);
    END IF;
END $$;
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE users ADD COLUMN IF NOT EXISTS driver_license TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_users_role'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT check_users_role CHECK (role IN ('Admin', 'Fleet Manager', 'Driver', 'Safety Officer', 'Financial Analyst'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_email TEXT NOT NULL REFERENCES users(email) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_email ON sessions(user_email);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS login_attempts (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    ip TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_email_at ON login_attempts(email, attempted_at);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_at ON login_attempts(ip, attempted_at);

-- Destinations
ALTER TABLE destinations ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE destinations ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Vehicles
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE vehicles ALTER COLUMN max_load_capacity TYPE NUMERIC(10,1);
ALTER TABLE vehicles ALTER COLUMN odometer TYPE NUMERIC(10,1);
ALTER TABLE vehicles ALTER COLUMN acquisition_cost TYPE NUMERIC(12,2);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_vehicles_type') THEN
        ALTER TABLE vehicles ADD CONSTRAINT check_vehicles_type CHECK (type IN ('Van', 'Box Truck', 'Flatbed', 'Semi', 'Light Truck', 'Medium Truck', 'Heavy Truck', 'Refrigerated'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_vehicles_status') THEN
        ALTER TABLE vehicles ADD CONSTRAINT check_vehicles_status CHECK (status IN ('Available', 'On Trip', 'In Shop', 'Retired'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_vehicles_max_load') THEN
        ALTER TABLE vehicles ADD CONSTRAINT check_vehicles_max_load CHECK (max_load_capacity > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_vehicles_odometer') THEN
        ALTER TABLE vehicles ADD CONSTRAINT check_vehicles_odometer CHECK (odometer >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_vehicles_cost') THEN
        ALTER TABLE vehicles ADD CONSTRAINT check_vehicles_cost CHECK (acquisition_cost >= 0);
    END IF;
END $$;

-- Drivers
ALTER TABLE drivers ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE drivers ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE drivers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE drivers ALTER COLUMN license_expiry_date TYPE DATE USING license_expiry_date::DATE;
ALTER TABLE drivers ALTER COLUMN safety_score TYPE NUMERIC(5,1);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_drivers_status') THEN
        ALTER TABLE drivers ADD CONSTRAINT check_drivers_status CHECK (status IN ('Available', 'On Trip', 'Suspended', 'Off Duty', 'Active'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_drivers_safety_score') THEN
        ALTER TABLE drivers ADD CONSTRAINT check_drivers_safety_score CHECK (safety_score >= 0 AND safety_score <= 100);
    END IF;
END $$;

-- Trips
ALTER TABLE trips ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE trips ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE trips ALTER COLUMN cargo_weight TYPE NUMERIC(10,1);
ALTER TABLE trips ALTER COLUMN planned_distance TYPE NUMERIC(10,1);
ALTER TABLE trips ALTER COLUMN revenue TYPE NUMERIC(12,2);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_status') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_status CHECK (status IN ('Draft', 'Dispatched', 'Completed', 'Cancelled'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_cargo_weight') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_cargo_weight CHECK (cargo_weight > 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_planned_distance') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_planned_distance CHECK (planned_distance >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_revenue') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_revenue CHECK (revenue >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_source_dest') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_source_dest CHECK (source_name <> destination_name);
    END IF;
END $$;

-- Enforce ON DELETE RESTRICT on trips
ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_vehicle_reg_fkey;
ALTER TABLE trips ADD CONSTRAINT trips_vehicle_reg_fkey FOREIGN KEY (vehicle_reg) REFERENCES vehicles(registration_number) ON DELETE RESTRICT;

ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_driver_license_fkey;
ALTER TABLE trips ADD CONSTRAINT trips_driver_license_fkey FOREIGN KEY (driver_license) REFERENCES drivers(license_number) ON DELETE RESTRICT;

ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_source_name_fkey;
ALTER TABLE trips ADD CONSTRAINT trips_source_name_fkey FOREIGN KEY (source_name) REFERENCES destinations(name) ON DELETE RESTRICT;

ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_destination_name_fkey;
ALTER TABLE trips ADD CONSTRAINT trips_destination_name_fkey FOREIGN KEY (destination_name) REFERENCES destinations(name) ON DELETE RESTRICT;

-- Maintenance Logs
ALTER TABLE maintenance_logs ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE maintenance_logs ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE maintenance_logs ALTER COLUMN cost TYPE NUMERIC(12,2);
ALTER TABLE maintenance_logs ALTER COLUMN start_date TYPE DATE USING start_date::DATE;
ALTER TABLE maintenance_logs ALTER COLUMN end_date TYPE DATE USING NULLIF(end_date, '')::DATE;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_maintenance_status') THEN
        ALTER TABLE maintenance_logs ADD CONSTRAINT check_maintenance_status CHECK (status IN ('Open', 'Closed', 'Completed'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_maintenance_cost') THEN
        ALTER TABLE maintenance_logs ADD CONSTRAINT check_maintenance_cost CHECK (cost >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_maintenance_dates') THEN
        ALTER TABLE maintenance_logs ADD CONSTRAINT check_maintenance_dates CHECK (end_date IS NULL OR end_date >= start_date);
    END IF;
END $$;

ALTER TABLE maintenance_logs DROP CONSTRAINT IF EXISTS maintenance_logs_vehicle_reg_fkey;
ALTER TABLE maintenance_logs ADD CONSTRAINT maintenance_logs_vehicle_reg_fkey FOREIGN KEY (vehicle_reg) REFERENCES vehicles(registration_number) ON DELETE RESTRICT;

-- Expenses
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE expenses ALTER COLUMN cost TYPE NUMERIC(12,2);
ALTER TABLE expenses ALTER COLUMN liters TYPE NUMERIC(10,1);
ALTER TABLE expenses ALTER COLUMN date TYPE DATE USING date::DATE;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_expenses_type') THEN
        ALTER TABLE expenses ADD CONSTRAINT check_expenses_type CHECK (type IN ('Fuel', 'Toll', 'Maintenance', 'Maintenance-Surge', 'Other'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_expenses_cost') THEN
        ALTER TABLE expenses ADD CONSTRAINT check_expenses_cost CHECK (cost >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_expenses_liters') THEN
        ALTER TABLE expenses ADD CONSTRAINT check_expenses_liters CHECK (liters IS NULL OR liters >= 0);
    END IF;
END $$;

ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_trip_id_fkey;
ALTER TABLE expenses ADD CONSTRAINT expenses_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE RESTRICT;

-- Partial unique indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_trips_vehicle_dispatched ON trips(vehicle_reg) WHERE status = 'Dispatched';
CREATE UNIQUE INDEX IF NOT EXISTS idx_trips_driver_dispatched ON trips(driver_license) WHERE status = 'Dispatched';
CREATE UNIQUE INDEX IF NOT EXISTS idx_maintenance_vehicle_open ON maintenance_logs(vehicle_reg) WHERE status = 'Open';

-- Indexes for known queries
CREATE INDEX IF NOT EXISTS idx_trips_vehicle_reg ON trips(vehicle_reg);
CREATE INDEX IF NOT EXISTS idx_trips_driver_license ON trips(driver_license);
CREATE INDEX IF NOT EXISTS idx_trips_status_created_at ON trips(status, created_at);
CREATE INDEX IF NOT EXISTS idx_expenses_trip_id ON expenses(trip_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_vehicle_reg ON maintenance_logs(vehicle_reg);

-- Audit Log & Idempotency Keys tables
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    entity TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    action TEXT NOT NULL,
    actor_email TEXT,
    details JSONB,
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log(entity, entity_id, at);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT NOT NULL,
    user_id TEXT NOT NULL,
    route TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status_code INT NOT NULL,
    response_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);
