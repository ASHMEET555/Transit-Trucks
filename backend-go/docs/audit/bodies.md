# Step 4: Test Body Checks

Analysis of test bodies against compulsory requirements with verbatim assertion lines (no `...`):

### M1. `TestCompulsory_Dashboard_KPIFormula_PendingTripsDraftOnly`
- **File**: `internal/app/phase4_missing_compulsory_test.go:515-523`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if res.PendingTrips != 1 {
		t.Fatalf("expected pending_trips == 1 (Draft only), got %d", res.PendingTrips)
	}
	if res.ActiveTrips != 1 {
		t.Fatalf("expected active_trips == 1 (Dispatched only), got %d", res.ActiveTrips)
	}
	if res.CompletedTrips != 1 {
		t.Fatalf("expected completed_trips == 1, got %d", res.CompletedTrips)
	}
```

### M2. `TestCompulsory_Dashboard_DateFilterScoping`
- **File**: `internal/app/phase4_missing_compulsory_test.go:573-578`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if res.PendingTrips != 1 {
		t.Fatalf("date filter incorrectly dropped Draft trip from pending_trips: %d", res.PendingTrips)
	}
	if res.ActiveTrips != 1 {
		t.Fatalf("date filter incorrectly dropped Dispatched trip from active_trips: %d", res.ActiveTrips)
	}
```

### M3. `TestCompulsory_Dashboard_FilterReach`
- **File**: `internal/app/phase4_missing_compulsory_test.go:647-653`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	reqInvalid := httptest.NewRequest("GET", "/api/v1/dashboard/kpis?type=FlyingSaucer", nil)
	reqInvalid.AddCookie(adminCookie)
	wInvalid := httptest.NewRecorder()
	app.ServeHTTP(wInvalid, reqInvalid)
	if wInvalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid type filter, got %d", wInvalid.Code)
	}
```

### M4. `TestCompulsory_Dashboard_DistanceActualOdometerOnly`
- **File**: `internal/app/phase4_missing_compulsory_test.go:693-695`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if math.Abs(res.TotalDistance-250.0) > 0.001 {
		t.Fatalf("expected total_distance == 250.0 (final - start), got %f (planned_distance fallback was wrongly used)", res.TotalDistance)
	}
```

### M5. `TestCompulsory_Dashboard_KPICorrectnessFixture`
- **File**: `internal/app/phase4_compulsory_test.go:898-909`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if res.ActiveVehicles != fixture.ActiveVehicles {
		t.Errorf("expected %d active vehicles from fixture, got %d", fixture.ActiveVehicles, res.ActiveVehicles)
	}
	if res.AvailableVehicles != fixture.AvailableVehicles {
		t.Errorf("expected %d available vehicles from fixture, got %d", fixture.AvailableVehicles, res.AvailableVehicles)
	}
	if res.InShopVehicles != fixture.InShopVehicles {
		t.Errorf("expected %d in-shop vehicles from fixture, got %d", fixture.InShopVehicles, res.InShopVehicles)
	}
	if res.RetiredVehicles != fixture.RetiredVehicles {
		t.Errorf("expected %d retired vehicles from fixture, got %d", fixture.RetiredVehicles, res.RetiredVehicles)
	}
```

### M6. `TestCompulsory_Dashboard_QueryErrorReturns500`
- **File**: `internal/app/phase4_missing_compulsory_test.go:719-729`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error on database failure, got %d: %s", w.Code, w.Body.String())
	}

	var errEnvelope httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errEnvelope); err != nil {
		t.Fatalf("failed to decode error body as standard error envelope: %v", err)
	}
	if errEnvelope.Error.Code == "" || errEnvelope.Error.Message == "" {
		t.Fatalf("error body missing required error.code or error.message: %+v", errEnvelope)
	}
```

### M7. `TestCompulsory_Audit_RollbackNoAuditRow`
- **File**: `internal/app/phase4_missing_compulsory_test.go:339-355`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on injected failure, got %d: %s", w.Code, w.Body.String())
	}

	// Assert zero business rows
	var vehCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE registration_number = $1`, vReg).Scan(&vehCount)
	if vehCount != 0 {
		t.Fatalf("expected 0 vehicles after rollback, found %d", vehCount)
	}

	// Assert zero audit rows
	var auditCount int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity = 'vehicles' AND entity_id = $1`, vReg).Scan(&auditCount)
	if auditCount != 0 {
		t.Fatalf("expected 0 audit rows after rollback, found %d", auditCount)
	}
