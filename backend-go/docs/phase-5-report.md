# Phase 5 — Realtime, Observability, Reliability: Remediation & Verification Report

## 1. Executive Summary
Phase 5 elevates TransitOps into an enterprise-grade, horizontally scalable, and observable fleet operations platform using PostgreSQL `LISTEN/NOTIFY` without Redis or external message brokers. WebSocket handshakes are secured using single-use tickets with strict `Origin` verification, client payloads are scrubbed of PII and filtered by role via an allowlist, observability is established via structured `slog` JSON logging with bounded route labels and error codes, and a protected Prometheus `/metrics` endpoint surfaces system and domain metrics. Reliability is enforced through strict request timeouts, database statement timeouts, a 2-second readiness probe (`/readyz`), tuned connection pools, and graceful shutdown. Automated compliance is supported by an idempotent driver license expiration scan job and notifications API.

Following the independent code review and gap audit (`docs/remediation-gap-audit.md`), all identified defects (P5-01 through P5-29) were remediated, verified with compulsory tests against PostgreSQL 16, and integrated into the Next.js 15 frontend.

---

## 2. Remediation Summary & Verification Details

| Item | Requirement & Fix | Verification & Test Evidence |
| :--- | :--- | :--- |
| **P5-01 / C1** | Metrics route cardinality: routes resolved via `a.getRoutePattern(r)` matching against registered routes in `a.registry.Routes()`. Dynamic parameters (`/api/v1/drivers/{license}`) are parameterized; unmapped paths collapse to `unmatched`. Zero sensitive parameters in metric labels. | `internal/app/phase5_metrics_test.go:TestCompulsory_BoundedRouteLabelsInMetrics` |
| **P5-02 / C2** | Access log route scrubbing: `middleware.AccessLog` uses the resolved route pattern from context (`middleware.GetRoutePattern(r)`). Driver licenses and entity IDs are never logged in the `route` field. | `internal/app/phase5_logging_test.go:TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` |
| **P5-03 / C2** | Access log error code: `httpx.WriteError` sets header `X-Error-Code`, which `middleware.AccessLog` reads and logs as `error_code` on error responses. | `internal/app/phase5_logging_test.go:TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` |
| **P5-04 / C3** | Request timeout 503: `middleware.RequestTimeout` wraps requests in a 15-second context timeout and timeout writer. If the deadline expires before completion, it writes HTTP 503 `REQUEST_TIMEOUT`. | `internal/app/phase5_reliability_test.go:TestCompulsory_RequestTimeout_503` |
| **P5-06 / C5** | Connection pool MinConns: pgx pool configured in `cmd/server/main.go` with `poolCfg.MinConns = 2` (configurable via `DB_MIN_CONNS`) and `poolCfg.MaxConns = 10` (configurable via `DB_MAX_CONNS`). | `cmd/server/main.go:32-42` |
| **P5-07 / C6** | Server timeouts & shutdown: `http.Server` configured with `ReadHeaderTimeout = 5 * time.Second` and `IdleTimeout = 60 * time.Second`. Graceful shutdown timeout set to 20 seconds. | `cmd/server/main.go:70-89` |
| **P5-07 / C7** | Panic recovery stack trace: `middleware.Recover` logs `debug.Stack()` server-side with correlation `request_id`, while returning standard 500 `INTERNAL_ERROR` without leaking internal traces to the response body. | `internal/app/phase5_logging_test.go:TestCompulsory_PanicRecoveryAndRequestID` |
| **P5-08 / C8** | Readiness probe budget & status: `/readyz` executes a database ping within a 2-second timeout. Returns HTTP 200 `{"status": "ok"}` on success, or HTTP 503 `{"status": "unavailable"}` when database is unreachable. | `internal/app/phase5_reliability_test.go:TestCompulsory_ReadyzDatabaseBudget` |
| **P5-09 / P5-22** | Event publishing & PII allowlist: all state mutations (vehicle retire, driver status changes, trip transitions) publish events via `realtime.PublishTx`. Allowlist sanitizer (`realtime.AllowedPayloadKeys`) guarantees zero PII in broadcast payloads. | `internal/realtime/realtime_test.go:TestPolicy_SanitizedClientPayloadPII`<br/>`internal/realtime/realtime_test.go:TestPolicy_RoleEventMatrix` |
| **P5-20 / S-05** | Legacy WebSocket route removed: unauthenticated `/ws/fleet` route removed. WebSocket connection strictly requires `GET /api/v1/ws?ticket=...` with a single-use ticket and valid `Origin` header (403 on missing/disallowed origin before upgrade). | `internal/realtime/hub_test.go:TestHub_MissingOrDisallowedOrigin_Rejected403` |
| **P5-28 / FE** | Frontend ticket URL: fixed `useFleetEvents.ts` and `api.ts` to normalize path, preventing duplicate `/api/v1/api/v1/realtime/ticket`. | `frontend/src/lib/realtime/useFleetEvents.ts:63-66`<br/>`frontend/src/lib/api.ts:66-70` |
| **P5-29 / FE** | Frontend backoff & visibility: `useFleetEvents.ts` implements exponential backoff with full jitter (1s base, 30s cap), debounces reconnect refetches, and listens to `visibilitychange` to reconnect when tab becomes visible. | `frontend/src/lib/realtime/useFleetEvents.ts:133-157` |
| **FE-01** | Frontend ROI duplication: removed duplicate `* 100` on dashboard ROI chart and table (`frontend/src/app/dashboard/page.tsx`). | `frontend/src/app/dashboard/page.tsx:210,337` |
| **FE-02** | Frontend role-based dashboard calls: Driver role calls `/dashboard/me` (avoiding 403 on `/dashboard/kpis`), displaying personal trips and distance. Other roles call `/dashboard/kpis`. | `frontend/src/app/dashboard/page.tsx:84-98` |
| **FE-03** | Frontend expense trip selector: `frontend/src/app/fuel-and-expenses/page.tsx` filters trips to `Dispatched` and `Completed` only, preventing 422 `EXPENSE_ON_INACTIVE_TRIP`. | `frontend/src/app/fuel-and-expenses/page.tsx:60-63` |
| **CSV Guard** | Frontend CSV export injection protection: `sanitizeCsvCell` in `frontend/src/lib/utils.ts` prepends `'` to any cell starting with `=`, `+`, `-`, `@`, `\t`, `\r` across expense and maintenance exports. | `frontend/src/app/fuel-and-expenses/page.tsx:125-135`<br/>`frontend/src/app/maintenance/page.tsx:148-158` |
| **E1 / E2** | License check job & notifications: `POST /api/v1/internal/jobs/license-check` returns `JobResult` matching OpenAPI (`processed`, `expired`, `warning`). `GET /api/v1/notifications` supports boolean `is_read`, limit, and keyset cursor pagination with 422 validation on invalid cursor. | `internal/app/phase5_jobs_test.go:TestCompulsory_LicenseCheckJobAndNotifications` |

