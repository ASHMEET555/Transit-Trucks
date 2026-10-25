-- name: GetTrip :one
SELECT * FROM trips
WHERE id = $1 LIMIT 1;

-- name: GetTripForUpdate :one
SELECT * FROM trips
WHERE id = $1 FOR UPDATE;

-- name: CreateTrip :one
INSERT INTO trips (
    source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: DispatchTrip :one
UPDATE trips
SET
    status = 'Dispatched',
    start_odometer = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CompleteTrip :one
UPDATE trips
SET
    status = 'Completed',
    final_odometer = $2,
    fuel_liters = $3,
    fuel_cost = $4,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CancelTrip :one
UPDATE trips
SET
    status = 'Cancelled',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RecordAuditLog :exec
INSERT INTO audit_log (
    entity, entity_id, action, actor_email, details, at
) VALUES (
    $1, $2, $3, $4, $5, now()
);

-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE user_id = $1 AND key = $2;

-- name: GetIdempotencyKeyForUpdate :one
SELECT * FROM idempotency_keys
WHERE user_id = $1 AND key = $2 FOR UPDATE;

-- name: CreateIdempotencyKey :exec
INSERT INTO idempotency_keys (
    key, user_id, route, request_hash, status_code, response_body, created_at, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, now(), now() + INTERVAL '24 hours'
);
