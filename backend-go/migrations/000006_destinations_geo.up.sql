-- Migration 000006: Destination geocoordinates + trip dispatch timestamp for live map
-- Destinations without coordinates appear in lists, never on the map.

ALTER TABLE destinations ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE destinations ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'destinations_latitude_range'
    ) THEN
        ALTER TABLE destinations ADD CONSTRAINT destinations_latitude_range
            CHECK (latitude IS NULL OR (latitude >= -90 AND latitude <= 90));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'destinations_longitude_range'
    ) THEN
        ALTER TABLE destinations ADD CONSTRAINT destinations_longitude_range
            CHECK (longitude IS NULL OR (longitude >= -180 AND longitude <= 180));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'destinations_coords_both_or_neither'
    ) THEN
        ALTER TABLE destinations ADD CONSTRAINT destinations_coords_both_or_neither
            CHECK ((latitude IS NULL) = (longitude IS NULL));
    END IF;
END $$;

-- Dispatch timestamp: powers estimated_progress on the live map.
ALTER TABLE trips ADD COLUMN IF NOT EXISTS dispatched_at TIMESTAMPTZ;
-- Backfill pre-migration dispatches (dispatch always bumped updated_at).
UPDATE trips SET dispatched_at = updated_at WHERE status = 'Dispatched' AND dispatched_at IS NULL;
