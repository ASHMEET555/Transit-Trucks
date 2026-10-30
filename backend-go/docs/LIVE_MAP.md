# Live Map: Estimated Positions (No GPS)

TransitOps has no GPS or telemetry source. The live map shows Dispatched trips
positioned by a documented estimate, never a measured location.

## Formula

```
elapsed_h = (now - dispatched_at) / 1h
estimated_progress = clamp(elapsed_h * 80 / planned_distance_km, 0, 1)
```

- Constant: `AssumedAverageSpeedKMH = 80` (`live_trips.go`).
- `dispatched_at` set at dispatch (`trip_service.go`); pre-migration rows backfilled
  from `updated_at` (migration `000006`).
- `planned_distance <= 0` or future timestamp → `0`.
- Result rounded to 4 decimals.

## Map rendering

- Marker = linear interpolation origin → destination by `estimated_progress`.
- Straight line only. Labeled "Estimated position, no GPS" on marker and tooltip.
- Trips whose source or destination lacks coordinates are excluded from
  `GET /api/v1/trips/live` but remain in trip lists.
- Client animates (glides) markers between refetches; server values are the truth.

## Role scope (`GET /api/v1/trips/live`)

Driver: own trips only. Safety Officer / Financial Analyst: no `driver_license`.
Operators (Admin, Fleet Manager): all. Payload never contains driver names.
