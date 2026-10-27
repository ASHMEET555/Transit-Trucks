package app

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// TestCompulsory_Phase5_SLOAndLatencyVerification validates the Phase 5 SLO targets:
// 1. Availability (non-5xx) >= 99.5%
// 2. p95 Read Latency < 300ms
// 3. p95 Write Latency < 500ms
// 4. 5xx rate < 0.5%
// 5. Event delivery latency after commit p95 < 2s
// 6. Graceful degradation under 3x concurrency spike
func TestCompulsory_Phase5_SLOAndLatencyVerification(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	cfg := Config{
		Secret:         "phase5-slo-verification-test-secret-32-chars",
		AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		Env:            "local",
		CookieDomain:   "localhost",
		JobSecret:      "test-job-secret",
		DatabaseURL:    dsn,
	}

	ctx := context.Background()
	application := New(db, cfg)
	application.Start(ctx)
	defer application.Close()

	select {
	case <-application.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for application hub ready")
	}

	server := httptest.NewServer(application)
	defer server.Close()

	// Seed test admin, vehicle, and driver
	adminEmail := "phase5-slo-admin@transitops.com"
	pwHash, _ := auth.HashPassword("AdminPass123!")
	_, err := db.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000088', $1, $2, 'Admin', true, now(), now())
		ON CONFLICT (email) DO UPDATE SET is_active = true`, adminEmail, pwHash)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	_, _ = db.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ('VH-1001', 'Model-X', 'Van', 5000, 1000, 20000, 'Available', true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'Available', is_active = true`)
	_, _ = db.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ('DL-1001', 'SLO Driver', 'Light', '2029-01-01', '+1-555-0199', 95.0, 'Available', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'Available', is_active = true`)

	token, _, err := auth.CreateSession(ctx, db, adminEmail, "127.0.0.1", "slo-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// -------------------------------------------------------------
	// Part A: Event Delivery Latency Test (p95 < 2s)
	// -------------------------------------------------------------
	ticketReq, _ := http.NewRequest("POST", server.URL+"/api/v1/realtime/ticket", nil)
	ticketReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	ticketResp, err := http.DefaultClient.Do(ticketReq)
	if err != nil || ticketResp.StatusCode != 200 {
		t.Fatalf("ticket request failed: status %v, err %v", ticketResp.StatusCode, err)
	}
	var ticketBody struct {
		Ticket string `json:"ticket"`
	}
	_ = json.NewDecoder(ticketResp.Body).Decode(&ticketBody)
	ticketResp.Body.Close()

	wsURL := "ws" + server.URL[len("http"):] + "/api/v1/ws?ticket=" + ticketBody.Ticket
	wsHeader := http.Header{"Origin": []string{"http://localhost:3000"}}
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer wsConn.Close()

	var eventLatencies []time.Duration
	for i := 0; i < 5; i++ {
		tBeforeCommit := time.Now()
		idem := uuid.New().String()
		draftPayload := []byte(`{"vehicle_reg":"VH-1001","driver_license":"DL-1001","source_name":"New York Central Hub","destination_name":"Chicago Logistics Hub","cargo_weight":300}`)
		draftReq, _ := http.NewRequest("POST", server.URL+"/api/v1/trips", bytes.NewReader(draftPayload))
		draftReq.Header.Set("Content-Type", "application/json")
		draftReq.Header.Set("Idempotency-Key", idem)
		draftReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})

		draftResp, err := http.DefaultClient.Do(draftReq)
		if err != nil {
			t.Fatalf("post draft: %v", err)
		}
		draftResp.Body.Close()

		_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			t.Fatalf("read event failed: %v", err)
		}
		deliveryLatency := time.Since(tBeforeCommit)
		eventLatencies = append(eventLatencies, deliveryLatency)

		var ev map[string]interface{}
		_ = json.Unmarshal(msg, &ev)
	}

	sort.Slice(eventLatencies, func(i, j int) bool { return eventLatencies[i] < eventLatencies[j] })
	p95Event := eventLatencies[int(float64(len(eventLatencies))*0.95)]
	t.Logf("Event delivery latency: min=%v, max=%v, p95=%v", eventLatencies[0], eventLatencies[len(eventLatencies)-1], p95Event)
	if p95Event > 2*time.Second {
		t.Errorf("SLO violation: Event delivery p95 (%v) exceeded 2s limit", p95Event)
	}

	// -------------------------------------------------------------
	// Part B: Steady Concurrent Read/Write Workload
	// -------------------------------------------------------------
	numVUs := 15
	reqsPerVU := 20
	var readDurations []time.Duration
	var writeDurations []time.Duration
	var mu sync.Mutex
	var totalReqs int64
	var err5xxCount int64

	var wg sync.WaitGroup
	for vu := 0; vu < numVUs; vu++ {
		wg.Add(1)
		go func(vuID int) {
			defer wg.Done()
			var lReads []time.Duration
			var lWrites []time.Duration

			for reqIdx := 0; reqIdx < reqsPerVU; reqIdx++ {
				// Read 1: GET /api/v1/fleet/vehicles
				r1, _ := http.NewRequest("GET", server.URL+"/api/v1/fleet/vehicles", nil)
				r1.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				t0 := time.Now()
				resp1, err1 := http.DefaultClient.Do(r1)
				lReads = append(lReads, time.Since(t0))
				atomic.AddInt64(&totalReqs, 1)
				if err1 != nil || resp1.StatusCode >= 500 {
					atomic.AddInt64(&err5xxCount, 1)
				}
				if resp1 != nil {
					resp1.Body.Close()
				}

				// Read 2: GET /api/v1/dashboard/kpis
				r2, _ := http.NewRequest("GET", server.URL+"/api/v1/dashboard/kpis", nil)
				r2.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				t1 := time.Now()
				resp2, err2 := http.DefaultClient.Do(r2)
				lReads = append(lReads, time.Since(t1))
				atomic.AddInt64(&totalReqs, 1)
				if err2 != nil || resp2.StatusCode >= 500 {
					atomic.AddInt64(&err5xxCount, 1)
				}
				if resp2 != nil {
					resp2.Body.Close()
				}

				// Write: POST /api/v1/trips (Draft)
				idemKey := uuid.New().String()
				draftPayload := []byte(`{"vehicle_reg":"VH-1002","driver_license":"DL-1005","source_name":"New York Central Hub","destination_name":"Chicago Logistics Hub","cargo_weight":200}`)
				r3, _ := http.NewRequest("POST", server.URL+"/api/v1/trips", bytes.NewReader(draftPayload))
				r3.Header.Set("Content-Type", "application/json")
				r3.Header.Set("Idempotency-Key", idemKey)
				r3.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				t2 := time.Now()
				resp3, err3 := http.DefaultClient.Do(r3)
				lWrites = append(lWrites, time.Since(t2))
				atomic.AddInt64(&totalReqs, 1)
				if err3 != nil || resp3.StatusCode >= 500 {
					atomic.AddInt64(&err5xxCount, 1)
				}
				if resp3 != nil {
					resp3.Body.Close()
				}

				time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
			}

			mu.Lock()
			readDurations = append(readDurations, lReads...)
			writeDurations = append(writeDurations, lWrites...)
			mu.Unlock()
		}(vu)
	}

	wg.Wait()

	sort.Slice(readDurations, func(i, j int) bool { return readDurations[i] < readDurations[j] })
	sort.Slice(writeDurations, func(i, j int) bool { return writeDurations[i] < writeDurations[j] })

	p95Read := readDurations[int(float64(len(readDurations))*0.95)]
	p95Write := writeDurations[int(float64(len(writeDurations))*0.95)]
	err5xxRate := float64(err5xxCount) / float64(totalReqs) * 100.0
	availRate := 100.0 - err5xxRate

	t.Logf("Steady State SLO Results (Total Reqs: %d):", totalReqs)
	t.Logf("  Availability: %.2f%% (target: >= 99.50%%)", availRate)
	t.Logf("  5xx Rate: %.2f%% (target: < 0.50%%)", err5xxRate)
	t.Logf("  p95 Read Latency: %v (target: < 300ms)", p95Read)
	t.Logf("  p95 Write Latency: %v (target: < 500ms)", p95Write)

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

	// -------------------------------------------------------------
	// Part C: 3x Spike Test (Degrades Gracefully)
	// -------------------------------------------------------------
	spikeVUs := 45
	var spikeErrors int64
	var spikeTotal int64
	var spikeWg sync.WaitGroup

	for vu := 0; vu < spikeVUs; vu++ {
		spikeWg.Add(1)
		go func() {
			defer spikeWg.Done()
			for i := 0; i < 5; i++ {
				req, _ := http.NewRequest("GET", server.URL+"/api/v1/dashboard/kpis", nil)
				req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				resp, err := http.DefaultClient.Do(req)
				atomic.AddInt64(&spikeTotal, 1)
				if err != nil || resp.StatusCode >= 500 {
					atomic.AddInt64(&spikeErrors, 1)
				}
				if resp != nil {
					resp.Body.Close()
				}
			}
		}()
	}
	spikeWg.Wait()

	spikeErrRate := float64(spikeErrors) / float64(spikeTotal) * 100.0
	t.Logf("3x Spike Test Results (Total Reqs: %d, Errors: %d, Error Rate: %.2f%%)", spikeTotal, spikeErrors, spikeErrRate)
	if spikeErrRate >= 1.0 {
		t.Errorf("Spike degradation unacceptable: Error rate (%.2f%%) exceeded 1.0%%", spikeErrRate)
	}
}
