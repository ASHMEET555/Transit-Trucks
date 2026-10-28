package realtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/app"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/auth"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
)

// TestCompulsory_EventDeliveryLatency verifies post-commit event delivery latency
// over WebSocket using client-measured timestamps (t0 = dispatch send, t1 = event receive).
// Note: This is a single-host measurement using the test process clock for both t0 and t1.
func TestCompulsory_EventDeliveryLatency(t *testing.T) {
	pool := getDB(t)
	defer pool.Close()
	ctx := context.Background()

	dsn := "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"

	cfg := app.Config{
		Secret:         "phase5-latency-verification-secret-32-chars",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "localhost",
		JobSecret:      "test-job-secret",
		DatabaseURL:    dsn,
	}

	application := app.New(pool, cfg)
	application.Start(ctx)
	defer application.Close()

	select {
	case <-application.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for application hub ready")
	}

	server := httptest.NewServer(application)
	defer server.Close()

	// Seed test users: Fleet Manager and Driver
	fmEmail := "fm.latency@transitops.com"
	drvEmail := "driver.latency@transitops.com"
	drvLic := "DL-LATENCY-999"
	vReg := "VH-LATENCY-999"

	pwHash, _ := auth.HashPassword("TestPass123!")

	_, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000091', $1, $2, 'Fleet Manager', true, now(), now())
		ON CONFLICT (email) DO UPDATE SET is_active = true`, fmEmail, pwHash)
	if err != nil {
		t.Fatalf("seed FM user: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role, driver_license, is_active, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000092', $1, $2, 'Driver', $3, true, now(), now())
		ON CONFLICT (email) DO UPDATE SET is_active = true, driver_license = $3`, drvEmail, pwHash, drvLic)
	if err != nil {
		t.Fatalf("seed Driver user: %v", err)
	}

	_, _ = pool.Exec(ctx, `INSERT INTO vehicles (registration_number, model, type, max_load_capacity, odometer, acquisition_cost, status, is_active, created_at, updated_at)
		VALUES ($1, 'Latency-Truck', 'Van', 5000, 1000, 20000, 'Available', true, now(), now())
		ON CONFLICT (registration_number) DO UPDATE SET status = 'Available', is_active = true`, vReg)

	_, _ = pool.Exec(ctx, `INSERT INTO drivers (license_number, name, license_category, license_expiry_date, contact_number, safety_score, status, is_active, created_at, updated_at)
		VALUES ($1, 'Latency Driver', 'Heavy', '2030-01-01', '+1-555-0999', 95.0, 'Available', true, now(), now())
		ON CONFLICT (license_number) DO UPDATE SET status = 'Available', is_active = true, license_category = 'Heavy'`, drvLic)

	// Issue sessions
	fmToken, _, err := auth.CreateSession(ctx, pool, fmEmail, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("create FM session: %v", err)
	}
	drvToken, _, err := auth.CreateSession(ctx, pool, drvEmail, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("create Driver session: %v", err)
	}

	// Request tickets via HTTP endpoint (ticket flow, not bypassed)
	getTicket := func(token string) string {
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/realtime/ticket", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("ticket request failed: status %v, err %v", resp.StatusCode, err)
		}
		defer resp.Body.Close()
		var res struct {
			Ticket string `json:"ticket"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&res)
		return res.Ticket
	}

	fmTicket := getTicket(fmToken)
	drvTicket := getTicket(drvToken)

	wsBase := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws?ticket="

	// Connect FM WebSocket client
	fmConn, _, err := websocket.DefaultDialer.Dial(wsBase+fmTicket, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("FM WS dial failed: %v", err)
	}
	defer fmConn.Close()

	// Connect Driver WebSocket client
	drvConn, _, err := websocket.DefaultDialer.Dial(wsBase+drvTicket, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("Driver WS dial failed: %v", err)
	}
	defer drvConn.Close()

	// Drain / buffer incoming events for Driver in background
	type incomingMsg struct {
		receivedAt time.Time
		event      realtime.Event
	}
	drvEvents := make(chan incomingMsg, 200)
	go func() {
		for {
			_, data, readErr := drvConn.ReadMessage()
			if readErr != nil {
				return
			}
			tRecv := time.Now()
			var ev realtime.Event
			if err := json.Unmarshal(data, &ev); err == nil {
				drvEvents <- incomingMsg{receivedAt: tRecv, event: ev}
			}
		}
	}()

	// Perform N=100 dispatches and record delivery latency
	N := 100
	samples := make([]time.Duration, 0, N)

	for i := 0; i < N; i++ {
		// Ensure previous dispatched trip is cleared so partial unique index allows dispatch
		_, _ = pool.Exec(ctx, `UPDATE trips SET status = 'Completed', final_odometer = 1200, completed_at = now() WHERE (vehicle_reg = $1 OR driver_license = $2) AND status = 'Dispatched'`, vReg, drvLic)
		_, _ = pool.Exec(ctx, `UPDATE vehicles SET status = 'Available' WHERE registration_number = $1`, vReg)
		_, _ = pool.Exec(ctx, `UPDATE drivers SET status = 'Available' WHERE license_number = $1`, drvLic)

		tripReqBody, _ := json.Marshal(map[string]any{
			"source_name":      "New York Central Hub",
			"destination_name": "Chicago Logistics Hub",
			"vehicle_reg":      vReg,
			"driver_license":   drvLic,
			"cargo_weight":     1200,
			"planned_distance": 150,
			"dispatch":         true,
		})

		req, _ := http.NewRequest("POST", server.URL+"/api/v1/trips", bytes.NewReader(tripReqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: fmToken})

		// Record t0 (client send time)
		t0 := time.Now()
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusCreated {
			respBody := ""
			if resp != nil {
				var b bytes.Buffer
				_, _ = b.ReadFrom(resp.Body)
				respBody = b.String()
			}
			t.Fatalf("[%d] dispatch request failed: status %v, err %v, body: %s", i, resp.StatusCode, err, respBody)
		}

		var tripCreated struct {
			ID int64 `json:"id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tripCreated)
		resp.Body.Close()
		targetTripID := tripCreated.ID

		// Await matching trip.dispatched event on Driver's WebSocket
		matched := false
		timer := time.NewTimer(3 * time.Second)
		for !matched {
			select {
			case msg := <-drvEvents:
				if msg.event.Type == realtime.EventTripDispatched {
					// Check trip_id match
					if tid, ok := msg.event.IDs["trip_id"]; ok {
						if fmt.Sprint(tid) == fmt.Sprint(targetTripID) {
							t1 := msg.receivedAt
							latency := t1.Sub(t0)
							samples = append(samples, latency)
							matched = true
						}
					}
				}
			case <-timer.C:
				t.Fatalf("[%d] timed out waiting for trip.dispatched event for trip %d", i, targetTripID)
			}
		}
		timer.Stop()
	}

	// Compute statistics
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	minLat := samples[0]
	medLat := samples[len(samples)/2]
	p95Lat := samples[int(float64(len(samples))*0.95)]
	maxLat := samples[len(samples)-1]

	t.Logf("Event Delivery Latency (N=%d samples): min=%v, median=%v, p95=%v, max=%v", N, minLat, medLat, p95Lat, maxLat)

	// Assert p95 < 2000 ms
	if p95Lat >= 2000*time.Millisecond {
		t.Fatalf("SLO violation: p95 event delivery latency %v >= 2000ms", p95Lat)
	}
}
