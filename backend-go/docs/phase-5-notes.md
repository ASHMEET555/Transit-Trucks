# Phase 5 — Realtime, Observability, Reliability: Endpoint Audit & Notes

## 1. Registered Routes Audit Table

| Method | Path | Role Set | Handler File | Error Shape | OpenAPI Presence | Frontend Caller | Notes |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `GET` | `/healthz` | Public | `app.go` | Plain/JSON | Yes (`/healthz`) | Monitoring | Process liveness |
| `GET` | `/readyz` | Public | `app.go` | `{"status":"ready"}` | Yes (`/readyz`) | Monitoring | DB ping check (2s) |
| `POST` | `/api/v1/auth/login` | Public | `app.go` | Standard Error | Yes | `/login` | Rate limited, argon2id |
| `POST` | `/api/v1/auth/logout` | AnyAuthenticated | `app.go` | Standard Error | Yes | AppSidebar | Session revocation |
| `GET` | `/api/v1/auth/me` | AnyAuthenticated | `app.go` | Standard Error | Yes | ProfileContext | Current user profile |
| `POST` | `/api/v1/auth/change-password` | AnyAuthenticated | `app.go` | Standard Error | Yes | `/settings` | Password rotation |
| `GET` | `/api/v1/users` | Admin | `users/handler.go` | Standard Error | Yes | User mgmt | Keyset paginated |
| `POST` | `/api/v1/users` | Admin | `users/handler.go` | Standard Error | Yes | User mgmt | Create user |
| `PATCH` | `/api/v1/users/{id}` | Admin | `users/handler.go` | Standard Error | Yes | User mgmt | Role/Status update |
| `POST` | `/api/v1/users/{id}/reset-password` | Admin | `users/handler.go` | Standard Error | Yes | User mgmt | Admin reset |
| `GET` | `/api/v1/fleet/vehicles`, `/api/v1/vehicles` | AnyAuthenticated | `app.go` | Standard Error | Yes (`/fleet/vehicles`) | `/fleet`, `/trips` | Keyset paginated |
| `POST` | `/api/v1/fleet/vehicles`, `/api/v1/vehicles` | Fleet Manager, Admin | `app.go` | Standard Error | Yes (`/fleet/vehicles`) | `/fleet` | Idempotent |
| `GET` | `/api/v1/fleet/vehicles/{reg}` | AnyAuthenticated | `app.go` | Standard Error | Yes | Drawer | Single vehicle |
| `PATCH`/`PUT` | `/api/v1/fleet/vehicles/{reg}` | Fleet Manager, Admin | `app.go` | Standard Error | Yes | Drawer | Update vehicle |
| `POST` | `/api/v1/fleet/vehicles/{reg}/retire` | Fleet Manager, Admin | `app.go` | Standard Error | Missing in OpenAPI | `/fleet` | Guarded by active trips |
| `DELETE` | `/api/v1/fleet/vehicles/{reg}` | Fleet Manager, Admin | `app.go` | Standard Error | Yes | `/fleet` | Soft/hard delete |
| `GET` | `/api/v1/fleet/drivers`, `/api/v1/drivers` | Admin, Fleet Manager, Safety Officer, Driver | `app.go` | Standard Error | Yes (`/fleet/drivers`) | `/drivers`, `/trips` | Keyset paginated |
| `POST` | `/api/v1/fleet/drivers`, `/api/v1/drivers` | Fleet Manager, Safety Officer, Admin | `app.go` | Standard Error | Yes (`/fleet/drivers`) | `/drivers` | Unique license |
| `GET` | `/api/v1/fleet/drivers/{license}` | Admin, Fleet Manager, Safety Officer, Driver | `app.go` | Standard Error | Yes | Drawer | Single driver |
| `PATCH`/`PUT` | `/api/v1/fleet/drivers/{license}` | Fleet Manager, Safety Officer, Admin | `app.go` | Standard Error | Yes | Drawer | Update driver |
| `POST` | `/api/v1/fleet/drivers/{license}/suspend` | Safety Officer, Admin | `app.go` | Standard Error | Missing in OpenAPI | `/drivers` | Safety action |
| `POST` | `/api/v1/fleet/drivers/{license}/reinstate` | Safety Officer, Admin | `app.go` | Standard Error | Missing in OpenAPI | `/drivers` | Safety action |
| `POST` | `/api/v1/fleet/drivers/{license}/set-off-duty` | Fleet Manager, Admin | `app.go` | Standard Error | Missing in OpenAPI | `/drivers` | Duty action |
| `POST` | `/api/v1/fleet/drivers/{license}/set-available` | Fleet Manager, Admin | `app.go` | Standard Error | Missing in OpenAPI | `/drivers` | Duty action |
| `DELETE` | `/api/v1/fleet/drivers/{license}` | Fleet Manager, Admin | `app.go` | Standard Error | Yes | `/drivers` | Soft/hard delete |
| `GET` | `/api/v1/destinations/`, `/api/v1/destinations` | AnyAuthenticated | `app.go` | Standard Error | Yes | `/trips`, `/live-map` | Hub registry |
| `POST` | `/api/v1/destinations/`, `/api/v1/destinations` | Fleet Manager, Admin | `app.go` | Standard Error | Yes | `/trips` | Add hub |
| `DELETE` | `/api/v1/destinations/{name}` | Fleet Manager, Admin | `app.go` | Standard Error | Yes | Hub removal | Guarded by trips |
| `GET` | `/api/v1/trips`, `/api/v1/trips/` | AnyAuthenticated | `app.go` | Standard Error | Yes (`/trips/`) | `/trips`, `/dashboard` | Paginated |
| `POST` | `/api/v1/trips`, `/api/v1/trips/` | Fleet Manager, Driver, Admin | `trip_service.go` | Standard Error | Yes (`/trips/`) | `/trips` | Draft/Direct dispatch |
| `GET` | `/api/v1/trips/{id}` | AnyAuthenticated | `trip_service.go` | Standard Error | Missing in OpenAPI | `/trips` | S-04 driver isolation |
| `POST` | `/api/v1/trips/{id}/dispatch` | Fleet Manager, Driver, Admin | `trip_service.go` | Standard Error | Missing in OpenAPI | `/trips` | Dispatches draft |
| `POST`/`PUT` | `/api/v1/trips/{id}/complete` | Fleet Manager, Driver, Admin | `trip_complete.go` | Standard Error | Yes | `/trips` | Final odo & fuel |
| `POST`/`PUT` | `/api/v1/trips/{id}/cancel` | Fleet Manager, Driver, Admin | `trip_cancel.go` | Standard Error | Yes | `/trips` | Cancels draft/disp |
| `POST` | `/api/v1/trips/suggest` | Fleet Manager, Driver, Admin | `trip_suggest.go` | Standard Error | Yes | `/trips` | Scored recommendation |
| `GET` | `/api/v1/maintenance`, `/api/v1/maintenance/` | Admin, Fleet Manager, Safety Officer | `maintenance_service.go` | Standard Error | Yes (`/maintenance/`) | `/maintenance` | Work orders |
| `POST` | `/api/v1/maintenance`, `/api/v1/maintenance/` | Fleet Manager, Admin | `maintenance_service.go` | Standard Error | Yes (`/maintenance/`) | `/maintenance` | Lock order enforced |
| `GET` | `/api/v1/maintenance/{id}` | Admin, Fleet Manager, Safety Officer | `maintenance_service.go` | Standard Error | Missing in OpenAPI | Detail drawer | Single log |
| `POST`/`PUT` | `/api/v1/maintenance/{id}/close` | Fleet Manager, Admin | `maintenance_service.go` | Standard Error | Yes | `/maintenance` | Close work order |
| `POST` | `/api/v1/expenses`, `/api/v1/finance/expenses` | Financial Analyst, Fleet Manager, Admin | `expense_service.go` | Standard Error | Yes (`/finance/expenses`) | `/fuel-and-expenses` | Normalized expenses |
| `GET` | `/api/v1/expenses` | Financial Analyst, Fleet Manager, Admin | `expense_service.go` | Standard Error | Missing in OpenAPI | Expense filter | Filtered expense list |
| `GET` | `/api/v1/finance/expenses/{id}` | Financial Analyst, Fleet Manager, Admin | `expense_service.go` | Standard Error | Yes | `/fuel-and-expenses` | Expenses for trip |
| `GET` | `/api/v1/finance/summary` | Financial Analyst, Fleet Manager, Admin | `expense_service.go` | Standard Error | Missing in OpenAPI | `/fuel-and-expenses` | Aggregated costs |
| `GET` | `/api/v1/dashboard/kpis` | Admin, Fleet Manager, Financial Analyst, Safety Officer | `dashboard_service.go` | Standard Error | Yes | `/dashboard`, `/analytics` | SQL aggregates, no PII |
| `GET` | `/api/v1/dashboard/me` | Driver | `dashboard_service.go` | Standard Error | Missing in OpenAPI | `/dashboard` (driver) | Driver own stats only |
| `GET` | `/api/v1/dashboard/export.csv` | Admin, Fleet Manager, Financial Analyst | `export_service.go` | `text/csv` / Err | Missing in OpenAPI | `/dashboard`, `/analytics` | Streamed with quote escape |
| `GET` | `/api/v1/audit` | Admin | `audit_service.go` | Standard Error | Missing in OpenAPI | Admin viewer | Keyset paginated audit |
| `GET` | `/ws/fleet` (Legacy) | AnyAuthenticated | `app.go` | Handshake | Yes | 3 frontend pages | **GAP S-05**: unauthenticated handshake, replaced by `/api/v1/realtime/ticket` + `/api/v1/ws` |

