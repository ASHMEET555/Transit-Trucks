-- name: GetVehicle :one
SELECT * FROM vehicles
WHERE registration_number = $1 LIMIT 1;

-- name: GetVehicleForUpdate :one
SELECT * FROM vehicles
WHERE registration_number = $1 FOR UPDATE;

-- name: CreateVehicle :one
INSERT INTO vehicles (
    registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateVehicle :one
UPDATE vehicles
SET
    model = $2,
    type = $3,
    max_load_capacity = $4,
    odometer = $5,
    acquisition_cost = $6,
    updated_at = now()
WHERE registration_number = $1
RETURNING *;

-- name: UpdateVehicleStatus :one
UPDATE vehicles
SET
    status = $2,
    updated_at = now()
WHERE registration_number = $1
RETURNING *;

-- name: SoftDeleteVehicle :exec
UPDATE vehicles
SET
    is_active = false,
    updated_at = now()
WHERE registration_number = $1;

-- name: HardDeleteVehicle :exec
DELETE FROM vehicles
WHERE registration_number = $1;

-- name: CountTripsForVehicle :one
SELECT COUNT(*) FROM trips
WHERE vehicle_reg = $1;

-- name: CountDispatchedTripsForVehicle :one
SELECT COUNT(*) FROM trips
WHERE vehicle_reg = $1 AND status = 'Dispatched';

-- name: CountOpenMaintenanceForVehicle :one
SELECT COUNT(*) FROM maintenance_logs
WHERE vehicle_reg = $1 AND status = 'Open';

-- name: ListVehicles :many
SELECT * FROM vehicles
WHERE
    ($1::text = '' OR registration_number > $1)
    AND ($2::text = '' OR type = $2)
    AND ($3::text = '' OR status = $3)
    AND ($4::boolean OR is_active = true)
    AND ($5::text = '' OR (registration_number ILIKE '%' || $5 || '%' OR model ILIKE '%' || $5 || '%'))
ORDER BY registration_number ASC
LIMIT $6;
