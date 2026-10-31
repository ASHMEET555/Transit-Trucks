-- Migration 000007 down: drop FK constraints and restore column names.
-- Idempotent: renames tolerate an already-restored table.

ALTER TABLE route_geometries DROP CONSTRAINT IF EXISTS fk_route_geometries_destination;
ALTER TABLE route_geometries DROP CONSTRAINT IF EXISTS fk_route_geometries_origin;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'route_geometries' AND column_name = 'destination_id') THEN
        ALTER TABLE route_geometries RENAME COLUMN destination_id TO destination;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'route_geometries' AND column_name = 'origin_id') THEN
        ALTER TABLE route_geometries RENAME COLUMN origin_id TO origin;
    END IF;
END $$;