---

## 2. Frontend Inline WebSocket Usage Inventory
Three frontend files establish inline WebSocket connections:
1. `frontend/src/app/dashboard/page.tsx:92` — `const socket = new WebSocket(wsUrl)`
2. `frontend/src/app/live-map/page.tsx:35` — `const socket = new WebSocket(wsUrl)`
3. `frontend/src/app/trips/page.tsx:150` — `const socket = new WebSocket(wsUrl)`

All 3 will be replaced by the shared hook `useFleetEvents` in Task 5.4.

---

## 3. Known Gaps Identified (Pre-Implementation)
- **GAP S-05**: Legacy `/ws/fleet` allows any authenticated user to connect directly with cookie, vulnerable to CSWSH. Replaced by single-use ticket pattern (`POST /api/v1/realtime/ticket` $\to$ `GET /api/v1/ws?ticket=...`).
- **GAP R-01**: In-memory websocket broadcasting (`a.broadcast`) fails across multi-instance deployments. Replaced by PostgreSQL `LISTEN/NOTIFY` on channel `fleet_events`.
- **GAP R-03**: Readiness probe must verify DB ping within 2s budget and return 503 if unreachable, without leaking system internals.
- **GAP R-04**: No per-request timeouts (15s context timeout) or DB statement timeout (5s).
- **GAP O-01**: No structured `slog` JSON logging with request ID; no `/metrics` endpoint with Prometheus counters and histograms.
- **GAP S-12**: Missing automated license-check job (`POST /api/v1/internal/jobs/license-check`) with deduplicated notifications.
