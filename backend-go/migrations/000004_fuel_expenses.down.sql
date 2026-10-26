-- Revert Migration 000004: Fuel migration to expenses and completed_at on trips

-- 1. Re-add fuel columns to trips
ALTER TABLE trips ADD COLUMN IF NOT EXISTS fuel_liters NUMERIC(10,1);
ALTER TABLE trips ADD COLUMN IF NOT EXISTS fuel_cost NUMERIC(12,2);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'check_trips_fuel') THEN
        ALTER TABLE trips ADD CONSTRAINT check_trips_fuel CHECK ((fuel_liters IS NULL AND fuel_cost IS NULL) OR (fuel_liters > 0 AND fuel_cost > 0));
    END IF;
END $$;

-- 2. Populate fuel columns on trips from expenses WHERE migrated_from_trip = true
UPDATE trips t
SET fuel_liters = e.liters,
    fuel_cost = e.cost
FROM (
    SELECT trip_id, liters, cost,
           ROW_NUMBER() OVER (PARTITION BY trip_id ORDER BY id DESC) as rn
    FROM expenses
    WHERE type = 'Fuel' AND trip_id IS NOT NULL AND (migrated_from_trip = true OR migrated_from_trip IS NULL)
) e
WHERE e.trip_id = t.id AND e.rn = 1;

-- 3. Delete only expenses created from trips migration
DELETE FROM expenses WHERE type = 'Fuel' AND trip_id IS NOT NULL AND migrated_from_trip = true;

-- 4. Drop indexes
DROP INDEX IF EXISTS idx_expenses_type;
DROP INDEX IF EXISTS idx_expenses_date;
DROP INDEX IF EXISTS idx_expenses_vehicle_reg;
DROP INDEX IF EXISTS idx_trips_completed_at;

-- 5. Clean up expenses table
ALTER TABLE expenses DROP COLUMN IF EXISTS migrated_from_trip;
ALTER TABLE expenses DROP COLUMN IF EXISTS vehicle_reg;
DELETE FROM expenses WHERE trip_id IS NULL;
ALTER TABLE expenses ALTER COLUMN trip_id SET NOT NULL;

-- 6. Drop completed_at from trips
ALTER TABLE trips DROP COLUMN IF EXISTS completed_at;
