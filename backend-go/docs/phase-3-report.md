# Phase 3 — Trip Dispatch Core: Implementation & Verification Report

## 1. Executive Summary
Phase 3 establishes an ACID-compliant, concurrency-hardened trip dispatch engine for TransitOps. It resolves race conditions in resource allocation (vehicles and drivers) through strict global lock ordering, partial PostgreSQL unique indexes, idempotency key guarantees, resource-level authorization for drivers, and automated suggestion scoring.

---

## 2. Product Plan Items Closed
| Plan Item | Description | Resolution Status |
| :--- | :--- | :--- |
| **D-01** | Trip lifecycle states (`Draft`, `Dispatched`, `Completed`, `Cancelled`) | **CLOSED**: Strict state machine implemented; draft trips created with optional atomic dispatch. |
| **D-08** | Driver & Vehicle double booking under concurrent requests | **CLOSED**: Strict lock ordering (`Vehicle` -> `Driver` -> `Trip`) + partial indexes eliminate double bookings. |
| **D-10** | Odometer monotonicity and optional fuel logging on trip completion | **CLOSED**: Handled with PostgreSQL CHECK constraints and handler validations. |
| **S-04** | Resource-level trip authorization | **CLOSED**: Drivers can only view/mutate trips linked to their own license number. |

---

## 3. Concurrency Verification Results

All compulsory concurrency tests were executed against PostgreSQL 16 on port 5432 with `-race` enabled.

| Test Scenario | Rounds | Concurrency / Round | Double Bookings | Deadlocks | Result |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Concurrent Vehicle Dispatch** | 100 | 50 concurrent requests | 0 | 0 | **PASS** |
| **Concurrent Driver Dispatch** | 100 | 50 concurrent requests | 0 | 0 | **PASS** |
| **Simultaneous Dispatch & Complete** | 100 | 20 pairs concurrent | 0 | 0 | **PASS** |
| **Mixed Ops Invariant Stress Test** | 1 | 200 mixed goroutines | 0 | 0 | **PASS** |
| **Reverse Lock Deadlock Test** | 50 | Inverted request orders | 0 | 0 | **PASS** |

### Key Observations:
- **Double Booking Guarantee**: Exactly 1 request succeeds (HTTP 200/201), and 49 requests fail with HTTP 409 (`VEHICLE_NOT_AVAILABLE`, `DRIVER_NOT_AVAILABLE`, or unique constraint fallbacks `TRIP_VEHICLE_UNAVAILABLE`, `TRIP_DRIVER_UNAVAILABLE`).
- **Deadlock Freedom**: Global lock hierarchy (`Vehicle` $\rightarrow$ `Driver` $\rightarrow$ `Trip`) ensures zero deadlock errors (`40P01`) under adversarial load.
- **Data Invariant**: At the conclusion of high-concurrency runs, every `Dispatched` trip has its associated vehicle and driver marked as `On Trip`, and every `Available` resource has zero active dispatched trips.

---

## 4. Architectural Decisions
1. **Driver License Category Mapping**:
   - `Heavy`: All vehicles (`Heavy Truck`, `Medium Truck`, `Light Truck`, `Box Truck`, `Van`).
   - `Medium`: `Medium Truck`, `Light Truck`, `Box Truck`, `Van`.
   - `Van` / `Light`: `Light Truck`, `Van`.
   - Other: Exact match.
2. **Cargo Overload Error Model**:
   - Cargo exceeding vehicle capacity returns HTTP `409 Conflict` with code `TRIP_OVERLOADED`.
3. **Dispatch Date Definition**:
   - Evaluated using current UTC date (`time.Now().UTC()`) against `drivers.license_expiry_date`.
4. **Idempotency Replay & In-Flight Protection**:
   - Idempotency keys stored with 24-hour expiration in `idempotency_keys`. Replays return cached response body with `X-Idempotent-Replay: true`.
   - Concurrent submissions with the same key are protected by `(user_id, key)` unique constraint and return `409 IDEMPOTENCY_IN_PROGRESS`.
   - Mismatched route or payload hash returns `422 IDEMPOTENCY_KEY_REUSED`.
5. **Fuel Storage**:
   - `fuel_liters` and `fuel_cost` stored directly in `trips` table with mutual dependency checks: both must be present and $>0$ if provided.
6. **Suggestion Scoring**:
   - Deterministic 100-point heuristic prioritizing capacity utilization, low vehicle odometer wear, license qualifications buffer, and expiration headroom. Ineligible candidates are clearly itemized with cause.

---

## 5. Frontend Adaptations
- **Files Modified**:
  - `frontend/src/lib/api.ts`: Enhanced `ApiError` with `code` and `requestId`; merged custom headers; extracted unified error envelope codes.
  - `frontend/src/lib/ProfileContext.tsx`: Added `driver_license` to `Profile` for client-side permission scoping.
  - `frontend/src/app/trips/page.tsx`:
    - Added "Dispatch immediately" toggle checkbox (`dispatch: boolean`).
    - Integrated client-side `Idempotency-Key` generation via UUID with persistent reuse on retry.
    - Added dispatch and cancel actions on `Draft` trips.
    - Updated completion modal with inputs for `final_odometer`, `fuel_liters`, and `fuel_cost`.
    - Resource-level action gating allowing drivers to complete/dispatch their own trips.
- **Frontend Verification**:
  - `npm test`: All RBAC tests passed (5/5).
  - `npm run build`: Production Next.js build compiled with 0 TypeScript/lint errors.

---

## 6. Phase 3 Commits
- `2796ee9` — `test(3.0): inventory and failing tests for known trip bugs`
- `feat(3.1): draft trip lifecycle and create-and-dispatch`
- `feat(3.2): transactional lock order and unique-index error mapping`
- `feat(3.3): dispatch rules with stable error codes`
- `feat(3.4): complete with odometer and fuel fields`
- `feat(3.5): cancel transitions and resource release`
- `feat(3.6): idempotency keys with replay and in-flight protection`
- `feat(3.7): resource-level trip authorization`
- `feat(3.8): suggest endpoint with pure scoring and reasons`
- `feat(3.9): trip audit rows and phase 5 hook points`
- `test(phase-3): compulsory concurrency and lifecycle test suite`
- `feat(frontend): trip dispatch toggle, idempotency keys, and fuel inputs`
- `docs(phase-3): architecture updates and phase-3 completion report`
