# Phase 1 Implementation & Verification Report

## 1. Commits List

- `e1cf990 feat(config): add validated startup configuration` (Task 1.1)
- `65960bb test(config): add config validation tests` (Task 1.1)
- `2a42046 feat: integrate Go backend foundation` (Tasks 1.2 - 1.7 foundation)
- `fix(1.1-1.2): close verification gaps from earlier tasks, remove invalid auth_handlers stub, add auth tests`
- `feat(1.3): admin CLI with create-admin and env fallback, guard solo admin`
- `feat(1.4): enforce route role registration safety and route enumeration`
- `feat(1.5): strict middleware pipeline, HSTS in production, CSP default-src none, request-id validation`
- `feat(1.6): unified error model and database constraint violation mapping`
- `feat(1.7): OpenAPI schema contract, schema.ts, and make gen targets`
- `feat(1.8): seed safety, production refusal, random seed password generation, repository hygiene`
- `chore(1.9): Next.js audit clean, frontend sidebar RBAC unit tests, package scripts`
- `ci(1.10): GitHub Actions CI workflow for backend, frontend, migrations, and secrets`
- `docs(phase-1): notes and completion report`

---

## 2. Gap IDs Closed

- **S-01 (Insecure Auth & Sessions)**: Migrated authentication from stateless JWT to stateful cookie-backed sessions using SHA-256 token hashes in PostgreSQL `sessions` table. Implemented Argon2id password hashing with constant-time dummy verification for non-existent accounts.
- **S-02 (Open Registration)**: Removed public user self-registration (`POST /api/v1/auth/register`) and deleted the frontend register page. Created `cmd/admin` CLI with `--force` protection and implemented admin-only user management endpoints (`/api/v1/users`).
- **S-03 (Missing RBAC)**: Centralized permission matrix in `internal/rbac`, enforced explicit role assignment for all routes, prevented registration of routes without role sets (with public whitelist restricted strictly to `/healthz`, `/readyz`, `/api/v1/auth/login`).
- **S-06 (Unstructured Errors & Stack Leaks)**: Implemented unified JSON error writer in `internal/platform/httpx` producing standard `{"error": {"code", "message", "request_id"}}` structure, with database constraint mapping and panic recovery obscuring internal details.
- **S-07 (Insecure CORS & Missing Headers)**: Restricted CORS to validated origins from `ALLOWED_ORIGINS` with `Vary: Origin`. Injected security headers (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `Content-Security-Policy: default-src 'none'`, and HSTS in production).
- **S-08 (Unbounded Request Bodies & Rate Limiting)**: Enforced 1 MB request body limit returning 413, rate limiting with IP sliding window (120 req/min) returning 429 with `Retry-After`, and login lockout (5 failures/email/15m and 20 failures/IP/15m).
- **S-09 (Seed Credential Insecurity)**: Blocked `cmd/seed` execution when `ENV=production`. Replaced hardcoded `"password123"` with Argon2id hashed random passwords generated at runtime or read from `SEED_PASSWORD`.
- **S-10 (Frontend Credential Storage & Leaks)**: Removed all `localStorage` and `sessionStorage` token and role persistence from the frontend. Auth state is managed via HttpOnly cookies and verified via `/api/v1/auth/me`. Removed pre-filled credentials from login form.
- **V-01 (Weak Secrets)**: Enforced minimum 32-byte secret requirement in `internal/config`, rejecting known-weak and repetitive secrets.
- **V-02 (Plaintext Password Comparison)**: Removed legacy plaintext comparison paths.
- **V-03 (Token Revocation Failure)**: Added server-side session revocation on `POST /api/v1/auth/logout` and password changes.
- **V-04 (Solo Admin Deactivation)**: Protected the last active administrator from deactivation or demotion (returns 409).

---

## 3. Compulsory Test Execution Results

All tests automated and verified against live PostgreSQL:

