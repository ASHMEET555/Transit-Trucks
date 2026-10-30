-- Revert Migration 000006: Destination geocoordinates + dispatch timestamp
ALTER TABLE trips DROP COLUMN IF EXISTS dispatched_at;
ALTER TABLE destinations DROP CONSTRAINT IF EXISTS destinations_coords_both_or_neither;
ALTER TABLE destinations DROP CONSTRAINT IF EXISTS destinations_longitude_range;
ALTER TABLE destinations DROP CONSTRAINT IF EXISTS destinations_latitude_range;
ALTER TABLE destinations DROP COLUMN IF EXISTS longitude;
ALTER TABLE destinations DROP COLUMN IF EXISTS latitude;
