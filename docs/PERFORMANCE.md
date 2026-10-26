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