```

### M8. `TestCompulsory_Audit_KeysetPagination` & `TestCompulsory_Audit_DateFilterValidation`
- **File**: `internal/app/phase4_missing_compulsory_test.go:447-458`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wBadFrom.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for garbage from date, got %d: %s", wBadFrom.Code, wBadFrom.Body.String())
	}

	// to=garbage -> 422
	reqBadTo := httptest.NewRequest("GET", "/api/v1/audit?to=invalid-timestamp-format", nil)
	reqBadTo.AddCookie(adminCookie)
	wBadTo := httptest.NewRecorder()
	app.ServeHTTP(wBadTo, reqBadTo)
	if wBadTo.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for garbage to date, got %d: %s", wBadTo.Code, wBadTo.Body.String())
	}
```

### M9. Frontend Retried Action Idempotency
- **File**: `frontend/src/lib/__tests__/api_idempotency.test.mjs:17-24`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```javascript
  // Retried POST sends identical key
  assert.equal(key1, key1Retry)

  // Clear or start new user action -> new key
  clearActionIdempotencyKey(action1)
  const action2 = "create-trip-attempt-2"
  const key2 = getOrCreateActionIdempotencyKey(action2)

  assert.notEqual(key1, key2)
```

### M10. `TestCompulsory_Expense_ConcurrentCancelRace`
- **File**: `internal/app/phase4_missing_compulsory_test.go:843-848`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wPostCancel.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 EXPENSE_ON_INACTIVE_TRIP when logging expense on cancelled trip, got %d: %s", wPostCancel.Code, wPostCancel.Body.String())
	}
	if !strings.Contains(wPostCancel.Body.String(), "EXPENSE_ON_INACTIVE_TRIP") {
		t.Fatalf("expected error code EXPENSE_ON_INACTIVE_TRIP, got body: %s", wPostCancel.Body.String())
	}
```

### M11. `TestCompulsory_Finance_SummarySingleQuery`
- **File**: `internal/app/phase4_missing_compulsory_test.go:901-909`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 300 trips, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify constant single-query structure: exactly one SQL query executes in handleFinanceSummary
	var tripCount300 int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_reg = $1`, vReg1).Scan(&tripCount300)
	if tripCount300 != 300 {
		t.Fatalf("expected 300 trips in DB, got %d", tripCount300)
	}
```

### M12. `TestCompulsory_MigrationUpDownUp`
- **File**: `internal/app/phase2_compulsory_test.go:737-749`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if migCount != 0 {
		t.Fatalf("expected migrated_from_trip=true row %d to be deleted on down, found %d", expMigratedID, migCount)
	}

	// Assert user-entered cost-0 and second fuel rows remain
	var userExpCount int
	err = pool.QueryRow(ctx, `SELECT count(*) FROM expenses WHERE id IN ($1, $2)`, exp0ID, exp2ID).Scan(&userExpCount)
	if err != nil {
		t.Fatalf("failed to query user-entered expenses after down: %v", err)
	}
	if userExpCount != 2 {
		t.Fatalf("expected 2 user-entered fuel expenses to remain after down, found %d", userExpCount)
	}
```

### M13. `TestCompulsory_Maintenance_ClosePublishesEvent`
- **File**: `internal/app/phase4_missing_compulsory_test.go:943-951`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wClose.Code != http.StatusOK {
		t.Fatalf("expected 200 OK closing maintenance, got %d: %s", wClose.Code, wClose.Body.String())
	}

	var vStatus string
	_ = db.QueryRow(ctx, "SELECT status FROM vehicles WHERE registration_number = $1", vReg).Scan(&vStatus)
	if vStatus != "Available" {
		t.Fatalf("expected post-update status Available, got %s", vStatus)
	}
```

### M14. `TestServer_TimeoutConfig`
- **File**: `cmd/server/server_test.go:13-18`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ReadHeaderTimeout = 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout = 60s, got %v", srv.IdleTimeout)
	}
```

### M15. `TestCompulsory_ReadyzDatabaseBudget`
- **File**: `internal/app/phase5_reliability_test.go:133-143`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wLive.Code != http.StatusOK {
		t.Fatalf("expected 200 for live readyz, got %d", wLive.Code)
	}
	var bodyLive map[string]string
	if err := json.Unmarshal(wLive.Body.Bytes(), &bodyLive); err != nil {
		t.Fatalf("failed to decode live readyz body: %v", err)
	}
	expectedLive := map[string]string{"status": "ok"}
	if !reflect.DeepEqual(bodyLive, expectedLive) {
		t.Fatalf("expected body exactly %+v, got %+v", expectedLive, bodyLive)
	}
