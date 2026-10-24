# Phase 2 Baseline Latency Benchmark

Measured 100 iterations per endpoint against PostgreSQL seed.

| Endpoint | p50 (ms) | p95 (ms) | p99 (ms) | Min (ms) | Max (ms) |
|---|---|---|---|---|---|
| `/api/v1/fleet/vehicles` | 4.11 | 5.77 | 6.88 | 2.08 | 6.88 |
| `/api/v1/fleet/drivers` | 0.00 | 4.13 | 5.66 | 0.00 | 5.66 |
| `/api/v1/destinations/` | 0.00 | 0.00 | 0.53 | 0.00 | 0.53 |
| `/api/v1/trips/` | 0.00 | 0.00 | 0.55 | 0.00 | 0.55 |
