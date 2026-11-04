-- Migration 000008: live extended telemetry & continental depots

ALTER TABLE vehicle_positions ADD COLUMN IF NOT EXISTS fuel_pct DOUBLE PRECISION NULL;
ALTER TABLE vehicle_positions ADD COLUMN IF NOT EXISTS odometer_km DOUBLE PRECISION NULL;
ALTER TABLE vehicle_positions ADD COLUMN IF NOT EXISTS reefer_temp_c DOUBLE PRECISION NULL;
ALTER TABLE vehicle_positions ADD COLUMN IF NOT EXISTS driving_minutes INT NULL;

INSERT INTO destinations (name, region, latitude, longitude) VALUES
  ('Reno Hub', 'West', 39.5296, -119.8138),
  ('Salt Lake City Hub', 'West', 40.7608, -111.8910),
  ('Kansas City Terminal', 'Midwest', 39.0997, -94.5786),
  ('Houston Freight Depot', 'South', 29.7604, -95.3698),
  ('Portland Logistics Hub', 'Northwest', 45.5152, -122.6784),
  ('Phoenix Transshipment Depot', 'Southwest', 33.4484, -112.0740)
ON CONFLICT (name) DO UPDATE SET
  latitude = EXCLUDED.latitude,
  longitude = EXCLUDED.longitude;

UPDATE destinations SET latitude = 41.8781, longitude = -87.6298 WHERE name = 'Chicago Logistics Hub';
UPDATE destinations SET latitude = 32.7767, longitude = -96.7970 WHERE name = 'Dallas Gateway Hub';
UPDATE destinations SET latitude = 33.7490, longitude = -84.3880 WHERE name = 'Atlanta Distribution Hub';
UPDATE destinations SET latitude = 39.7392, longitude = -104.9903 WHERE name = 'Denver Intermodal Hub';
UPDATE destinations SET latitude = 34.0522, longitude = -118.2437 WHERE name = 'Los Angeles Freight Hub';
UPDATE destinations SET latitude = 47.6062, longitude = -122.3321 WHERE name = 'Seattle Terminal';
UPDATE destinations SET latitude = 40.7128, longitude = -74.0060 WHERE name = 'New York Central Hub';
