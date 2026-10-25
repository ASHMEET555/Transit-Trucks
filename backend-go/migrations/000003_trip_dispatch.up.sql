-- Migration 000003: Trip Dispatch, Lifecycle, Idempotency & Mileage Tracking
ALTER TABLE trips ADD COLUMN IF NOT EXISTS start_odometer NUMERIC(10,1);
ALTER TABLE trips ADD COLUMN IF NOT EXISTS final_odometer NUMERIC(10,1);
ALTER TABLE trips ADD COLUMN IF NOT EXISTS fuel_liters NUMERIC(10,1);
ALTER TABLE trips ADD COLUMN IF NOT EXISTS fuel_cost NUMERIC(12,2);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_odometer') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_odometer CHECK (final_odometer IS NULL OR start_odometer IS NULL OR final_odometer >= start_odometer);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_fuel') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_fuel CHECK ((fuel_liters IS NULL AND fuel_cost IS NULL) OR (fuel_liters > 0 AND fuel_cost > 0));
    END IF;
END $$;

ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '24 hours');
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);
