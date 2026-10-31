-- Migration 000006 down: remove live tracking artifacts

DROP TABLE IF EXISTS route_geometries;
ALTER TABLE destinations DROP COLUMN IF EXISTS longitude;
ALTER TABLE destinations DROP COLUMN IF EXISTS latitude;
DROP INDEX IF EXISTS idx_vehicle_positions_reg_recorded;
DROP TABLE IF EXISTS vehicle_positions;
