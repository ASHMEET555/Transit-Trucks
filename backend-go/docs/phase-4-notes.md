# Phase 4 — Maintenance, Finance, Dashboard, Audit: Inventory & Notes

## 1. Inventory of Current Code

### 1.1 Maintenance Endpoints & Handlers
- **Routes (in `backend-go/internal/app/app.go`):**
  - `GET /api/v1/maintenance/` -> `a.maintenance`: Allowed roles: `rbac.AnyAuthenticated`.
  - `POST /api/v1/maintenance/` -> `a.openMaintenance`: Allowed roles: `rbac.RoleFleetManager`, `rbac.RoleAdmin`.
  - `PUT /api/v1/maintenance/{id}/close` -> `a.closeMaintenance`: Allowed roles: `rbac.RoleFleetManager`, `rbac.RoleAdmin`.
- **Existing SQL:**
  - `SELECT * FROM maintenance_logs` (unpaginated, no ordering).
  - `SELECT * FROM vehicles WHERE registration_number=$1` (no `FOR UPDATE` lock).
  - `INSERT INTO maintenance_logs(vehicle_reg,description,cost,start_date,status) VALUES($1,$2,$3,$4,'Open') RETURNING id` (no transaction, no idempotency key check).
  - `UPDATE vehicles SET status='In Shop' WHERE registration_number=$1`.
  - `SELECT * FROM maintenance_logs WHERE id=$1`.
  - `UPDATE maintenance_logs SET status='Closed',end_date=$1 WHERE id=$2`.
  - `UPDATE vehicles SET status='Available' WHERE registration_number=$1` (unconditional update, no check for remaining open logs or Retired state).
- **Gaps against Spec:**
  - Target paths: `GET /api/v1/maintenance` and `GET /api/v1/maintenance/{id}` must be accessible only to `Admin`, `Fleet Manager`, `Safety Officer` (Financial Analyst and Driver: 403).
  - `POST /api/v1/maintenance` requires `Idempotency-Key` header and strict lock order (`Vehicle` -> `Maintenance Log`).
  - `POST /api/v1/maintenance/{id}/close` requires `Idempotency-Key` header, lock order, checks if vehicle was `Retired` (must remain `Retired`), and checks if another `Open` log exists before setting `Available`.
  - Date validations: `start_date` must not be $> 1$ day in future (UTC); `end_date` must be $\ge$ `start_date`.
  - Description max 500 chars, cost `NUMERIC(12,2) >= 0`.
  - Audit logging inside the same transaction.

### 1.2 Finance & Expenses Endpoints & Handlers
- **Routes (in `backend-go/internal/app/app.go`):**
  - `POST /api/v1/finance/expenses` -> `a.expense`: Allowed roles: `rbac.RoleFinancialAnalyst`, `rbac.RoleFleetManager`, `rbac.RoleAdmin`.
  - `GET /api/v1/finance/expenses/{id}` -> `a.expenses`: Allowed roles: `rbac.AnyAuthenticated`.
- **Existing SQL:**
  - `SELECT count(*) FROM trips WHERE id=$1`.
  - `INSERT INTO expenses(trip_id,type,liters,cost,date) VALUES($1,$2,$3,$4,$5) RETURNING id`.
  - `SELECT * FROM expenses WHERE trip_id=$1`.
- **Gaps against Spec:**
  - Target routes:
    - `POST /api/v1/expenses` (Idempotency-Key): `Fleet Manager`, `Financial Analyst`, `Admin`. (Keep `/api/v1/finance/expenses` alias).
    - `GET /api/v1/expenses`: filters (`trip_id`, `vehicle_reg`, `type`, `from`, `to`), cursor pagination, limit clamped to 100. Roles: `Fleet Manager`, `Financial Analyst`, `Admin`. Driver and Safety Officer: 403.
    - `GET /api/v1/finance/summary`: per-vehicle totals (fuel, maintenance expenses, other expenses, closed maintenance cost, distance in range, cost per km). Roles: `Financial Analyst`, `Fleet Manager`, `Admin`.
  - Trip status validation: Expense allowed ONLY on `Dispatched` or `Completed` trips. Draft or Cancelled must return 422 with code `EXPENSE_ON_INACTIVE_TRIP`.
  - Amount: NUMERIC(12,2) $\ge 0$. Liters required and $> 0$ when type is `Fuel`, must be null otherwise.
  - Migration: migrate Phase 3 `fuel_liters` and `fuel_cost` from `trips` table into `expenses` rows, then drop the trip columns.

### 1.3 Dashboard Endpoints & Handlers
- **Routes (in `backend-go/internal/app/app.go`):**
  - `GET /api/v1/dashboard/kpis` -> `a.kpis`: Allowed roles: `rbac.AnyAuthenticated`.