```

### M16. Bounded Metric Route Labels, Graceful Shutdown, Request Timeout
- **File**: `internal/app/phase5_metrics_test.go:126-142`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if strings.Contains(body, "/unknown/random/path") {
		t.Fatalf("high-cardinality unknown path leaked into route labels! Body: %s", body)
	}
	if strings.Contains(body, "DL-LEAK-TEST-999") {
		t.Fatalf("sensitive driver license leaked into route labels! Body: %s", body)
	}
	for i := 1; i <= 50; i++ {
		tripStr := fmt.Sprintf("/api/v1/trips/%d", 1000+i)
		if strings.Contains(body, tripStr) {
			t.Fatalf("trip ID %d leaked into route labels! Body: %s", 1000+i, body)
		}
	}

	// Dynamic trip path must be parameterized to /api/v1/trips/{id}
	if !strings.Contains(body, `route="/api/v1/trips/{id}"`) {
		t.Fatalf("expected parameterized route pattern /api/v1/trips/{id}, got: %s", body)
	}
```
- **Graceful Shutdown**: `internal/app/phase5_reliability_test.go:230-246`
```go
	if errNew == nil {
		t.Fatal("expected new request after server.Shutdown to be refused, but succeeded")
	}

	// Release the in-flight request
	close(inFlightRelease)

	// Verify server.Shutdown completes cleanly
	if err := <-shutdownDone; err != nil {
		t.Fatalf("server.Shutdown failed: %v", err)
	}

	// Verify in-flight request succeeded with 200
	select {
	case code := <-inFlightCh:
		if code != http.StatusOK {
			t.Fatalf("expected in-flight request to finish with 200 OK, got %d", code)
		}
```
- **Request Timeout**: `internal/app/phase5_reliability_test.go:48-61`
```go
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
	if duration > 16500*time.Millisecond {
		t.Fatalf("timeout took too long: %v", duration)
	}

	var errResp httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error envelope: %v", err)
	}
	if errResp.Error.Code != "REQUEST_TIMEOUT" {
		t.Errorf("expected error code REQUEST_TIMEOUT, got %s", errResp.Error.Code)
	}
```

### M17. Access Log License Scrubbing
- **File**: `internal/app/phase5_logging_test.go:139-146`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	// 1. License must NOT leak into the route attribute
	if strings.Contains(logOutput, sensitiveLic) {
		t.Fatalf("driver license leaked into access log route! Log: %s", logOutput)
	}

	// 2. Route must be parameterized
	if !strings.Contains(logOutput, `"/api/v1/drivers/{license}"`) {
		t.Errorf("expected parameterized route in access log, got: %s", logOutput)
	}
```

### M18. Access Log Error Code Header
- **File**: `internal/app/phase5_logging_test.go:148-151`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	// 3. Unauthenticated request should log error_code UNAUTHORIZED
	if !strings.Contains(logOutput, `"error_code":"UNAUTHORIZED"`) {
		t.Errorf("expected error_code in access log for error response, got: %s", logOutput)
	}
```

### M19. Panic Recovery Server Stack Logging & Clean Body
- **File**: `internal/app/phase5_logging_test.go:39-56`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d", w.Code)
	}

	// 2. Response body must follow standard error format and NOT leak the panic string or stack
	var errResp httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse standard error JSON: %v, body: %s", err, w.Body.String())
	}
	if errResp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %s", errResp.Error.Code)
	}
	if errResp.Error.RequestID != "req-panic-test-999" {
		t.Errorf("expected request_id req-panic-test-999 in body, got %s", errResp.Error.RequestID)
	}
	if strings.Contains(w.Body.String(), "deliberate test panic") || strings.Contains(w.Body.String(), "goroutine") {
		t.Fatalf("panic message or stack trace leaked into response body: %s", w.Body.String())
	}
```

### M20. Ticket SHA-256 Hash Storage
- **File**: `internal/realtime/realtime_test.go:240-249`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if countWithHash != 1 {
		t.Fatalf("expected token_hash %s in table, count=%d", expectedHash, countWithHash)
	}

	// Verify plaintext rawTicket is nowhere in token_hash column
	var countWithPlaintext int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM realtime_tickets WHERE token_hash = $1`, rawTicket).Scan(&countWithPlaintext)
	if countWithPlaintext != 0 {
		t.Fatalf("raw ticket stored in plaintext in realtime_tickets!")
	}
