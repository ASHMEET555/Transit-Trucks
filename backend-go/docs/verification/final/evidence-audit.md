# Phase 4 & Phase 5 Evidence Audit

Generated from fresh runs against PostgreSQL database.

## Test Log Verification

| Test Name | File | Exists in Repo | Ran & Captured in Log | Result |
| :--- | :--- | :---: | :---: | :---: |
| `TestOpenAPITypesDrift` | `internal/api/openapi_drift_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_RouterEnumeration` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_AuthAndLockout` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_UserManagementAndLastAdmin` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_SecurityHeadersAndErrorModel` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_CORS` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Readyz` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_RoleRouteMatrix` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ChangePasswordInvalidatesOtherSessions` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_IPLockout` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ConstraintMapping` | `internal/app/phase1_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_DuplicateRegistrationAndRace` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_NegativeValuesAndSqlCheckViolation` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_StatusBanana` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_PatchContainingStatus` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_SoftDeleteAndIncludeInactive` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_DriverRoleScopeAndWriteProtection` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_PaginationNoGapsAndLimitClamp` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_PartialUniqueIndexes` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_MigrationUpDownUp` | `internal/app/phase2_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ConcurrentVehicleDispatch` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ConcurrentDriverDispatch` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ConcurrentDispatchAndComplete` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_200MixedConcurrentOperations` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_LockOrderNoDeadlock` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_RuleIsolation` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_TripLifecycle` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_IdempotencyKey` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_TripAuthorization` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_FaultInjectionAndRollback` | `internal/app/phase3_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_OpenValidation` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_CloseValidation` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_RetireGuard` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_ConcurrentOpen` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_ConcurrentOpenAndDispatch` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Maintenance_RoleMatrix` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Finance_ExpenseValidation` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Finance_SummaryMath` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Finance_RoleMatrix` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Finance_FuelMigration` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_KPICorrectnessFixture` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/TestCompulsory_Dashboard_KPICorrectnessFixture.log` | PASS |
| `TestCompulsory_Dashboard_Benchmark10kTrips` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/TestCompulsory_Dashboard_Benchmark10kTrips.log` | PASS |
| `TestCompulsory_Dashboard_RoleMatrix` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_DriverPIIScrubbed` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_ExportCSV_StreamAndInjection` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/TestCompulsory_Dashboard_ExportCSV_StreamAndInjection.log` | PASS |
| `TestCompulsory_DataMinimization_ComprehensiveMatrix` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_RoleMatrix` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_MutatingActionLoggingAndFiltering` | `internal/app/phase4_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_TransactionalIntegrity` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_RollbackNoAuditRow` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_KeysetPagination` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Audit_DateFilterValidation` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_DateFilterScoping` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_FilterReach` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_DistanceActualOdometerOnly` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Dashboard_QueryErrorReturns500` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/TestCompulsory_Dashboard_QueryErrorReturns500.log` | PASS |
| `TestCompulsory_Expense_ConcurrentCancelRace` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Finance_SummarySingleQuery` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/TestCompulsory_Finance_SummarySingleQuery.log` | PASS |
| `TestCompulsory_Maintenance_ClosePublishesEvent` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Trip_Idempotency` | `internal/app/phase4_missing_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_WebSocketHandshakeTicketsAndOrigin` | `internal/app/phase5_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_TwoInstanceFanOutAndRollback` | `internal/app/phase5_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_RoleEventFilteringOverWebSocket` | `internal/app/phase5_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_MaxConnectionsAndSlowConsumer` | `internal/app/phase5_compulsory_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_LicenseCheckJobAndNotifications` | `internal/app/phase5_jobs_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_Phase5_SLOAndLatencyVerification` | `internal/app/phase5_load_test.go` | YES | `docs/verification/final/TestCompulsory_Phase5_SLOAndLatencyVerification.log` | PASS |
| `TestCompulsory_PanicRecoveryAndRequestID` | `internal/app/phase5_logging_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_LogScrubSecretNeverLogged` | `internal/app/phase5_logging_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_AccessLog_ErrorCodeAndNoLicenseLeak` | `internal/app/phase5_logging_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ProtectedMetricsEndpoint` | `internal/app/phase5_metrics_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_BoundedRouteLabelsInMetrics` | `internal/app/phase5_metrics_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_RequestTimeout_503` | `internal/app/phase5_reliability_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_StatementTimeout_504` | `internal/app/phase5_reliability_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestCompulsory_ReadyzDatabaseBudget` | `internal/app/phase5_reliability_test.go` | YES | `docs/verification/final/TestCompulsory_ReadyzDatabaseBudget.log` | PASS |
| `TestCompulsory_GracefulShutdownInFlight` | `internal/app/phase5_reliability_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestHub_MissingOrDisallowedOrigin_Rejected403` | `internal/realtime/hub_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestTickets_LifecycleAndSingleUse` | `internal/realtime/realtime_test.go` | YES | `docs/verification/final/TestTickets_LifecycleAndSingleUse.log` | PASS |
| `TestPolicy_RoleEventMatrix` | `internal/realtime/realtime_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestPolicy_SanitizedClientPayloadPII` | `internal/realtime/realtime_test.go` | YES | `docs/verification/final/compulsory.log` | PASS |
| `TestTickets_HashStorageNoPlaintext` | `internal/realtime/realtime_test.go` | YES | `docs/verification/final/TestTickets_HashStorageNoPlaintext.log` | PASS |
| `TestServer_TimeoutConfig` | `cmd/server/server_test.go` | YES | `docs/verification/final/TestServer_TimeoutConfig.log` | PASS |
| `npm test` (idempotency & realtime suite) | `frontend/src/lib/__tests__/*.test.mjs` | YES | `docs/verification/final/npm_test.log` | PASS (13/13) |
| `govulncheck` | Go toolchain + dependencies | YES | `docs/verification/final/govulncheck.log` | PASS (0 vulnerabilities) |
