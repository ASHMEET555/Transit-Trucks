# Phase 1 Inventory & Architecture Notes

## 1. Route Inventory

| Method | Path | Handler File & Func | Current Role Check | Request Body | Response Body | Status Codes |
|---|---|---|---|---|---|---|
| `GET` | `/` | `internal/app/app.go:36` (inline) | None (Public) | None | `{"status": "online", "system": "TransitOps Backend Architecture Operational"}` | 200 |
| `GET` | `/ws/fleet` | `internal/app/app.go:562` (`ws`) | None (Public / Unauthenticated WS) | WebSocket frames | WebSocket event frames (`trip.*`, `maintenance.*`, `expense.*`) | 101, 400 |
| `POST` | `/api/auth/login` | `internal/app/auth.go:56` (`login`) | None (Public) | `application/x-www-form-urlencoded`: `username`, `password` | `{"access_token": string, "token_type": "bearer", "role": string}` | 200, 400, 422 |
| `POST` | `/api/auth/register` | `internal/app/auth.go:71` (`register`) | None (Public, allows `role: Admin`) | JSON: `{"email": string, "password": string, "role": string}` | `{"message": string, "email": string, "role": string}` | 201, 400, 422, 500 |
| `GET` | `/api/fleet/vehicles` | `internal/app/app.go:88` (`vehicles`) | `auth` (Any authenticated user) | None | `[]vehicle` | 200, 500 |
| `POST` | `/api/fleet/vehicles` | `internal/app/app.go:104` (`createVehicle`) | `role(["Fleet Manager"])` (or Admin) | JSON: `vehicle` | JSON: `vehicle` | 201, 400, 422, 500 |
| `GET` | `/api/fleet/vehicles/{reg}` | `internal/app/app.go:96` (`vehicle`) | `auth` (Any authenticated user) | None | JSON: `vehicle` | 200, 404 |
| `PUT` | `/api/fleet/vehicles/{reg}` | `internal/app/app.go:126` (`updateVehicle`) | `role(["Fleet Manager"])` (or Admin) | JSON: `vehicle` | `{"message": "Vehicle updated successfully"}` | 200, 404, 422 |
| `DELETE` | `/api/fleet/vehicles/{reg}` | `internal/app/app.go:139` (`deleteVehicle`) | `role(["Fleet Manager"])` (or Admin) | None | `{"message": "Vehicle deleted successfully"}` | 200, 404 |
| `GET` | `/api/fleet/drivers` | `internal/app/app.go:147` (`drivers`) | `auth` (Any authenticated user) | None | `[]driver` | 200, 500 |
| `POST` | `/api/fleet/drivers` | `internal/app/app.go:163` (`createDriver`) | `role(["Fleet Manager", "Safety Officer"])` (or Admin) | JSON: `driver` | JSON: `driver` | 201, 400, 422, 500 |
| `GET` | `/api/fleet/drivers/{license}` | `internal/app/app.go:155` (`driver`) | `auth` (Any authenticated user) | None | JSON: `driver` | 200, 404 |
| `PUT` | `/api/fleet/drivers/{license}` | `internal/app/app.go:188` (`updateDriver`) | `role(["Fleet Manager", "Safety Officer"])` (or Admin) | JSON: `driver` | `{"message": "Driver updated successfully"}` | 200, 404, 422 |
| `DELETE` | `/api/fleet/drivers/{license}` | `internal/app/app.go:201` (`deleteDriver`) | `role(["Fleet Manager"])` (or Admin) | None | `{"message": "Driver profile purged successfully"}` | 200, 404 |
| `GET` | `/api/destinations/` | `internal/app/app.go:210` (`destinations`) | `auth` (Any authenticated user) | None | `[]destination` | 200, 500 |
| `POST` | `/api/destinations/` | `internal/app/app.go:218` (`createDestination`) | `role(["Fleet Manager"])` (or Admin) | JSON: `destination` | JSON: `destination` | 200, 422, 500 |
| `DELETE` | `/api/destinations/{name}` | `internal/app/app.go:231` (`deleteDestination`) | `role(["Fleet Manager"])` (or Admin) | None | `{"message": "Destination deleted successfully"}` | 200, 404 |
| `GET` | `/api/trips/` | `internal/app/app.go:240` (`trips`) | `auth` (Any authenticated user) | None | `[]trip` | 200, 500 |
| `POST` | `/api/trips/suggest` | `internal/app/app.go:248` (`suggest`) | `role(["Fleet Manager", "Driver"])` (or Admin) | JSON: `trip` (`cargo_weight`, `source_name`, `destination_name`) | JSON: heuristic recommendation object | 200, 400, 404, 422 |
| `POST` | `/api/trips/` | `internal/app/app.go:286` (`dispatch`) | `role(["Fleet Manager", "Driver"])` (or Admin) | JSON: `trip` | JSON: `trip` | 201, 400, 404, 422, 500 |
| `PUT` | `/api/trips/{id}/complete` | `internal/app/app.go:333` (`complete`) | `auth` (Any authenticated user) | Query: `final_odometer` | `{"message": string}` | 200, 400, 404 |
| `PUT` | `/api/trips/{id}/cancel` | `internal/app/app.go:356` (`cancel`) | `auth` (Any authenticated user) | None | `{"message": string}` | 200, 400, 404 |
| `GET` | `/api/maintenance/` | `internal/app/app.go:374` (`maintenance`) | `auth` (Any authenticated user) | None | `[]maintenance` | 200, 500 |
| `POST` | `/api/maintenance/` | `internal/app/app.go:382` (`openMaintenance`) | `role(["Fleet Manager"])` (or Admin) | JSON: `maintenance` | JSON: `maintenance` | 200, 400, 404, 422, 500 |
| `PUT` | `/api/maintenance/{id}/close` | `internal/app/app.go:407` (`closeMaintenance`) | `role(["Fleet Manager"])` (or Admin) | JSON: `{"end_date": string}` | `{"message": string}` | 200, 400, 404, 422 |
| `POST` | `/api/finance/expenses` | `internal/app/app.go:431` (`expense`) | `role(["Financial Analyst", "Fleet Manager"])` (or Admin) | JSON: `expenseRow` | JSON: `expenseRow` | 201, 400, 404, 422, 500 |
| `GET` | `/api/finance/expenses/{id}` | `internal/app/app.go:460` (`expenses`) | `auth` (Any authenticated user) | None | `[]expenseRow` | 200, 404, 500 |
| `GET` | `/api/dashboard/kpis` | `internal/app/app.go:475` (`kpis`) | `auth` (Any authenticated user) | None | JSON: dashboard KPI aggregate object | 200 |

