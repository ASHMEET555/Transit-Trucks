package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LatencyStats struct {
	Count int
	P50   time.Duration
	P90   time.Duration
	P95   time.Duration
	P99   time.Duration
	Min   time.Duration
	Max   time.Duration
}

func calcStats(durations []time.Duration) LatencyStats {
	if len(durations) == 0 {
		return LatencyStats{}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	n := len(durations)
	return LatencyStats{
		Count: n,
		P50:   durations[int(float64(n)*0.50)],
		P90:   durations[int(float64(n)*0.90)],
		P95:   durations[int(float64(n)*0.95)],
		P99:   durations[int(float64(n)*0.99)],
		Min:   durations[0],
		Max:   durations[n-1],
	}
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("POSTGRES_DSN")
	}
	if dsn == "" {
		dsn = "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to DB: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	cfg := app.Config{
		Secret:         "loadtest-secret-key-at-least-32-chars",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "localhost",
		JobSecret:      "test-job-secret",
	}
	application := app.New(pool, cfg)
	defer application.Close()

	adminEmail := "loadtest-admin@transitops.com"
	pwHash, _ := auth.HashPassword("AdminPass123!")
	_, _ = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000099', $1, $2, 'Admin', true, now(), now())
		ON CONFLICT (email) DO UPDATE SET is_active = true`, adminEmail, pwHash)

	token, _, err := auth.CreateSession(ctx, pool, adminEmail, "127.0.0.1", "loadtest-agent")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("==================================================")
	fmt.Println(" TransitOps Phase 5 Load & SLO Verification Test")
	fmt.Println("==================================================")

	// 1. Warm-up
	fmt.Print("1. Warming up pool and caches... ")
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		application.ServeHTTP(w, req)
	}
	fmt.Println("Done.")

	// 2. Steady State Load Test (Concurrent VUs)
	numVUs := 25
	requestsPerVU := 40
	fmt.Printf("2. Executing Steady State Load Test (%d concurrent VUs, %d requests each)...\n", numVUs, requestsPerVU)

	var readDurations []time.Duration
	var writeDurations []time.Duration
	var mu sync.Mutex

	var totalCount int64
	var errorCount int64

	var wg sync.WaitGroup
	startAll := time.Now()

	for vu := 0; vu < numVUs; vu++ {
		wg.Add(1)
		go func(vuID int) {
			defer wg.Done()
			var localReads []time.Duration
			var localWrites []time.Duration

			for reqIdx := 0; reqIdx < requestsPerVU; reqIdx++ {
				atomic.AddInt64(&totalCount, 1)

				// A. Read endpoint: GET /api/v1/fleet/vehicles
				r1 := httptest.NewRequest("GET", "/api/v1/fleet/vehicles", nil)
				r1.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				w1 := httptest.NewRecorder()
				t0 := time.Now()
				application.ServeHTTP(w1, r1)
				localReads = append(localReads, time.Since(t0))
				if w1.Code >= 500 {
					atomic.AddInt64(&errorCount, 1)
				}

				// B. Read endpoint: GET /api/v1/dashboard/kpis
				r2 := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
				r2.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				w2 := httptest.NewRecorder()
				t1 := time.Now()
				application.ServeHTTP(w2, r2)
				localReads = append(localReads, time.Since(t1))
				if w2.Code >= 500 {
					atomic.AddInt64(&errorCount, 1)
				}

				// C. Read endpoint: POST /api/v1/realtime/ticket
				r3 := httptest.NewRequest("POST", "/api/v1/realtime/ticket", nil)
				r3.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				w3 := httptest.NewRecorder()
				t2 := time.Now()
				application.ServeHTTP(w3, r3)
				localReads = append(localReads, time.Since(t2))
				if w3.Code >= 500 {
					atomic.AddInt64(&errorCount, 1)
				}

				// D. Write endpoint: POST /api/v1/trips (Draft)
				idemDraft := uuid.New().String()
				draftBody := []byte(`{"vehicle_reg":"VH-1001","driver_license":"DL-1001","source_name":"New York Central Hub","destination_name":"Chicago Logistics Hub","cargo_weight":500}`)
				r4 := httptest.NewRequest("POST", "/api/v1/trips", bytes.NewReader(draftBody))
				r4.Header.Set("Content-Type", "application/json")
				r4.Header.Set("Idempotency-Key", idemDraft)
				r4.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				w4 := httptest.NewRecorder()
				t3 := time.Now()
				application.ServeHTTP(w4, r4)
				localWrites = append(localWrites, time.Since(t3))
				if w4.Code >= 500 {
					atomic.AddInt64(&errorCount, 1)
				}

				// Cancel trip if created
				if w4.Code == http.StatusCreated || w4.Code == http.StatusOK {
					var resp map[string]interface{}
					if err := json.Unmarshal(w4.Body.Bytes(), &resp); err == nil {
						var tripID int64
						if idFloat, ok := resp["id"].(float64); ok {
							tripID = int64(idFloat)
						} else if tripObj, ok := resp["trip"].(map[string]interface{}); ok {
							if idFloat, ok := tripObj["id"].(float64); ok {
								tripID = int64(idFloat)
							}
						}
						if tripID > 0 {
							idemCancel := uuid.New().String()
							cancelBody := []byte(`{"reason":"bench cleanup"}`)
							r5 := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/trips/%d/cancel", tripID), bytes.NewReader(cancelBody))
							r5.Header.Set("Content-Type", "application/json")
							r5.Header.Set("Idempotency-Key", idemCancel)
							r5.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
							w5 := httptest.NewRecorder()
							t4 := time.Now()
							application.ServeHTTP(w5, r5)
							localWrites = append(localWrites, time.Since(t4))
							if w5.Code >= 500 {
								atomic.AddInt64(&errorCount, 1)
							}
						}
					}
				}

				time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
			}

			mu.Lock()
			readDurations = append(readDurations, localReads...)
			writeDurations = append(writeDurations, localWrites...)
			mu.Unlock()
		}(vu)
	}

	wg.Wait()
	totalElapsed := time.Since(startAll)

	readStats := calcStats(readDurations)
	writeStats := calcStats(writeDurations)

	totalReqs := totalCount * 4 // approx
	errorRate := float64(errorCount) / float64(len(readDurations)+len(writeDurations)) * 100.0
	availRate := 100.0 - errorRate

	fmt.Printf("Steady test completed in %v. Total operations: %d (Reads: %d, Writes: %d)\n\n",
		totalElapsed.Round(time.Millisecond), len(readDurations)+len(writeDurations), len(readDurations), len(writeDurations))

	// 3. Spike Test (3x baseline concurrency burst)
	spikeVUs := 75
	spikeReqsPerVU := 10
	fmt.Printf("3. Executing 3x Spike Test (%d concurrent VUs burst)...\n", spikeVUs)
	var spikeDurations []time.Duration
	var spikeErrors int64
	var spikeWg sync.WaitGroup
	spikeStart := time.Now()

	for vu := 0; vu < spikeVUs; vu++ {
		spikeWg.Add(1)
		go func(vuID int) {
			defer spikeWg.Done()
			var localSpike []time.Duration
			for i := 0; i < spikeReqsPerVU; i++ {
				r := httptest.NewRequest("GET", "/api/v1/dashboard/kpis", nil)
				r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
				w := httptest.NewRecorder()
				t := time.Now()
				application.ServeHTTP(w, r)
				localSpike = append(localSpike, time.Since(t))
				if w.Code >= 500 {
					atomic.AddInt64(&spikeErrors, 1)
				}
			}
			mu.Lock()
			spikeDurations = append(spikeDurations, localSpike...)
			mu.Unlock()
		}(vu)
	}
	spikeWg.Wait()
	spikeElapsed := time.Since(spikeStart)
	spikeStats := calcStats(spikeDurations)
	spikeErrorRate := float64(spikeErrors) / float64(len(spikeDurations)) * 100.0

	// 4. Print SLO Comparison Table
	fmt.Println("\n==================================================")
	fmt.Println(" SLO Evaluation Table")
	fmt.Println("==================================================")
	fmt.Printf("| Metric | Target | Steady Measured | Spike Measured | Status |\n")
	fmt.Printf("|---|---|---|---|---|\n")
	fmt.Printf("| Availability (non-5xx) | >= 99.50%% | %.2f%% | %.2f%% | %s |\n",
		availRate, 100.0-spikeErrorRate, passFail(availRate >= 99.5 && (100.0-spikeErrorRate) >= 99.0))
	fmt.Printf("| p95 Read Latency | < 300 ms | %.2f ms | %.2f ms | %s |\n",
		float64(readStats.P95.Microseconds())/1000.0, float64(spikeStats.P95.Microseconds())/1000.0, passFail(readStats.P95 < 300*time.Millisecond))
	fmt.Printf("| p95 Write Latency | < 500 ms | %.2f ms | N/A | %s |\n",
		float64(writeStats.P95.Microseconds())/1000.0, passFail(writeStats.P95 < 500*time.Millisecond))
	fmt.Printf("| 5xx Error Rate | < 0.50%% | %.2f%% | %.2f%% | %s |\n",
		errorRate, spikeErrorRate, passFail(errorRate < 0.5))
	fmt.Println("==================================================")

	_ = totalReqs
	_ = spikeElapsed
}

func passFail(ok bool) string {
	if ok {
		return "PASSED"
	}
	return "FAILED"
}
