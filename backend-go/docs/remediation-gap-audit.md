# Remediation Gap Audit: Phase 4 & Phase 5

Audit performed across `backend-go/` and `frontend/src/` to verify independent code review claims against the codebase prior to remediation.

## Summary of Findings

| Review Item | Status | Evidence (file:line) | Notes |
|---|---|---|---|
| **P5-01** (Metrics route labels raw paths) | **CONFIRMED** | `backend-go/internal/app/metrics.go:74-92`, `app.go:125-141` | Outer request has empty `r.Pattern`; `boundRoute` treats `/api/v1/*` as bounded and returns raw URL path. |
| **P5-02** (Access log logs raw path & license) | **CONFIRMED** | `backend-go/internal/middleware/middleware.go:133-140` | `r.Pattern` empty on outer request; falls back to `r.URL.Path` including driver license and trip ID. |
| **P5-03** (No error_code in access log) | **CONFIRMED** | `backend-go/internal/middleware/middleware.go:143-151` | Only `request_id`, `method`, `route`, `status`, `latency_ms`, `user_id`, `role` logged. `error_code` missing. |
| **P5-04** (15s timeout never writes 503) | **CONFIRMED** | `backend-go/internal/middleware/middleware.go:37-49` | Sets context deadline; does not intercept write or return 503 `REQUEST_TIMEOUT` if handler returns 500. |
| **P5-06** (MinConns not set) | **CONFIRMED** | `backend-go/cmd/server/main.go:32-35` | `MinConns` omitted (defaults to 0); plan requires 2. |
| **P5-07** (Shutdown 10s not 20s, no server timeouts) | **CONFIRMED** | `backend-go/cmd/server/main.go:63-66,81` | Shutdown context timeout is 10s (spec requires 20s); `http.Server` has no `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`. |
| **P4-05** (KPI pending counts Draft and Dispatched) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:148` | `status IN ('Draft', 'Dispatched')`; plan requires Draft only. |
| **P4-06** (KPI date filter applies to Draft/Dispatched) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:159-160` | Filters all status counts with `COALESCE(completed_at, updated_at)`. |
| **P4-07** (type, status, region filters do not reach KPIs) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:84-189,252-253` | Filters only applied to per-vehicle query; headline counts ignore them; region column does not exist. |
| **P4-10** (Distance falls back to planned_distance) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:154-156,219-221` | Falls back to `planned_distance` when odometers are missing or inconsistent. Plan requires actual odometer difference only. |
| **P4-09** (Fuel efficiency mixes populations) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:172-197,297` | Total completed trip distance divided by total expense liters (different trip sets). |
| **P4-11** (KPI errors ignored returning 200 with zeros) | **CONFIRMED** | `backend-go/internal/app/dashboard_service.go:132,139,169,180,189,306` | Query errors swallowed with `_ = err` and row scans with `continue`. |
| **P4-01** (Audit rows missing for user/driver/vehicle mutations) | **CONFIRMED** | `backend-go/internal/app/app.go:644-1295`, `trip_audit.go:10` | Only 9 `recordAuditLogTx` calls exist; missing user create/update/reset, driver state actions, vehicle/driver create/update/delete. |
| **P4-03** (Audit cursor mismatch) | **CONFIRMED** | `backend-go/internal/app/audit_service.go:102,117` | Ordered by `(at DESC, id DESC)` but keyset condition is only `id < $n`. |
| **P4-04** (Audit bad from/to returns 500) | **CONFIRMED** | `backend-go/internal/app/audit_service.go:83-101` | String concatenation with `::TIMESTAMPTZ` throws database syntax error returning 500 instead of 422. |
| **P4-16** (Expense create does not lock trip row) | **CONFIRMED** | `backend-go/internal/app/expense_service.go:139-147` | Queries `trips` without `FOR UPDATE`, allowing race condition with concurrent trip cancellation. |
| **P4-19** (Finance summary is N+1 and ignores errors) | **CONFIRMED** | `backend-go/internal/app/expense_service.go:482-555` | 3 queries executed per vehicle in a loop; errors discarded. |
| **P4-22** (Migration 000004 down deletes all Fuel expenses) | **CONFIRMED** | `backend-go/migrations/000004_fuel_expenses.down.sql:14-27,38` | Deletes all `expenses WHERE type='Fuel'`, losing post-migration standalone fuel expenses. |
| **P4-25** (Maintenance close vehicle.status_changed dead code) | **CONFIRMED** | `backend-go/internal/app/maintenance_service.go:486-491` | `if vStatus == "Available"` checks pre-update status, so event is never published. |
| **P4-27** (Idempotency keyed by email not user ID) | **CONFIRMED** | `backend-go/internal/app/trip_idempotency.go:41-44` | Uses `user.Email` in SQL `user_id` parameter; should be `user.ID`. |
| **P5-09** (State changes with no event publish) | **CONFIRMED** | `backend-go/internal/app/app.go:778-829,1149-1256,644-1440` | Vehicle retire, driver suspend/reinstate/off-duty/available, and CRUD do not publish events. |
| **P5-22** (Raw NOTIFY carries driver_license) | **CONFIRMED** | `backend-go/internal/app/trip_service.go:307,422`, `policy.go:94` | `driver_license` included in NOTIFY event `IDs` map. |
| **P5-14** (Notifications payload has license_number) | **CONFIRMED** | `backend-go/internal/app/jobs.go:103,120` | `license_number` placed in notification payload. |
| **P5-25** (k6 hardcoded credentials and wrong port) | **CONFIRMED** | `scripts/load/steady.js:35-40`, `spike.js:27-32` | Hardcodes `admin@transitops.com` / `Password123!`; port default 8080 instead of 8000. |
| **P5-26** (k6 wrong shape) | **CONFIRMED** | `scripts/load/steady.js:13-15`, `spike.js:11-17` | Steady is 8 min (plan: 10 min); spike is 4x for 1 min (plan: 3x for 2 min). |
| **P5-27** (k6 no dispatch burst) | **CONFIRMED** | `scripts/load/steady.js:92-133` | Only creates draft then cancels; no dispatch burst; single vehicle/driver pair causes 429s. |
| **P5-28** (Frontend ticket URL doubles /api/v1) | **CONFIRMED** | `frontend/src/lib/realtime/useFleetEvents.ts:63-66`, `data.ts:2` | Calls `apiRequest("/api/v1/realtime/ticket")` while `BACKEND_URL` already ends in `/api/v1`. |
| **FE-01** (ROI multiplied by 100 twice) | **CONFIRMED** | `frontend/src/app/dashboard/page.tsx:114,242`, `analytics/page.tsx:333` | Multiplies `v.roi * 100` even though backend already returns percentage. |
| **FE-02** (Every role calls /dashboard/kpis; Driver gets 403) | **CONFIRMED** | `frontend/src/app/dashboard/page.tsx:67` | Unconditionally fetches `/dashboard/kpis`; driver role receives 403; `/dashboard/me` never called. |
| **FE-03** (Expense form lists Draft and Cancelled trips) | **CONFIRMED** | `frontend/src/app/fuel-and-expenses/page.tsx:58-62,100-109` | Unfiltered trip list causes `EXPENSE_ON_INACTIVE_TRIP` (422) when selected. |
| **P5-29** (Backoff and jitter do not match plan) | **CONFIRMED** | `frontend/src/lib/realtime/useFleetEvents.ts:106-117` | Starts at 1.5s with partial jitter; no visibility listener; no refetch debounce. |
| **OpenAPI missing Phase 4 paths** | **CONFIRMED** | `api/openapi.yaml` | Missing `/maintenance/{id}`, `/expenses`, `/finance/summary`, `/dashboard/me`, `/dashboard/export.csv`, `/audit`. |
| **Reports cite absolute paths & false claims** | **CONFIRMED** | `backend-go/docs/phase-4-report.md`, `phase-5-report.md` | Contains `file:///E:/...` and claims single-query CTEs, MinConns: 2, etc. |
| **P5-21** (Safety Officer event policy) | **FALSE** | `backend-go/internal/realtime/policy.go:68-75` | Code correctly denies trip/vehicle events to Safety Officer per plan 5.3. Docs table was wrong. |
| **P5-20 / S-05** (Legacy /ws/fleet route) | **CONFIRMED** | `backend-go/internal/app/app.go:329` | Route still registered. Plan instructs to remove it completely. |