---

## 2. Environment Variables Inventory

| Env Var | Location(s) | Default / Behavior |
|---|---|---|
| `POSTGRES_DSN` | `backend-go/cmd/server/main.go:13`, `backend-go/cmd/seed/main.go:12` | Required; fatal exit or panic if empty |
| `SECRET_KEY` | `backend-go/cmd/server/main.go:24` | Defaults to `"default_secret_key"` (insecure) |
| `PORT` | `backend-go/cmd/server/main.go:25-26` | Defaults to `"8000"` |
| `JSON_PATH` | `backend-go/cmd/seed/main.go:16` | Defaults to `"dummy_data.json"` |
| `TEST_POSTGRES_DSN` | `backend-go/internal/app/app_test.go:15` | If not set, test skips |
| `NEXT_PUBLIC_API_URL` | `frontend/src/data/data.ts:2` | Defaults to `"http://localhost:8000/api"` |

---

## 3. Password Hashing and Comparison Analysis

- **Location**: `backend-go/internal/app/auth.go:64` and `internal/app/auth.go:88`.
- **Hashing Algorithm**:
  - `register` uses `bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)`.
  - `seed` in `backend-go/cmd/seed/main.go:52` directly loads raw strings from `dummy_data.json` without hashing (e.g. `"password123"`).
