# Live-map source model

## What is simulated now

- Truck positions come from `cmd/simulator`, a demo source only. It polls
  Dispatched trips plus the `route_geometries` cache from Postgres and posts
  to `POST /api/v1/telemetry/positions` with `source=simulator` — the exact
  same HTTP endpoint and the exact same `IngestPosition` service path that a
  real device will use. It never writes to the database directly.
- The UI labels simulator data **"Simulated"** (fresh: "Simulated · updated
  Ns ago"). Simulator data is never labelled live.

## What is not tracked

- No browser geolocation, no driver-phone location sharing, no GPS UI.
- No on-time delivery or idle-hours tracking: the backend does not compute
  them, so the overview does not show them. Active vehicles, trips, costs
  and maintenance shown are backend-computed only.
- The simulator never completes trips: trucks dwell 15 sim-minutes at the
  destination and hold position.

## What is ready for a real source

- `vehicle_positions.source` accepts `'simulator' | 'device'`; the
  48-hour retention, `(vehicle_reg, recorded_at)` dedupe and validation
  bounds already apply to every source.
- `IngestPosition` accepts device reports today when called with the
  device flag enabled (covered by service tests).
- The UI labels any non-simulator source **"Live"** (fresh) or **"Stale"**,
  from the `source` + `age_seconds` returned by `GET /api/v1/live/vehicles`.

## Exact steps to enable a real source later

1. Build device authentication (not built yet): authenticate the telematics
   device or its gateway and mint a session/token the same way the
   simulator logs in with `SIM_ADMIN_EMAIL` / `SIM_ADMIN_PASSWORD` today.
2. Keep the `source` value `'device'` in the POST body to
   `/api/v1/telemetry/positions`. No schema or API change is needed.
3. Flip the server gate: change the `source == "device"` branch in
   `handlePostPosition` (currently `403 DEVICE_INGEST_NOT_ENABLED`) to
   accept authenticated device sessions — a configuration and auth change
   only. `SIM_ENABLED` stays for the demo simulator.
4. The live board, status pill ("Live · updated …"), routes and progress
   work unchanged: they key off `source`, not off which client posted.

## Physics parameters and time scale

Tick 1 s real time, sim time = real time × `SIM_TIME_SCALE` (default 20).
Speed limit = route average speed (`distance_m / duration_s`, else
80 km/h), reduced to 60% where heading change over the next 80 m exceeds
25°. Acceleration ≤ 1.5 m/s², braking ≤ 2.5 m/s². Stop exactly at the
destination, dwell 15 simulated minutes, hold forever. `SIM_SEED` sets
deterministic per-vehicle start offsets (0–500 m) only.

## OSRM setup and attribution

- `OSRM_URL` (default `http://localhost:5000`), dev demo server ≤ 1 rps,
  never production. Prepare a Midwest extract per
  `backend-go/docs/OSRM.md`; `docker-compose.yml` provides `osrm` +
  env-gated `simulator` services (`SIM_ENABLED=1` to run).
- Routing data © OpenStreetMap contributors, ODbL. The map tiles carry the
  OSM attribution link; approximate (straight-line fallback) routes render
  dashed and are never presented as road routes.
