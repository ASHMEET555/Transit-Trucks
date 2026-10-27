# Remediation Verification 2 — Step 0 Confirmation

| Item / Claim | Finding in Code | Status |
| :--- | :--- | :--- |
| `git grep -n "func TestCompulsory_Audit_\|func TestCompulsory_Expense_\|func TestHub_MissingOrDisallowedOrigin"` | Found only `TestCompulsory_Audit_RoleMatrix` and `TestCompulsory_Audit_MutatingActionLoggingAndFiltering`. Required tests `TestCompulsory_Audit_TransactionalIntegrity`, `TestCompulsory_Audit_RollbackNoAuditRow`, `TestCompulsory_Audit_KeysetPagination`, `TestCompulsory_Audit_DateFilterValidation`, `TestCompulsory_Expense_ConcurrentCancelRace`, and `TestHub_MissingOrDisallowedOrigin_Rejected403` do not exist. | **CONFIRMED** (Missing as claimed) |
| `git grep -n "Shutdown("` | `backend-go/cmd/server/main.go:99` invokes `srv.Shutdown(shutdownCtx)`. Tests did not invoke `Shutdown` on `http.Server`. | **CONFIRMED** |
| `git grep -n "ws_tickets"` | No table or identifier named `ws_tickets`; table is named `realtime_tickets`. | **CONFIRMED** |
| `git grep -n "In_Progress"` | Neither `maintenance_logs` nor `trips` use `In_Progress`; status is `'Open'` and `'Closed'` for maintenance logs. | **CONFIRMED** |
| `frontend/src/lib/api.ts` fallback UUID | Lines 46-52: auto-generates `${Date.now()}-${Math.random().toString(36).substring(2, 9)}` if `crypto.randomUUID` is unavailable (not standard RFC 4122 UUID); does not reuse key on user action retry. | **CONFIRMED** |
| `backend-go/internal/realtime/tickets.go:55-80` consume query | Lines 63-70: Uses `UPDATE realtime_tickets SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now() RETURNING user_id, role`. Single atomic update. | **CONFIRMED** |
| Metrics test, reliability test, dashboard service | `phase5_metrics_test.go` and `phase5_reliability_test.go` have weak/incomplete assertions (synthetic slowHandler, no Server.Shutdown in flight, 50 paths instead of 1000). Dashboard service returns 500 via standard `httpx.WriteError(w, r, 500, "INTERNAL_ERROR", ...)`. | **CONFIRMED** |
