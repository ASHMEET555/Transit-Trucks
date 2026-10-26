-- Migration 000004: Fuel migration to expenses and completed_at on trips

-- 1. Add completed_at column to trips
ALTER TABLE trips ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;
UPDATE trips SET completed_at = updated_at WHERE status = 'Completed' AND completed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_trips_completed_at ON trips(completed_at);

-- 2. Modify expenses table to support vehicle_reg and optional trip_id
ALTER TABLE expenses ALTER COLUMN trip_id DROP NOT NULL;
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS vehicle_reg TEXT REFERENCES vehicles(registration_number) ON DELETE RESTRICT;

-- Backfill vehicle_reg for existing expenses from trips
UPDATE expenses e
SET vehicle_reg = t.vehicle_reg
FROM trips t
WHERE e.trip_id = t.id AND e.vehicle_reg IS NULL;

-- 3. Migrate fuel_liters and fuel_cost from completed trips into expenses
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'trips' AND column_name = 'fuel_liters'
    ) THEN
        INSERT INTO expenses (trip_id, vehicle_reg, type, liters, cost, date, created_at, updated_at)
        SELECT
            t.id,
            t.vehicle_reg,
            'Fuel',
            t.fuel_liters,
            t.fuel_cost,
            (t.updated_at AT TIME ZONE 'UTC')::date,
            t.updated_at,
            t.updated_at
        FROM trips t
        WHERE t.status = 'Completed'
          AND (COALESCE(t.fuel_cost, 0) > 0 OR COALESCE(t.fuel_liters, 0) > 0);
    END IF;
END $$;

-- 4. Drop check constraint check_trips_fuel and fuel columns from trips
ALTER TABLE trips DROP CONSTRAINT IF EXISTS check_trips_fuel;
ALTER TABLE trips DROP COLUMN IF EXISTS fuel_liters;
ALTER TABLE trips DROP COLUMN IF EXISTS fuel_cost;

-- 5. Indexes for expenses query filtering
CREATE INDEX IF NOT EXISTS idx_expenses_vehicle_reg ON expenses(vehicle_reg);
CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date);
CREATE INDEX IF NOT EXISTS idx_expenses_type ON expenses(type);
