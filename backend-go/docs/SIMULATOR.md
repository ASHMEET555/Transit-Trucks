# Vehicle Simulator (dev)

`cmd/simulator` drives virtual trucks along cached routes so the live board
has movement without hardware.

## Approach

- **Reads** go straight to Postgres: every 10s it polls Dispatched trips
  plus the `route_geometries` cache (`pollDispatched`). Trucks without a
  cached geometry are skipped until dispatch computes one.
- **Writes** go only through the public API: each tick posts to
  `POST /api/v1/telemetry/positions` with `source=simulator`, exactly like
  any other client. There is no backdoor ingest path.
- **Auth** is a service login: on start (and on 401) it logs in with the
  seed admin credentials from `SIM_ADMIN_EMAIL` / `SIM_ADMIN_PASSWORD`
  (default `admin@transitops.com`, password from `SIM_ADMIN_PASSWORD`) and
  sends the session as a `Bearer` token. The server accepts
  simulator-source reports only when `SIM_ENABLED=1`.
- It **never completes trips**: arrived trucks dwell at the destination
  (15 sim-minutes) and hold position indefinitely.

## Physics (1s tick x SIM_TIME_SCALE, default 20)

- Speed limit = route average speed (cached distance/duration, else 80 km/h).
- Curves: sharpest turn >25 deg within the next 80 m caps speed at 0.6x.
- Acceleration <= 1.5 m/s^2, braking <= 2.5 m/s^2.
- Stops exactly at the destination, then dwells 15 sim-minutes.
- `SIM_SEED` gives deterministic per-vehicle start offsets (0-500 m) only.

## Run (dev only)

```bash
# server must accept simulator ingest:
SIM_ENABLED=1 docker compose --profile sim up simulator
# or locally:
SIM_ADMIN_PASSWORD='...' BACKEND_URL=http://localhost:8000 \
  POSTGRES_DSN=postgresql://transitops:transitops@localhost:5432/transitops \
  SIM_ENABLED=1 go run ./cmd/simulator
```

Env: `BACKEND_URL` (default `http://localhost:8000`), `SIM_TIME_SCALE`
(default 20), `SIM_SEED` (default 0), `OSRM_URL` is server-side only.
