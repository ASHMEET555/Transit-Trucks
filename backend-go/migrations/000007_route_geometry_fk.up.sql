-- Migration 000007: route_geometries hub references become foreign keys.
-- destinations PK is name (TEXT), so rename columns to origin_id/destination_id
-- and constrain them; pair uniqueness is preserved.
-- All statements are idempotent: the suite re-applies pending migrations
-- after `force 5`, so renames must tolerate an already-renamed table.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'route_geometries' AND column_name = 'origin') THEN
        ALTER TABLE route_geometries RENAME COLUMN origin TO origin_id;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'route_geometries' AND column_name = 'destination') THEN
        ALTER TABLE route_geometries RENAME COLUMN destination TO destination_id;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_route_geometries_origin') THEN
        ALTER TABLE route_geometries ADD CONSTRAINT fk_route_geometries_origin
            FOREIGN KEY (origin_id) REFERENCES destinations(name) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_route_geometries_destination') THEN
        ALTER TABLE route_geometries ADD CONSTRAINT fk_route_geometries_destination
            FOREIGN KEY (destination_id) REFERENCES destinations(name) ON DELETE RESTRICT;
    END IF;
END $$;
