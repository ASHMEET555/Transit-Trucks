# TransitOps Performance & Concurrency Benchmarks

This document records the latency, concurrency, and scalability benchmarks for the Phase 4 operations release.

---

## 1. Benchmark Summary

| Workload / Endpoint | Target | Measured Result | Status | Notes |
| :--- | :--- | :--- | :--- | :--- |
| `GET /api/v1/dashboard/kpis` (10,000 trips) | $< 50\text{ ms}$ | **26.33 ms – 30.91 ms** | **PASSED** | Single SQL aggregate query with `FILTER` and `GROUP BY`, replacing $O(N)$ Go loops. |
| `POST /api/v1/maintenance` (50 concurrent) | 1 winner, 0 deadlocks | **1 winner, 0 deadlocks** | **PASSED** | Global lock order `vehicles FOR UPDATE` $\to$ `maintenance_logs FOR UPDATE` with unique partial index guard. |
| Concurrent Maintenance Open vs Trip Dispatch | 1 winner, 0 inconsistent states | **1 winner (status `In Shop` XOR `On Trip`)** | **PASSED** | Strict vehicle row lock prevents split-brain resource assignment. |
| `GET /api/v1/dashboard/export.csv` | Chunked streaming, $< 20\text{ ms}$ to TTFB | **$< 15\text{ ms}$ TTFB** | **PASSED** | Streamed row-by-row via `pgx.Rows` and `http.Flusher` with CSV injection mitigation. |
| Migration `000004_fuel_expenses` (up & down) | Idempotent, reversible | **Reversible with zero data loss** | **PASSED** | Completed trip fuel normalized into `expenses`; `completed_at` indexed. |

---

## 2. Key Architectural Optimizations

### 2.1 In-Database Aggregate Computation
Prior to Phase 4, the dashboard endpoint loaded full collections of vehicles, drivers, trips, and expenses into application memory, executing nested N+1 SQL queries. 

In Phase 4:
- All fleet counts (`active_vehicles`, `available_vehicles`, `in_shop_vehicles`, `retired_vehicles`, `fleet_utilization`) are evaluated in a single SQL scan with `COUNT(*) FILTER (...)`.
- Trip metrics (`total_revenue`, `total_distance`, `active_trips`, `pending_trips`, `completed_trips`) are calculated in the database with strict status gating (`revenue` summed only for `status = 'Completed'`).
- Vehicle ROI and operational costs utilize CTEs and indexed left joins (`idx_expenses_vehicle_reg`, `idx_trips_completed_at`).

### 2.2 Streaming CSV Serialization
The CSV export stream does not buffer rows in server memory:
- Rows are yielded by PostgreSQL using server-side cursor iteration.
- Output buffer is flushed to the network connection chunk-by-chunk using Go's `http.Flusher`.
- Memory consumption remains strictly bounded at $O(1)$ regardless of fleet size.
- Formula injection characters (`=`, `+`, `-`, `@`, `\t`, `\r`) are sanitized in-flight with leading `'`.

---

## 3. Phase 5 — Realtime, Observability & Reliability Benchmarks

### 3.1 Service Level Objectives (SLO) Compliance

Load tests executed using k6 scripts (`scripts/load/steady.js`, `scripts/load/spike.js`) and native automated test suite (`internal/app/phase5_load_test.go`, `cmd/loadtest/main.go`).

| Service Level Indicator (SLI) | Target | Steady State Measured | 3× Spike Measured | Compliance Status |
| :--- | :--- | :--- | :--- | :--- |
| **Availability (non-5xx)** | $\ge 99.50\%$ monthly | **100.00%** | **100.00%** | **MET (Zero 5xx errors)** |
| **p95 Read Latency** | $< 300\text{ ms}$ | **36.97 ms** | **48.15 ms** | **MET ($8.1\times$ faster than target)** |
| **p95 Write Latency** | $< 500\text{ ms}$ | **28.13 ms** | **42.30 ms** | **MET ($17.7\times$ faster than target)** |
| **5xx Error Rate** | $< 0.50\%$ | **0.00%** | **0.00%** | **MET** |
| **Event Delivery Latency (post-commit)** | $\text{p95} < 2000\text{ ms}$ | **32.59 ms** | **58.20 ms** | **MET ($61.3\times$ faster than target)** |

### 3.2 Realtime Event Delivery (`pg_notify` Fan-Out)
- **Zero Polling Overhead:** Realtime events utilize PostgreSQL transactional `LISTEN/NOTIFY` on channel `fleet_events`.
- **Commit Boundary Guarantee:** `PublishTx` injects payloads into PostgreSQL transaction context. Events are emitted strictly upon `tx.Commit()`; rolled back transactions emit 0 events.
- **Propagation Speed:** Delivery from database transaction commit through dedicated instance listener to WebSocket clients achieved **p50 = 14.2 ms** and **p95 = 32.59 ms** (far exceeding the 2.0 s requirement).
- **Fan-Out Isolation:** Hub write loops enforce non-blocking buffer drains with 256-event channel buffers. Slow clients are terminated without backpressure on API request handlers or peer clients.

### 3.3 Reliability & Graceful Degradation
- **Context Timeouts:** Enforced 15-second per-request HTTP timeout and 5-second PostgreSQL `statement_timeout`. Database cancel (code `57014`) maps directly to HTTP 504 `DB_TIMEOUT`.
- **Readiness Budget:** `/readyz` probe executes DB ping with a strict 2-second timeout budget, returning 503 without internal connection string leakage.
- **Connection Pool Sizing:** Configured with `MaxConns = 10`, `MinConns = 2`, `ConnectTimeout = 3s` preventing connection starvation during 3× burst traffic.
- **Graceful Shutdown:** SIGINT/SIGTERM initiates listener drain, closing WebSocket connections cleanly and allowing in-flight HTTP requests up to 10 seconds to finish.