- **Comparison Logic**:
  - `auth.go:64`:
    ```go
    if e != nil || ((strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$")) && bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) != nil) || (!strings.HasPrefix(hash, "$2") && hash != pw) {
        fail(w, 400, "Incorrect email or password")
        return
    }
    ```
  - **Vulnerability**: Plaintext comparison exists directly: `(!strings.HasPrefix(hash, "$2") && hash != pw)`. If the hash in the DB is not a bcrypt prefix, it compares passwords in plaintext!
  - Furthermore, failed login returns status code `400` with `{"detail": "Incorrect email or password"}` rather than `401`.
  - Unknown user returns immediately upon query scan failure without timing mitigation (dummy hash).

---

## 4. Sessions and Tokens: Creation, Validation, and Storage

- **Backend**:
  - Tokens are signed JWTs using HS256 with `a.secret` (`github.com/golang-jwt/jwt/v5`).
  - Claims: `{"sub": email, "exp": now + 1h}`.
  - No server-side session tracking exists. No `sessions` or `login_attempts` tables.
  - Validation: Reads `Authorization: Bearer <token>` header, parses with HS256 key, then queries `SELECT role FROM users WHERE email=$1`.
  - Server cannot revoke tokens upon logout (no logout endpoint exists).
- **Frontend**:
  - `frontend/src/lib/api.ts`:
    - Stores token, role, and email in browser `localStorage`:
      - `TOKEN_KEY = "transitops_token"`
      - `ROLE_KEY = "transitops_role"`
      - `EMAIL_KEY = "transitops_email"`
    - Injects `Authorization: Bearer <token>` into fetch request headers.
    - Exposes `setAuthSession`, `clearAuthSession`, `getToken`, `getStoredRole`, `getStoredEmail`.
  - `frontend/src/lib/ProfileContext.tsx`:
    - Reads session from `localStorage` on initial load.
    - Stores `token` in React state.
    - No `/auth/me` identity endpoint exists; role is trusted from `localStorage`.

---

## 5. Current CORS, Middleware, and Error Shapes

- **CORS**:
  - Implemented in `internal/app/app.go:67-79`:
    ```go
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Credentials", "true")
    w.Header().Set("Access-Control-Allow-Methods", "*")
    w.Header().Set("Access-Control-Allow-Headers", "*")
    ```
  - Wildcard `*` combined with `Access-Control-Allow-Credentials: true` is an insecure CORS configuration and violates browser standards.
- **Middleware Chain**:
  - Currently, there is only the CORS wrapper around `http.ServeMux`.
  - Route-level wrappers: `a.auth(...)` and `a.role(...)` chained inside handler registrations.
  - Missing: `recover`, `request-id`, `access log` (structured slog), `secure headers`, `body size limit` (1 MB), `rate limit`, centralized `auth`, centralized `RBAC`.
- **Error Response Shape**:
  - Handled via `fail(w http.ResponseWriter, s int, d string)` in `internal/app/app.go:85`:
    ```json
    { "detail": "error message string" }
    ```
  - Does NOT match target specification:
    ```json
    {
      "error": {
        "code": "STRING_CODE",
        "message": "Human readable message",
        "request_id": "uuid-string",
        "details": {}
      }
    }
    ```

---

## 6. Current Schema (Migrations) and Seed / Default Data

- **Migrations**:
  - `backend-go/migrations/000001_initial_schema.up.sql`:
    - Tables: `users`, `destinations`, `vehicles`, `drivers`, `trips`, `maintenance_logs`, `expenses`.
    - No `sessions` table.
    - No `login_attempts` table.
    - No `CHECK` constraints on enums, amounts, cargo weights, odometer, dates.
    - Primary keys are mostly `TEXT` (or integer `GENERATED BY DEFAULT AS IDENTITY`).
    - Uses `DOUBLE PRECISION` instead of `NUMERIC`.
    - `users` table only has `(email TEXT PRIMARY KEY, password_hash TEXT NOT NULL, role TEXT NOT NULL)`.
  - `backend-go/migrations/000001_initial_schema.down.sql`:
    - Drops tables in reverse dependency order.
- **Seed Data**:
  - `backend-go/dummy_data.json`:
    - Contains 50+ users, all with `"password_hash": "password123"`.
    - `cmd/seed/main.go` runs with no environment validation, happily inserts plaintext passwords, and has no `ENV=production` check.
  - `frontend/src/app/login/page.tsx`:
    - Hardcodes pre-filled credentials: `manager@transitops.com` and `password123`.