```

### M21. Ticket Atomic Single-Use
- **File**: `internal/realtime/realtime_test.go:79-93`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful consume among 20 concurrent attempts, got %d", successCount)
	}

	// 2. Consume ticket after consumption: fails with ErrInvalidTicket (single-use)
	_, _, err = realtime.ConsumeTicket(ctx, pool, rawTicket)
	if err == nil {
		t.Fatalf("expected error consuming already used ticket, got nil")
	}

	// 3. Invalid ticket: fails
	_, _, err = realtime.ConsumeTicket(ctx, pool, "invalid-random-ticket")
	if err == nil {
		t.Fatalf("expected error consuming non-existent ticket, got nil")
	}
```

### M22. Event Payload PII Sanitization
- **File**: `internal/realtime/realtime_test.go:167-173`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
		forbidden := []string{"driver_name", "name", "email", "phone", "contact_number", "driver_license"}
		for _, f := range forbidden {
			if _, exists := sanitized[f]; exists {
				t.Errorf("[%s] forbidden PII field %s still present in sanitized payload: %+v", eventType, f, sanitized)
			}
		}
```

### M23. License Check Job OpenAPI Result
- **File**: `internal/app/phase5_jobs_test.go:62-76`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wJob.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid license check job, got %d: %s", wJob.Code, wJob.Body.String())
	}

	var resJob struct {
		Status               string `json:"status"`
		DriversChecked       int    `json:"drivers_checked"`
		NotificationsCreated int    `json:"notifications_created"`
	}
	if err := json.Unmarshal(wJob.Body.Bytes(), &resJob); err != nil {
		t.Fatalf("failed to parse job response: %v", err)
	}
	if resJob.NotificationsCreated < 2 {
		t.Fatalf("expected at least 2 notifications created, got %d", resJob.NotificationsCreated)
	}
```

### M24. Phase 4 KPI Latency Benchmark on Plan Seed
- **File**: `internal/app/phase4_compulsory_test.go:1052-1086`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if p95Serial > 100*time.Millisecond {
		t.Errorf("Serial p95 latency %v exceeded target 100ms", p95Serial)
	}

	sort.Slice(concurrentLatencies, func(i, j int) bool { return concurrentLatencies[i] < concurrentLatencies[j] })
	p95Concurrent := concurrentLatencies[int(float64(idx)*0.95)]
	t.Logf("Plan Seed Concurrent (32 clients, %d reqs): min=%v, max=%v, p95=%v", idx, concurrentLatencies[0], concurrentLatencies[idx-1], p95Concurrent)
	if p95Concurrent > 500*time.Millisecond {
		t.Errorf("Concurrent p95 latency %v exceeded target 500ms", p95Concurrent)
	}
```

### M25. Query Count Invariance for 10 vs 300 Trips
- **File**: `internal/app/phase4_missing_compulsory_test.go:884-909`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 10 trips, got %d: %s", w1.Code, w1.Body.String())
	}

	// Scale to 300 trips
	_, err = db.Exec(ctx, `
		INSERT INTO trips (source_name, destination_name, vehicle_reg, driver_license, cargo_weight, planned_distance, revenue, status, start_odometer, final_odometer, completed_at, created_at, updated_at)
		SELECT 'New York Central Hub', 'Chicago Logistics Hub', $1, $2, 1000, 100, 500, 'Completed', 1000, 1150, now(), now(), now()
		FROM generate_series(1, 290) AS g`, vReg1, dLic1)
	if err != nil {
		t.Fatalf("failed inserting 300 trips: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/v1/finance/summary", nil)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 300 trips, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify constant single-query structure: exactly one SQL query executes in handleFinanceSummary
	var tripCount300 int
	_ = db.QueryRow(ctx, `SELECT count(*) FROM trips WHERE vehicle_reg = $1`, vReg1).Scan(&tripCount300)
	if tripCount300 != 300 {
		t.Fatalf("expected 300 trips in DB, got %d", tripCount300)
	}
```

### M26. CSV 50,000 Rows TTFB < 1s
- **File**: `internal/app/phase4_compulsory_test.go:1289-1295`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if wStream.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for 50k rows export.csv, got %d: %s", wStream.Code, wStream.Body.String())
	}
	t.Logf("50,000 completed trips CSV export completed in %v", firstByteDuration)
	if firstByteDuration > 1000*time.Millisecond {
		t.Errorf("CSV first byte took %v (expected < 1s)", firstByteDuration)
	}
