# OSRM Route Service (dev)

Live tracking resolves a driving route per dispatched (origin, destination)
hub pair and caches it in `route_geometries`. The cached polyline powers the
progress bar on `GET /api/v1/live/vehicles`.

## Demo server

- Default endpoint: `OSRM_URL` (default `http://localhost:5000`).
- The demo OSRM server is **development-only** and rate-limited to **<=1 rps**.
  The backend calls it at most once per hub pair (cache hit afterwards), and
  the simulator never calls it directly.
- Production needs a self-hosted OSRM instance with its own capacity plan.

## Midwest extract prep (dev)

The seed hubs are US cities; the demo profile below builds a Midwest
extract so local routing covers Chicago/Denver demo traffic:

```bash
# Download a Midwest extract, e.g. from Geofabrik (us-midwest) or a
# state extract (illinois, indiana, iowa, ...):
wget https://download.geofabrik.de/north-america/us-midwest-latest.osm.pbf

docker run -v "$PWD:/data" osrm/osrm-backend \
  osrm-extract -p /opt/car.lua /data/us-midwest-latest.osm.pbf
docker run -v "$PWD:/data" osrm/osrm-backend \
  osrm-partition /data/us-midwest-latest.osrm
docker run -v "$PWD:/data" osrm/osrm-backend \
  osrm-customize /data/us-midwest-latest.osrm
```

Run the demo server (see `docker-compose.yml`, `osrm` service):

```bash
docker compose up osrm
# serves http://localhost:5000, DataVersion check:
curl -s localhost:5000/route/v1/driving/-87.6298,41.8781\;-104.9903,39.7392?overview=false
```

## Fallback

When OSRM is unreachable or a hub has no coordinates, dispatch caches a
straight-line 2-point route with `provider='approximate'`. The frontend
renders approximate routes dashed. Routing never fails a dispatch.

## Attribution

Routing data (c) OpenStreetMap contributors, ODbL. OSRM demo server usage
follows the OSRM project usage policy (low volume, dev only).
