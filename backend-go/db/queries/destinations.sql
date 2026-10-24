-- name: GetDestination :one
SELECT * FROM destinations
WHERE name = $1 LIMIT 1;

-- name: CreateDestination :one
INSERT INTO destinations (
    name, region
) VALUES (
    $1, $2
)
RETURNING *;

-- name: DeleteDestination :exec
DELETE FROM destinations
WHERE name = $1;

-- name: CountTripsForDestination :one
SELECT COUNT(*) FROM trips
WHERE source_name = $1 OR destination_name = $1;

-- name: ListDestinations :many
SELECT * FROM destinations
WHERE
    ($1::text = '' OR name > $1)
    AND ($2::text = '' OR (name ILIKE '%' || $2 || '%' OR region ILIKE '%' || $2 || '%'))
ORDER BY name ASC
LIMIT $3;