| Test Name | File | Result |
|---|---|---|
| `TestCompulsory_RouterEnumeration` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_AuthAndLockout` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_UserManagementAndLastAdmin` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_SecurityHeadersAndErrorModel` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_CORS` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_Readyz` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_RoleRouteMatrix` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_ChangePasswordInvalidatesOtherSessions` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_IPLockout` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestCompulsory_ConstraintMapping` | `internal/app/phase1_compulsory_test.go` | PASS |
| `TestManualSmokeScenario` | `internal/app/smoke_test.go` | PASS |
| `TestHygiene_NoPassword123InSource` | `internal/app/hygiene_test.go` | PASS |
| `TestHygiene_NoLocalStorageInFrontendAuth` | `internal/app/hygiene_test.go` | PASS |
| `TestSeedProductionGuard` | `cmd/seed/seed_test.go` | PASS |
| `TestOpenAPITypesDrift` | `internal/api/drift_test.go` | PASS |
| `TestConfigValidation` | `internal/config/config_test.go` | PASS |
| `TestPasswordHashing` | `internal/auth/password_test.go` | PASS |
| `RBAC - Sidebar Visibility (5 roles)` | `frontend/src/lib/__tests__/rbac_sidebar.test.mjs` | PASS |
| `Migrations up -> down -> up` | `cmd/migrate` | PASS |

---

## 4. Decisions Not Specified in the Plan

1. **Path for Generated TypeScript Contract**: The plan specified generating TypeScript types into `frontend/src/lib/api/schema.ts`, while early foundation code created `types.gen.ts`. We synchronized both so imports remain fully backward-compatible while fulfilling the exact contract path.
2. **Database Field Types for Phase 1**: Full migration of legacy serial IDs to UUID across existing domain entities (trips, vehicles) was deferred to Phase 2 schema migration per section 3.6 of the plan. Users table was enhanced with UUID `id`, `is_active`, and `driver_license` in `000002_auth_foundation.up.sql`.
3. **CLI Administration Flags**: `cmd/admin` supports both flag arguments (`--email`, `--password`, `--force`), standard environment variables (`ADMIN_EMAIL`, `ADMIN_PASSWORD`), and interactive stdin fallback.

---

## 5. Gaps Found in Already-Committed 1.1 and 1.2 and Fixes Applied

1. **Empty Stub File in Foundation Commit**: A zero-byte stub file `internal/app/auth_handlers.go` was present in the git tree, which prevented Go packages from building (`expected 'package', found 'EOF'`). The `logout` handler was already correctly declared in `app.go`. The empty stub file was removed.
2. **Missing Automated Auth & Lockout Integration Tests**: Although password hashing had a unit test, integration tests for session cookie attributes, identical error responses, 15-minute lockouts, and session revocation were missing. Comprehensive automated tests were added in `internal/app/phase1_compulsory_test.go`.
3. **Request ID Sanitization**: Updated `httpx.EnsureRequestID` to validate inbound `X-Request-ID` length (<= 64 characters) and character set, generating a fresh UUID if malformed.
4. **Security Header Content-Security-Policy**: Aligned API Content Security Policy to `default-src 'none'` and added conditional HSTS header for non-local environments.

---

## 6. Items Not Done & Rationale

- **Go Race Detector on Local Windows Host**: Running `go test -race` locally failed with `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in` due to the local Windows MinGW compiler being 32-bit. All tests pass with `go test -count=1 ./...` locally. The race detector (`-race`) is configured and pinned in `.github/workflows/ci.yml` for execution on 64-bit Linux containers.

---

## 7. Existing Behavior Changed That Phase 2+ Depends On

1. **API Route Prefix**: All API routes are now accessed under `/api/v1/...` instead of `/api/...`.
2. **Authentication Flow**: Endpoints now expect session authentication via HttpOnly `transitops_session` cookie (or `Authorization: Bearer <token>` fallback) rather than stateless HS256 JWTs.
3. **Standard Error Envelope**: All API error responses conform to `{"error": {"code": string, "message": string, "request_id": string}}`.
4. **RBAC Guard**: Unauthenticated requests to non-whitelisted routes receive 401; requests with unauthorized roles receive 403.
