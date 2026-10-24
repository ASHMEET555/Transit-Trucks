-- name: GetDriver :one
SELECT * FROM drivers
WHERE license_number = $1 LIMIT 1;

-- name: GetDriverForUpdate :one
SELECT * FROM drivers
WHERE license_number = $1 FOR UPDATE;

-- name: CreateDriver :one
INSERT INTO drivers (
    license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateDriver :one
UPDATE drivers
SET
    name = $2,
    license_category = $3,
    license_expiry_date = $4,
    contact_number = $5,
    safety_score = $6,
    updated_at = now()
WHERE license_number = $1
RETURNING *;

-- name: UpdateDriverStatus :one
UPDATE drivers
SET
    status = $2,
    updated_at = now()
WHERE license_number = $1
RETURNING *;

-- name: SoftDeleteDriver :exec
UPDATE drivers
SET
    is_active = false,
    updated_at = now()
WHERE license_number = $1;

-- name: HardDeleteDriver :exec
DELETE FROM drivers
WHERE license_number = $1;

-- name: CountTripsForDriver :one
SELECT COUNT(*) FROM trips
WHERE driver_license = $1;

-- name: CountDispatchedTripsForDriver :one
SELECT COUNT(*) FROM trips
WHERE driver_license = $1 AND status = 'Dispatched';

-- name: ListDrivers :many
SELECT * FROM drivers
WHERE
    ($1::text = '' OR license_number > $1)
    AND ($2::text = '' OR status = $2)
    AND ($3::boolean OR is_active = true)
    AND ($4::text = '' OR (name ILIKE '%' || $4 || '%' OR license_number ILIKE '%' || $4 || '%'))
    AND ($5::int = 0 OR (license_expiry_date <= CURRENT_DATE + ($5::int * INTERVAL '1 day')))
ORDER BY license_number ASC
LIMIT $6;
