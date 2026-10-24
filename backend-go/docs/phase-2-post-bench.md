# Phase 2 Post-Implementation Latency Benchmark

Measured 100 iterations per endpoint against Seed v2 PostgreSQL database.

| Endpoint | p50 (ms) | p95 (ms) | p99 (ms) | Min (ms) | Max (ms) |
|---|---|---|---|---|---|
| `/api/v1/fleet/vehicles` | 3.26 | 3.92 | 5.36 | 1.94 | 5.36 |
| `/api/v1/fleet/drivers` | 0.00 | 3.40 | 4.23 | 0.00 | 4.23 |
| `/api/v1/destinations/` | 0.00 | 0.00 | 0.54 | 0.00 | 0.54 |
| `/api/v1/trips/` | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
| `/api/v1/vehicles` | 0.00 | 0.00 | 0.52 | 0.00 | 0.52 |
| `/api/v1/drivers` | 0.00 | 0.00 | 0.53 | 0.00 | 0.53 |
| `/api/v1/destinations` | 0.00 | 0.00 | 0.00 | 0.00 | 0.00 |