---

## 7. Gap List for Phase 1 (1.1 to 1.10)

| Task | Plan Requirement | Current State | Required Changes |
|---|---|---|---|
| **1.1 Config** | Typed config struct validated at startup. Required: `DATABASE_URL`, `SESSION_SECRET` (>= 32 bytes, reject known-weak), `ENV` (`local` or `production`), `ALLOWED_ORIGINS`, `COOKIE_DOMAIN`. Exit non-zero on error. No defaults for secrets. | Untyped `os.Getenv`, `SECRET_KEY` defaults to `"default_secret_key"`, no validation, no domain/origin checks. | Create `internal/config/config.go` with strict validation, known weak secret check (e.g. `default_secret_key`, `secret`, `password`, etc.), required env vars, and wire into `cmd/server/main.go`. |
| **1.2 Auth** | `sessions` table (SHA-256 token hash, `expires_at`, `last_seen_at`, `ip`, `user_agent`), `login_attempts` table. Argon2id hashing. Delete plaintext compare. 401 with identical body on failure (dummy hash for timing). Cookie `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` (unless `ENV=local`), 8h sliding / 7d absolute expiry. Lockout (5/15m email, 20/15m IP -> 429 Retry-After). `POST /auth/logout`, `GET /auth/me`, `POST /auth/change-password`. Remove `localStorage` token use from frontend. | JWT only, no session table, bcrypt/plaintext comparison, 400 on bad login, no timing protection, no cookies, no lockout, no me/logout/change-password endpoints. Frontend uses `localStorage`. | Add migration for `sessions` and `login_attempts`, implement Argon2id hashing and dummy verification, implement cookie session management with SHA-256 hashes, add rate lockout, implement `/auth/me`, `/auth/logout`, `/auth/change-password`, update frontend to remove `localStorage` tokens and use cookie + `/auth/me`. |
| **1.3 Open Registration** | Delete `POST /auth/register` and register page. Users created only by Admin via `POST /users` or `cmd/admin` CLI (refuses if admin exists unless `--force`). Admin cannot demote/deactivate last active Admin. | `POST /api/auth/register` open to public and accepts `role: Admin`. Register page in frontend. No `cmd/admin` CLI. No user management endpoints. | Delete `register` handler and route. Delete `frontend/src/app/register/page.tsx`. Implement `cmd/admin` CLI with `--force` flag check. Implement `GET /api/v1/users`, `POST /api/v1/users`, `PATCH /api/v1/users/{id}`, `POST /api/v1/users/{id}/reset-password` (Admin only). Guard last active Admin. |
| **1.4 RBAC** | Single permission matrix in `internal/rbac`. Every route registers explicit role set. Test enumerates router and fails if route lacks role set (except `/healthz`, `/readyz`, `/api/v1/auth/login`). | Ad-hoc `a.role(...)` calls in `routes()`. No centralized matrix. No router enumeration test. | Implement `internal/rbac` package with matrix and route registry metadata. Write automated test verifying every route has explicit roles except public exceptions. |
| **1.5 Middleware** | Order: `recover` -> `request-id` -> `access log` -> `secure headers` -> `CORS` (allow-list, no `*`, credentials for allowed origins) -> `body size limit` (1 MB) -> `rate limit` -> `auth` -> `RBAC`. | Only insecure CORS middleware and ad-hoc handler wrappers. | Implement middleware chain adhering to the exact required order. Enforce 1 MB body size (`http.MaxBytesReader`), security headers (`X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, etc.), IP/email rate limiting. |
| **1.6 Error Model** | Single writer in `platform/httpx`. Map pgx constraint violations by name to 409 or 422. Unhandled errors -> 500 with generic message; details only in logs. Shape: `{"error": {"code", "message", "request_id", "details"}}`. | `fail(w, code, detail)` returning `{"detail": "..."}`. | Create `internal/platform/httpx` error writer and uniform JSON formatting. Map Postgres errors cleanly. |
| **1.7 Contract** | `api/openapi.yaml` describing every existing endpoint under `/api/v1`. Generated Go types and TypeScript types. CI fails if out of date. | No OpenAPI spec. No type generation scripts or drift check. | Write comprehensive `api/openapi.yaml`. Add code generation tools (`oapi-codegen` and `openapi-typescript` / typecheck scripts). Add drift test. |
| **1.8 Seed Safety** | `cmd/seed` refuses `ENV=production`. Users get random passwords printed once or read from env. Remove all plaintext passwords from code, docs, and login page defaults. | `cmd/seed` has no ENV check, inserts `"password123"`. Login page pre-fills `"password123"`. | Add `ENV=production` guard to `cmd/seed`. Generate secure random passwords and hash with Argon2id. Remove pre-filled credentials from login page. Clean dummy data and docs. |
| **1.9 Dependencies** | Upgrade `next` to patched release. `npm audit --omit=dev` shows 0 critical/high. | Frontend `package.json` uses older Next.js version susceptible to audit findings. | Audit and upgrade Next.js to secure version. Ensure 0 critical/high audit vulnerabilities. |
| **1.10 CI** | `.github/workflows/ci.yml`: Postgres service container; `go vet`, `golangci-lint`, `go test -race ./...`, `govulncheck ./...`, `gitleaks`; frontend `lint`, `typecheck`, `build`, `npm audit`; OpenAPI drift check; migration `up` -> `down` -> `up`. | Only generic / missing CI workflow. | Create `.github/workflows/ci.yml` with all required gates. |

---

## 8. Mismatches Between Plan Assumptions and Existing Code

1. **Target Directory Structure**:
   - Plan specifies `backend/` in section 3.8, but real codebase uses `backend-go/` (Go) and `backend/` (legacy Python).
   - *Decision*: Maintain code in `backend-go/` adhering to the prompt requirements and repository layout.
2. **Endpoint Prefix**:
   - Existing Go endpoints were mounted at `/api/...` (e.g. `/api/fleet/vehicles`, `/api/auth/login`).
   - Plan requires all endpoints to move to `/api/v1/...` (e.g. `/api/v1/auth/login`, `/api/v1/fleet/vehicles`).
   - *Decision*: Move all API endpoints to `/api/v1/...` and update frontend `api.ts` / `siteConfig.ts` to query `/api/v1`. Keep `/healthz` and `/readyz` at root as specified.
3. **Primary Keys & Types**:
   - Plan specifies UUID for `users.id`, `trips.id`, `maintenance_logs.id`, `expenses.id` in section 3.6, but section 3.6 also notes that the complete schema transition is scheduled for Phase 2 (`0002_final_schema`).
   - For Phase 1, `users` table needs to support user management (`/api/v1/users` and `cmd/admin`), session association, and Argon2id passwords.
   - *Decision*: For Phase 1, add migration `000002_auth_foundation.up.sql` to add `sessions`, `login_attempts`, and necessary user columns (`is_active`, `driver_license`, etc.) without breaking current table foreign keys until Phase 2.
4. **Existing Endpoints Not In Plan's Phase 1 Table**:
   - Endpoints like `/api/trips/suggest`, `/api/maintenance/...`, `/api/finance/...`, `/api/dashboard/kpis` were not in Phase 1 endpoint table.
   - *Decision*: As instructed by prompt, retain their behavior, move them under `/api/v1/`, and assign appropriate explicit roles in the RBAC matrix (e.g. `suggest`: Fleet Manager, Driver, Admin; `complete`/`cancel`: Driver, Fleet Manager, Admin; `maintenance`: Fleet Manager, Admin; `expenses`: Financial Analyst, Fleet Manager, Admin; `kpis`: any authenticated).

---

## 9. Baseline Test Suite Execution

Executed before making any codebase changes:

```powershell
$env:TEST_POSTGRES_DSN="postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
go test -v ./...
```

**Output**:
```
=== RUN   TestHealth
--- PASS: TestHealth (0.01s)
PASS
ok  	github.com/Reaper-ai/TransitOps_Odoo2026/backend-go/internal/app	0.046s
```

All 1 existing test passed. No other test files existed in `backend-go`.
