# Step 1: Claim Inventory

Extracted claims from `phase-4-report.md`, `phase-5-report.md`, `VERDICT.md`, and `evidence-audit.md`.

| Source Document | Claim ID / Feature | Cited Test Name | Cited File |
| :--- | :--- | :--- | :--- |
| `phase-4-report.md:14` | P4-01 / P4-02 | `TestCompulsory_Audit_TransactionalIntegrity` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:14` | P4-01 / P4-02 | `TestCompulsory_Audit_RollbackNoAuditRow` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:15` | P4-03 | `TestCompulsory_Audit_KeysetPagination` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:16` | P4-04 | `TestCompulsory_Audit_DateFilterValidation` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:17` | P4-05 | `TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:18` | P4-06 | `TestCompulsory_Dashboard_DateFilterScoping` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:19` | P4-07 | `TestCompulsory_Dashboard_FilterReach` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:20` | P4-09 / P4-10 | `TestCompulsory_Dashboard_DistanceActualOdometerOnly` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:21` | P4-11 | `TestCompulsory_Dashboard_QueryErrorReturns500` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:22` | P4-16 | `TestCompulsory_Expense_ConcurrentCancelRace` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:23` | P4-19 | `TestCompulsory_Finance_SummarySingleQuery` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:24` | P4-22 | `TestCompulsory_MigrationUpDownUp` | `internal/app/phase2_compulsory_test.go` |
| `phase-4-report.md:25` | P4-25 | `TestCompulsory_Maintenance_ClosePublishesEvent` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:26` | P4-27 | `TestCompulsory_Trip_Idempotency` | `internal/app/phase4_missing_compulsory_test.go` |
| `phase-4-report.md:27` | OpenAPI Contract | `TestOpenAPITypesDrift` | `internal/api/openapi_drift_test.go` |
| `phase-5-report.md:14` | P5-01 / C1 | `TestCompulsory_BoundedRouteLabelsInMetrics` | `internal/app/phase5_metrics_test.go` |
| `phase-5-report.md:15` | P5-02 / C2 | `TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` | `internal/app/phase5_logging_test.go` |
| `phase-5-report.md:16` | P5-03 / C2 | `TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` | `internal/app/phase5_logging_test.go` |
| `phase-5-report.md:17` | P5-04 / C3 | `TestCompulsory_RequestTimeout_503` | `internal/app/phase5_reliability_test.go` |
| `phase-5-report.md:18` | P5-06 / C5 | Source Inspection | `cmd/server/main.go:32-42` |
| `phase-5-report.md:19` | P5-07 / C6 | Source Inspection | `cmd/server/main.go:70-89` |
| `phase-5-report.md:20` | P5-07 / C7 | `TestCompulsory_PanicRecoveryAndRequestID` | `internal/app/phase5_logging_test.go` |
| `phase-5-report.md:21` | P5-08 / C8 | `TestCompulsory_ReadyzDatabaseBudget` | `internal/app/phase5_reliability_test.go` |
| `phase-5-report.md:22` | P5-09 / P5-22 | `TestPolicy_SanitizedClientPayloadPII` | `internal/realtime/realtime_test.go` |
| `phase-5-report.md:22` | P5-09 / P5-22 | `TestPolicy_RoleEventMatrix` | `internal/realtime/realtime_test.go` |
| `phase-5-report.md:23` | P5-20 / S-05 | `TestHub_MissingOrDisallowedOrigin_Rejected403` | `internal/realtime/hub_test.go` |
| `phase-5-report.md:24` | P5-28 / FE | Source Inspection | `frontend/src/lib/realtime/useFleetEvents.ts` |
| `phase-5-report.md:25` | P5-29 / FE | Source Inspection | `frontend/src/lib/realtime/useFleetEvents.ts` |
| `phase-5-report.md:30` | E1 / E2 | `TestCompulsory_LicenseCheckJobAndNotifications` | `internal/app/phase5_jobs_test.go` |
| `phase-5-report.md:80` | SLO & Latency | `TestCompulsory_Phase5_SLOAndLatencyVerification` | `internal/app/phase5_load_test.go` |
| `VERDICT.md:10` | M1 | `TestCompulsory_Audit_KeysetPagination` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:11` | M2 | `TestCompulsory_Audit_DateFilterValidation` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:12` | M3 | `TestCompulsory_Audit_TransactionalIntegrity` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:13` | M4 | `TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:14` | M5 | `TestCompulsory_Dashboard_KPICorrectnessFixture` | `internal/app/phase4_compulsory_test.go` |
| `VERDICT.md:15` | M6 | `TestCompulsory_Dashboard_QueryErrorReturns500` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:16` | M7 | `TestCompulsory_Expense_ConcurrentCancelRace` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:17` | M8 | `TestCompulsory_Finance_SummarySingleQuery` | `internal/app/phase4_missing_compulsory_test.go` |
| `VERDICT.md:18` | M9 | Frontend Idempotency tests | `frontend/src/lib/__tests__/api_idempotency.test.mjs` |
| `VERDICT.md:19` | M10 | `TestCompulsory_BoundedRouteLabelsInMetrics` | `internal/app/phase5_metrics_test.go` |
| `VERDICT.md:20` | M11 | `TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` | `internal/app/phase5_logging_test.go` |
| `VERDICT.md:21` | M12 | `TestCompulsory_RequestTimeout_503` | `internal/app/phase5_reliability_test.go` |
| `VERDICT.md:22` | M13 | `TestCompulsory_StatementTimeout_504` | `internal/app/phase5_reliability_test.go` |
| `VERDICT.md:23` | M14 | `TestServer_TimeoutConfig` | `cmd/server/server_test.go` |
| `VERDICT.md:24` | M15 | `TestCompulsory_ReadyzDatabaseBudget` | `internal/app/phase5_reliability_test.go` |
| `VERDICT.md:25` | M16 | `TestCompulsory_GracefulShutdownInFlight` | `internal/app/phase5_reliability_test.go` |
| `VERDICT.md:26` | M17 | `TestPolicy_RoleEventMatrix`, `TestPolicy_SanitizedClientPayloadPII` | `internal/realtime/realtime_test.go` |
| `VERDICT.md:27` | M18 | `TestEvents_PayloadSizeCheck` | `internal/realtime/realtime_test.go` |
| `VERDICT.md:28` | M19 | `TestHub_MissingOrDisallowedOrigin_Rejected403` | `internal/realtime/hub_test.go` |
| `VERDICT.md:29` | M20 | `TestTickets_HashStorageNoPlaintext` | `internal/realtime/realtime_test.go` |
| `VERDICT.md:30` | M21 | `TestTickets_LifecycleAndSingleUse` | `internal/realtime/realtime_test.go` |
| `VERDICT.md:31` | M22 | `TestCompulsory_LicenseCheckJobAndNotifications` | `internal/app/phase5_jobs_test.go` |
| `VERDICT.md:32` | M23 | `govulncheck` clean toolchain | `govulncheck` |