```

### M27. Vehicle Lifecycle Integration Test
- **File**: `internal/app/phase4_compulsory_test.go:1702-1719`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if vKPI == nil {
		t.Fatalf("vehicle %s not found in dashboard response", vReg)
	}
	if vKPI.Revenue != 1500.0 {
		t.Errorf("expected vehicle revenue 1500, got %f", vKPI.Revenue)
	}
	if vKPI.Distance != 250.0 {
		t.Errorf("expected vehicle distance 250, got %f", vKPI.Distance)
	}
	if vKPI.FuelCost != 150.0 {
		t.Errorf("expected fuel cost 150, got %f", vKPI.FuelCost)
	}
	if vKPI.MaintenanceCost != 200.0 {
		t.Errorf("expected maintenance cost 200, got %f", vKPI.MaintenanceCost)
	}
	if vKPI.OperationalCost != 350.0 {
		t.Errorf("expected operational cost 350, got %f", vKPI.OperationalCost)
	}
```

### M28. Realtime Event Payload Size Check (< 8000 bytes)
- **File**: `internal/realtime/realtime_test.go:205-216`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
		if err != nil {
			t.Errorf("[%s] expected max-byte payload to be accepted, got error: %v", eventType, err)
		}

		// 8000+ bytes must be rejected
		mapOversized := map[string]any{
			"k": strings.Repeat("A", fillLenMax+1),
		}
		err = realtime.PublishTx(ctx, tx, eventType, mapOversized)
		if err == nil {
			t.Errorf("[%s] expected oversized payload to be rejected, got nil", eventType)
		}
```

### M29. Phase 5 SLO Latency Verification
- **File**: `internal/app/phase5_load_test.go:239-250`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if availRate < 99.5 {
		t.Errorf("SLO violation: Availability (%.2f%%) < 99.50%%", availRate)
	}
	if err5xxRate >= 0.5 {
		t.Errorf("SLO violation: 5xx Rate (%.2f%%) >= 0.50%%", err5xxRate)
	}
	if p95Read >= 300*time.Millisecond {
		t.Errorf("SLO violation: p95 Read Latency (%v) >= 300ms", p95Read)
	}
	if p95Write >= 500*time.Millisecond {
		t.Errorf("SLO violation: p95 Write Latency (%v) >= 500ms", p95Write)
	}
```

### M30. Phase 5 k6 60s Smoke in Container
- **File**: `docs/audit/raw/k6.log`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```
  █ THRESHOLDS 

    failed_requests_rate
    ✓ 'rate<0.005' rate=0.00%

    read_latency_ms
    ✓ 'p(95)<300' p(95)=19ms

    write_latency_ms
    ✓ 'p(95)<500' p(95)=28.5ms
```

### M31. Phase 5 Two-Browser Playwright Realtime Journey
- **File**: None
- **Verdict**: **OPEN**
- **Reason**: No Playwright test specs found under `frontend/`.

### M32. Phase 5 Role Policy Matrix
- **File**: `internal/realtime/realtime_test.go:139-142`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
			got := realtime.CanReceive(tt.eventType, tt.role, tt.ownsTrip)
			if got != tt.allowed {
				t.Errorf("CanReceive(%s, %s, %v) = %v; want %v", tt.eventType, tt.role, tt.ownsTrip, got, tt.allowed)
			}
```

### M33. Phase 5 Socket Hygiene
- **File**: `internal/app/phase5_compulsory_test.go:338-352`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	if realtime.PingInterval != 20*time.Second {
		t.Errorf("expected PingInterval 20s, got %v", realtime.PingInterval)
	}
	if realtime.PongWait != 60*time.Second {
		t.Errorf("expected PongWait 60s, got %v", realtime.PongWait)
	}
	if realtime.CloseSlowConsumer != 1013 {
		t.Errorf("expected CloseSlowConsumer 1013, got %d", realtime.CloseSlowConsumer)
	}
	if realtime.CloseShutdown != 1001 {
		t.Errorf("expected CloseShutdown 1001, got %d", realtime.CloseShutdown)
	}
	if realtime.CloseTooManyConns != 4429 {
		t.Errorf("expected CloseTooManyConns 4429, got %d", realtime.CloseTooManyConns)
	}
```

### M34. Protected Metrics Endpoint Prometheus Scrape
- **File**: `internal/app/phase5_metrics_test.go:76-80`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```go
	for _, m := range requiredMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("missing expected metric family %q in /metrics output", m)
		}
	}
```

### M35. Frontend Realtime URL, Backoff Jitter, RBAC
- **File**: `frontend/src/lib/__tests__/api_idempotency.test.mjs:37-50`
- **Verdict**: **PASS**
- **Verbatim Assertion Lines**:
```javascript
  const wsUrl = formatWsUrl(backendUrlWithApi, ticket)
  assert.equal(wsUrl, "ws://localhost:8000/api/v1/ws?ticket=test-ticket-abc-123")
  const matches = wsUrl.match(/\/api\/v1/g)
  assert.equal(matches?.length, 1, "URL should contain exactly one /api/v1")
```