---

## 3. Realtime & Observability Architecture

### 3.1 Single-Use WebSocket Ticket Lifecycle
1. Client requests ticket via authenticated `POST /api/v1/realtime/ticket`.
2. Server generates a cryptographically secure random token (32 bytes), hashes it with SHA-256, and stores `(token_hash, user_id, role, expires_at)` in `realtime_tickets`.
3. Client establishes WebSocket connection via `GET /api/v1/ws?ticket=<token>`.
4. Server validates `Origin` header against allowed origins (403 before upgrade if missing/disallowed).
5. Server atomically consumes the ticket (`UPDATE realtime_tickets SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`).
6. Upgrades connection to WebSocket (101 Switching Protocols) and binds connection to user ID and role.

### 3.2 Transactional Postgres LISTEN/NOTIFY Event Fan-Out
- Mutating services call `realtime.PublishTx(ctx, tx, eventType, payload)` within the SQL transaction.
- `pg_notify('fleet_events', ...)` is executed within the transaction; rolled-back transactions emit zero events.
- A dedicated session-held connection runs `LISTEN fleet_events` with auto-reconnect.
- Events are dispatched to connected WebSocket clients via an allowlist sanitizer (`realtime.AllowedPayloadKeys`) ensuring zero PII leakage.
- Slow consumers whose send channel buffers (64 messages) fill up are dropped cleanly with WebSocket close code 1013 (`CloseSlowConsumer`).

### 3.3 Prometheus Metrics Families
`GET /metrics` requires a valid Bearer token matching `METRICS_TOKEN` (returns 404 to unauthorized callers).
Output verified metric families:
- `http_requests_total{method, route, status_code, status}` (bounded route cardinality)
- `http_request_duration_seconds{method, route}` histogram
- `http_in_flight_requests` gauge
- `db_pool_connections{state="total|idle|acquired"}` and `db_pool_total_connections`, `db_pool_idle_connections`, `db_pool_acquired_connections`
- `ws_connections{role}`
- `events_published_total`, `events_delivered_total`, `events_dropped_total`
- `domain_vehicles_total{status}`, `domain_trips_total{status}` (cached 15s)

---

## 4. Verification Logs & Performance Evidence

### Go Test Suite (`CGO_ENABLED=0 go test -count=1 ./...`)
Log: `docs/verification/go-test.log`
```
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/cmd/seed	0.972s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/api	1.779s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app	107.058s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth	0.288s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/config	0.048s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware	0.092s
ok  	github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime	0.158s
```
- Total test status: **100% PASS**
- `go vet ./...`: **Clean, zero warnings** (`docs/verification/go-vet.log`)

### SLO & Latency Benchmark Results (`TestCompulsory_Phase5_SLOAndLatencyVerification`)
- **Event Delivery Latency (post-commit to WebSocket client):**
  - Minimum: 10.18 ms
  - Maximum: 40.48 ms
  - **p95: 40.48 ms** (SLO target: $< 2000\text{ ms}$)
- **Steady State Operations (900 requests at 15 concurrent VUs):**
  - **Availability:** 100.00% (SLO target: $\ge 99.50\%$)
  - **5xx Rate:** 0.00% (SLO target: $< 0.50\%$)
  - **p95 Read Latency:** 31.93 ms (SLO target: $< 300\text{ ms}$)
  - **p95 Write Latency:** 24.11 ms (SLO target: $< 500\text{ ms}$)
- **3x Concurrency Spike (225 requests under burst):**
  - **Error Rate:** 0.00% (SLO target: $< 1.00\%$)

### Frontend Verification Suite
- `npm run typecheck`: **PASS** (`docs/verification/frontend-typecheck.log`)
- `npm run lint`: **PASS** (`docs/verification/frontend-lint.log`)
- `npm test`: **8/8 PASS** (`docs/verification/frontend-test.log`)
- `npm run build`: **15/15 static pages PASS** (`docs/verification/frontend-build.log`)
