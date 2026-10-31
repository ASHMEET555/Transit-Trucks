-- Migration 000006: live tracking (vehicle positions, hub coordinates, route geometry cache)

CREATE TABLE IF NOT EXISTS vehicle_positions (
    id BIGSERIAL PRIMARY KEY,
    vehicle_reg TEXT NOT NULL REFERENCES vehicles(registration_number) ON DELETE RESTRICT,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    heading_deg DOUBLE PRECISION NOT NULL,
    speed_kmh DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('simulator', 'device')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_vehicle_positions_reg_recorded
    ON vehicle_positions(vehicle_reg, recorded_at DESC);

ALTER TABLE destinations ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION NULL;
ALTER TABLE destinations ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION NULL;

UPDATE destinations SET latitude = 40.7128, longitude = -74.0060 WHERE name = 'New York Central Hub';
UPDATE destinations SET latitude = 41.8781, longitude = -87.6298 WHERE name = 'Chicago Logistics Hub';
UPDATE destinations SET latitude = 34.0522, longitude = -118.2437 WHERE name = 'Los Angeles Freight Hub';
UPDATE destinations SET latitude = 32.7767, longitude = -96.7970 WHERE name = 'Dallas Gateway Hub';
UPDATE destinations SET latitude = 33.7490, longitude = -84.3880 WHERE name = 'Atlanta Distribution Hub';
UPDATE destinations SET latitude = 47.6062, longitude = -122.3321 WHERE name = 'Seattle Terminal';
UPDATE destinations SET latitude = 39.7392, longitude = -104.9903 WHERE name = 'Denver Intermodal Hub';
UPDATE destinations SET latitude = 25.7617, longitude = -80.1918 WHERE name = 'Miami Port Hub';

CREATE TABLE IF NOT EXISTS route_geometries (
    id BIGSERIAL PRIMARY KEY,
    origin TEXT NOT NULL,
    destination TEXT NOT NULL,
    polyline6 TEXT NOT NULL,
    distance_m DOUBLE PRECISION NOT NULL,
    duration_s DOUBLE PRECISION NOT NULL,
    provider TEXT NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_route_geometries_pair UNIQUE (origin, destination)
);