- **Existing SQL & Loop Logic:**
  - Fetches all vehicles, all drivers, all trips, all maintenance logs into memory.
  - Nested loops: for each vehicle, loops all trips; inside trip loop, runs `SELECT * FROM expenses WHERE trip_id=$1` (P-01 N+1 bug).
  - Sums revenue and distance across ALL trips regardless of status (D-09 bug).
  - Uses `t.PlannedDistance` instead of `final_odometer - start_odometer` (D-09 bug).
  - Sums all maintenance logs, even `Open` ones (D-09 bug).
  - No role minimization: anyone authenticated can see all analytics (S-11 bug).
- **Gaps against Spec:**
  - Replace in-memory loops with single/few SQL aggregate queries.
  - Implement exact KPI definitions per `docs/KPI_DEFINITIONS.md`.
  - Accessible to `Admin`, `Fleet Manager`, `Financial Analyst`, `Safety Officer`.
  - No driver PII in response.
  - New route `GET /api/v1/dashboard/me` for `Driver` role (own trips count by status, own completed distance, own last 20 trips; no fleet financials).
  - New route `GET /api/v1/dashboard/export.csv`: streams completed trips row-by-row (pgx cursor + http flushing; no full buffering; formula injection protection).

### 1.4 Audit Log Reader
- **Current State:**
  - `audit_log` table exists and rows are inserted by trip and auth operations.
  - No endpoint exists to read audit logs.
- **Gaps against Spec:**
  - New route `GET /api/v1/audit`: Admin only. Filters: `entity`, `entity_id`, `actor_user_id`, `from`, `to`, `cursor`, `limit` (max 100). Deterministic order (`at DESC, id DESC`).
  - Audit logging for all Phase 4 actions (maintenance open/close, expense create).

---

## 2. Frontend Inventory

- `frontend/src/app/maintenance/page.tsx`:
  - Calls `GET /maintenance/` -> expects array of maintenance logs.
  - Calls `POST /maintenance/` -> body `{ vehicle_reg, description, cost, start_date }`.
  - Calls `PUT /maintenance/${id}/close` -> body `{ end_date }`.
- `frontend/src/app/fuel-and-expenses/page.tsx`:
  - Calls `GET /finance/expenses/${id}` -> expects expenses for trip id.
  - Calls `POST /finance/expenses` -> body `{ trip_id, type, cost, liters, date }`.
- `frontend/src/app/dashboard/page.tsx` & `frontend/src/app/analytics/page.tsx`:
  - Calls `GET /dashboard/kpis` -> expects `DashboardKPIs` response.
- `frontend/src/lib/rbacConfig.ts`:
  - Enforces route permissions client-side (`dashboard`, `maintenance`).

---

## 3. Known Bugs Identified for Phase 4

1. **D-02: Retired Vehicle Un-retires via Maintenance**:
   - `openMaintenance` allows a Retired vehicle to enter maintenance (`status = 'In Shop'`).
   - `closeMaintenance` resets vehicle status to `Available`.
   - **Fix:** Reject open maintenance on Retired vehicles (`409 VEHICLE_RETIRED`). If a vehicle was retired mid-maintenance, `closeMaintenance` leaves it `Retired`.
2. **D-02: Closing One of Multiple Open Logs Sets Vehicle Available**:
   - `closeMaintenance` unconditionally updates vehicle status to `Available`.
   - **Fix:** Check `COUNT(*) FROM maintenance_logs WHERE vehicle_reg = $1 AND status = 'Open' AND id != $2`. Set `Available` only if count is 0 and vehicle is not Retired.
3. **D-09: Dashboard Counts Cancelled & Draft Trips in Revenue & Distance**:
   - `kpis` sums `revenue` and `distance` across all trips in `ts`.
   - **Fix:** Filter strictly by `status = 'Completed'`.
4. **D-09: Dashboard Uses Planned Distance Instead of Odometer Difference**:
   - `kpis` adds `t.PlannedDistance`.
   - **Fix:** Compute `final_odometer - start_odometer` for completed trips.
5. **P-01: Dashboard N+1 Query Loop**:
   - `kpis` runs `SELECT * FROM expenses WHERE trip_id=$1` inside a per-vehicle and per-trip loop.
   - **Fix:** Single SQL aggregate query with `LEFT JOIN expenses` and `GROUP BY`.
6. **S-11: Dashboard Missing Data Minimization & Role Matrix**:
   - Drivers can view fleet financials via `/dashboard/kpis`.
   - **Fix:** Restrict `/dashboard/kpis` to management roles; provide `/dashboard/me` for drivers.
