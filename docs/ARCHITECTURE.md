# TransitOps System Architecture

## Overview
TransitOps is a fleet and trip management platform built with a high-concurrency Go backend (`backend-go/`) and a Next.js 15 TypeScript frontend (`frontend/`). This document defines the domain rules, concurrency guarantees, and lifecycle constraints for the trip dispatch engine.

---

## 1. Driver License & Vehicle Compatibility Matrix
TransitOps enforces driver licensing rules at dispatch time. A driver's license category must qualify them to operate the assigned vehicle type:

| Driver License Category | Permitted Vehicle Types |
| :--- | :--- |
| **Heavy** | Heavy Truck, Medium Truck, Light Truck, Box Truck, Van (All types) |
| **Medium** | Medium Truck, Light Truck, Box Truck, Van |
| **Van** / **Light** | Light Truck, Van |
| *Other* | Strict case-insensitive string equality with vehicle type |

If a driver's category is insufficient for the vehicle type, dispatch fails with HTTP `409 Conflict` and error code `LICENSE_CATEGORY_MISMATCH`.

---

## 2. Cargo Capacity & Overload Rejection
- When creating or dispatching a trip, the cargo weight must not exceed the vehicle's `max_load_capacity`.
- Overloaded assignments are rejected with HTTP `409 Conflict` and error code `TRIP_OVERLOADED`.
- Cargo weight must be strictly positive (`cargo_weight > 0`), otherwise HTTP `422 Unprocessable Entity` (`VALIDATION_FAILED`) is returned.

---

## 3. Dispatch Date & Driver Expiry
- Driver license expiry is verified against the dispatch instant in UTC (`time.Now().UTC()`).
- If `driver.license_expiry_date <= currentDateUTC`, dispatch fails with HTTP `409 Conflict` and error code `DRIVER_LICENSE_EXPIRED`.
- Licenses expiring on future dates remain valid until midnight UTC of their expiration date.

---

## 4. Idempotency Key Semantics
TransitOps provides at-least-once safety with exact-once execution semantics via the `Idempotency-Key` HTTP header.

1. **Storage & Scope**: Keys are stored in the `idempotency_keys` table scoped by `user_id` and `key`, along with the request route, SHA-256 payload hash, HTTP status code, and serialized response body.
2. **Replay**: When a request arrives with an existing key within 24 hours:
   - If route differs or payload hash differs: returns HTTP `422 Unprocessable Entity` with `IDEMPOTENCY_KEY_REUSED`.
   - If route and payload match: immediately replays the cached status code and body with header `X-Idempotent-Replay: true`.
3. **In-Flight Protection**: The unique constraint on `(user_id, key)` prevents concurrent overlapping executions of the same key. A second concurrent request fails with HTTP `409 Conflict` and `IDEMPOTENCY_IN_PROGRESS`.
4. **Retention**: Keys expire after 24 hours (`expires_at = now() + INTERVAL '24 hours'`). Expired keys are cleared and re-executed fresh.

---

## 5. Global Lock Order & Concurrency Control
To prevent database deadlocks across high-concurrency dispatch, complete, and cancel operations, TransitOps enforces a strict global acquisition order:

$$\text{Vehicle} \rightarrow \text{Driver} \rightarrow \text{Trip}$$

- Resources are always locked via `SELECT ... FOR UPDATE` in this order.
- Within identical resource types, locks are ordered by primary key ascending.
- If a route begins with a `trip_id`, the handler looks up `vehicle_reg` and `driver_license` first without holding locks, then acquires locks in the global order.
- Partial unique indexes (`idx_trips_vehicle_dispatched`, `idx_trips_driver_dispatched`) act as secondary safety nets: any race condition violation is captured and mapped to stable error codes `TRIP_VEHICLE_UNAVAILABLE` or `TRIP_DRIVER_UNAVAILABLE`.

---

## 6. Fuel & Odometer Logging on Trip Completion
- When completing a trip (`POST /api/v1/trips/{id}/complete`):
  - `final_odometer` is mandatory and must be $\ge$ the vehicle's `start_odometer`.
  - The vehicle's master odometer is updated to `final_odometer`.
- Fuel logging is optional, but `fuel_liters` and `fuel_cost` must both be provided together and both $> 0$. Partial fuel specifications are rejected with HTTP `422 Unprocessable Entity` (`VALIDATION_FAILED`).
- CHECK constraints in PostgreSQL (`check_trips_odometer`, `check_trips_fuel`) enforce database-level integrity.

---

## 7. Assignment Suggestion Scoring Formula
The endpoint `POST /api/v1/trips/suggest` utilizes a deterministic scoring heuristic:

$$\text{Score} = S_{\text{capacity}} + S_{\text{odometer}} + S_{\text{license}} + S_{\text{headroom}}$$

1. **Capacity Utilization ($0 - 50$ pts)**:
   $$\text{utilization} = \frac{\text{cargo\_weight}}{\text{capacity}}$$
   $$S_{\text{capacity}} = \text{utilization} \times 50$$
   Right-sized vehicles earn higher points; candidates with $\text{utilization} > 1.0$ are classified as ineligible (`VEHICLE_OVERLOADED`).
2. **Odometer & Vehicle Wear ($0 - 20$ pts)**:
   $$S_{\text{odometer}} = \left(\frac{100,000 - \min(\text{odometer}, 100,000)}{100,000}\right) \times 20$$
   Lower odometer readings indicate lower mechanical wear.
3. **License Buffer ($0 \text{ or } 15$ pts)**:
   Drivers with superior qualifications (e.g. `Heavy` category driving `Light Truck`) receive $+15$ pts buffer score.
4. **License Expiry Headroom ($0 - 15$ pts)**:
   $$S_{\text{headroom}} = \min\left(\frac{\text{days\_to\_expiry}}{365}, 1.0\right) \times 15$$
   Ensures drivers with ample license validity are prioritized.
