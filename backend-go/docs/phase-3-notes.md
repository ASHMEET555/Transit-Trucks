# Phase 3 Inventory & Analysis Notes: Trip Dispatch Core

## 1. Current Trip Code Inventory

| Endpoint | Method | Role Set | Current Handler | Current SQL / Repository Operations |
|---|---|---|---|---|
| `/api/v1/trips/` | `GET` | Any authenticated | `trips` | `SELECT * FROM trips` (unpaginated, unbuffered) |
| `/api/v1/trips/suggest` | `POST` | `Fleet Manager`, `Driver`, `Admin` | `suggest` | `SELECT * FROM vehicles WHERE status='Available' AND max_load_capacity >= $1`<br/>`SELECT * FROM drivers WHERE status='Available' AND license_expiry_date > $1`<br/>In-memory nested iteration, single best pair |
| `/api/v1/trips/` | `POST` | `Fleet Manager`, `Driver`, `Admin` | `dispatch` | Non-atomic check-then-write:<br/>1. `SELECT * FROM vehicles WHERE registration_number=$1`<br/>2. `SELECT * FROM drivers WHERE license_number=$1`<br/>3. `INSERT INTO trips(...) RETURNING id`<br/>4. `UPDATE vehicles SET status='On Trip'`<br/>5. `UPDATE drivers SET status='On Trip'` |
| `/api/v1/trips/{id}/complete` | `PUT` | `Fleet Manager`, `Driver`, `Admin` | `complete` | 1. `SELECT * FROM trips WHERE id=$1`<br/>2. Reads `final_odometer` from URL query parameter<br/>3. `SELECT * FROM vehicles WHERE registration_number=$1`<br/>4. `UPDATE trips SET status='Completed'`<br/>5. `UPDATE vehicles SET odometer=$1, status='Available'`<br/>6. `UPDATE drivers SET status='Available'` |
| `/api/v1/trips/{id}/cancel` | `PUT` | `Fleet Manager`, `Driver`, `Admin` | `cancel` | 1. `SELECT * FROM trips WHERE id=$1`<br/>2. `UPDATE trips SET status='Cancelled'`<br/>3. `UPDATE vehicles SET status='Available'`<br/>4. `UPDATE drivers SET status='Available'` |

---

## 2. Identified Bugs & Vulnerabilities in Current Logic

1. **Check-Then-Write Dispatch Race Condition**:
   - `dispatch` reads vehicle and driver without `FOR UPDATE`.
   - Concurrent requests pass vehicle/driver availability checks simultaneously.
   - When multiple requests attempt to dispatch the same vehicle or driver, the second either throws unhandled 500 or produces inconsistent state instead of a clean `409` conflict with `TRIP_VEHICLE_UNAVAILABLE` or `TRIP_DRIVER_UNAVAILABLE`.
2. **Non-Atomic Multi-Statement Writes**:
   - `INSERT INTO trips`, `UPDATE vehicles`, and `UPDATE drivers` run as separate non-transactional database operations.
   - If an update fails, a network hiccup occurs, or the server halts, the trip is created but resources remain `Available`, or vice-versa.
   - Same issue applies to `complete` and `cancel`.
3. **No Driver Resource Ownership Check (S-04)**:
   - Any user with the `Driver` role can call `complete` or `cancel` on ANY trip in the database.
   - There is no verification that the authenticated user's `driver_license` matches `trips.driver_license`.
4. **No Draft Trip State**:
   - `POST /api/v1/trips` immediately forces `status = "Dispatched"` and immediately locks vehicle and driver into `On Trip`.
   - Users cannot plan or prepare a `Draft` trip without prematurely seizing real fleet resources.
5. **Suggest Endpoint Deficiencies**:
   - Returns only a single pair `bv`/`bd` or 404 if no vehicles match.
   - Ineligible candidates are silently dropped without reason codes.
   - Scoring logic is hardcoded inside handler rather than isolated in a testable pure function.
6. **Missing Idempotency**:
   - No `Idempotency-Key` validation on trip mutation endpoints.
   - Network retries from frontend or mobile clients cause duplicate dispatches, double completion, or 400/409 errors.
7. **Missing Mileage & Expense Tracking Columns**:
   - `trips` table lacks `start_odometer`, `final_odometer`, `fuel_liters`, and `fuel_cost`.
   - Completion handler expects odometer in URL query param (`?final_odometer=...`) rather than JSON body.

---

## 3. Frontend Trip Calls & Expectations

| Frontend File | Request | Expected Response / Behavior |
|---|---|---|
| `frontend/src/app/trips/page.tsx` | `GET /trips/` | List of trips. Currently extracted using `extractItems` from Phase 2. |
| `frontend/src/app/trips/page.tsx` | `POST /trips/` | Creates trip. Expected to support draft creation or atomic create+dispatch. |
| `frontend/src/app/trips/page.tsx` | `POST /trips/suggest` | Returns `{ suggested_vehicle_reg, suggested_driver_license, heuristic_score, ... }`. |
| `frontend/src/app/trips/page.tsx` | `PUT /trips/{id}/complete?final_odometer={val}` | Marks trip completed, restores vehicle & driver. |
| `frontend/src/app/trips/page.tsx` | `PUT /trips/{id}/cancel` | Marks trip cancelled, restores vehicle & driver. |
| `frontend/src/app/dashboard/page.tsx` | `GET /trips/` | Summary counts of active trips. |
| `frontend/src/app/analytics/page.tsx` | `GET /trips/` | Route and cargo aggregations. |
| `frontend/src/app/live-map/page.tsx` | `GET /trips/` | Filters `status === 'Dispatched'` for map rendering. |
