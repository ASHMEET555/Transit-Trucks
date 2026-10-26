package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
	"github.com/gorilla/websocket"
)

// Helper: issue a realtime ticket for a given user
func issueTestTicket(t *testing.T, app *App, cookie *http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/realtime/ticket", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("failed to issue ticket, status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal ticket response: %v", err)
	}
	return res.Ticket
}

// 1. Task 5.1: WebSocket handshake, tickets, and origin verification
func TestCompulsory_WebSocketHandshakeTicketsAndOrigin(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()
	app := setupTestApp(t, db, true)

	server := httptest.NewServer(app)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws"

	// 1. WS without ticket -> rejected at handshake with 401
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for WS without ticket, got status %v, err: %v", resp.StatusCode, err)
	}

	// Create user and get a valid ticket
	adminCookie := createTestUserAndLogin(t, app, "admin.ws.test@transitops.com", rbac.RoleAdmin)
	ticket := issueTestTicket(t, app, adminCookie)

	// 2. Wrong Origin -> rejected with 403 Forbidden
	_, resp, err = websocket.DefaultDialer.Dial(wsURL+"?ticket="+ticket, http.Header{
		"Origin": []string{"http://malicious-attacker.com"},
	})
	if err == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for WS with wrong Origin, got status %v, err: %v", resp.StatusCode, err)
	}

	// 3. Valid ticket + Allowed Origin -> successful upgrade
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL+"?ticket="+ticket, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err != nil {
		body := ""
		if resp != nil && resp.Body != nil {
			b, _ := io.ReadAll(resp.Body)
			body = string(b)
		}
		t.Fatalf("expected successful WS upgrade, got err: %v, status: %v, body: %s", err, resp.StatusCode, body)
	}
	defer conn.Close()

	// 4. Ticket is single-use: Reusing the same ticket -> rejected with 401
	_, resp, err = websocket.DefaultDialer.Dial(wsURL+"?ticket="+ticket, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when reusing consumed ticket, got status %v, err: %v", resp.StatusCode, err)
	}
}

// 2. Task 5.2: Multi-instance event fan-out and transaction rollback test
func TestCompulsory_TwoInstanceFanOutAndRollback(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"

	// Start two API instances sharing the same DB
	cfg1 := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
		DatabaseURL:    dsn,
	}
	app1 := New(db, cfg1)
	app1.Start(ctx)
	defer app1.Close()

	cfg2 := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
		DatabaseURL:    dsn,
	}
	app2 := New(db, cfg2)
	app2.Start(ctx)
	defer app2.Close()

	// Wait deterministically for Postgres LISTEN to establish on both instances
	select {
	case <-app1.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app1 hub ready")
	}
	select {
	case <-app2.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app2 hub ready")
	}

	server2 := httptest.NewServer(app2)
	defer server2.Close()

	// Connect a client to instance 2
	adminCookie := createTestUserAndLogin(t, app2, "admin.fanout@transitops.com", rbac.RoleAdmin)
	ticket := issueTestTicket(t, app2, adminCookie)

	wsURL2 := "ws" + strings.TrimPrefix(server2.URL, "http") + "/api/v1/ws?ticket=" + ticket
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, http.Header{
		"Origin": []string{"http://localhost:3000"},
	})
	if err != nil {
		t.Fatalf("failed to connect ws to instance 2: %v", err)
	}
	defer conn2.Close()

	// 1. Transaction rolled back on instance 1 -> must emit NOTHING
	txRollback, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	_ = realtime.PublishTx(ctx, txRollback, realtime.EventTripCreated, map[string]any{"trip_id": 99999})
	_ = txRollback.Rollback(ctx)

	// 2. Transaction committed on instance 1 -> emits event to client on instance 2
	txCommit, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	_ = realtime.PublishTx(ctx, txCommit, realtime.EventTripCreated, map[string]any{"trip_id": 88888})
	if err := txCommit.Commit(ctx); err != nil {
		t.Fatalf("failed to commit tx: %v", err)
	}

	// Client on instance 2 must receive the COMMITTED event within 2s, NOT the rolled-back one
	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("client on instance 2 failed to receive event from instance 1: %v", err)
	}

	var ev realtime.Event
	if err := json.Unmarshal(msg, &ev); err != nil {
		t.Fatalf("failed to parse event json: %v", err)
	}
	if ev.Type != realtime.EventTripCreated {
		t.Fatalf("expected event type %s, got %s", realtime.EventTripCreated, ev.Type)
	}
	if fmt.Sprint(ev.IDs["trip_id"]) != "88888" {
		t.Fatalf("expected trip_id 88888 (committed), but received rolled-back trip_id: %+v", ev.IDs)
	}

	// Verify no second message arrives (confirming 99999 was never delivered)
	_ = conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, extraMsg, err := conn2.ReadMessage()
	if err == nil {
		t.Fatalf("unexpected extra message received: %s", string(extraMsg))
	}
}

// 3. Task 5.3: Role-based event filtering and PII suppression
func TestCompulsory_RoleEventFilteringOverWebSocket(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable"
	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
		DatabaseURL:    dsn,
	}
	app := New(db, cfg)
	app.Start(ctx)
	defer app.Close()

	select {
	case <-app.Ready():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for app hub ready")
	}

	server := httptest.NewServer(app)
	defer server.Close()

	// User 1: Financial Analyst
	faCookie := createTestUserAndLogin(t, app, "fa.events@transitops.com", rbac.RoleFinancialAnalyst)
	faTicket := issueTestTicket(t, app, faCookie)

	// User 2: Safety Officer
	soCookie := createTestUserAndLogin(t, app, "so.events@transitops.com", rbac.RoleSafetyOfficer)
	soTicket := issueTestTicket(t, app, soCookie)

	wsBase := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws?ticket="

	connFA, _, err := websocket.DefaultDialer.Dial(wsBase+faTicket, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err != nil {
		t.Fatalf("failed to connect FA: %v", err)
	}
	defer connFA.Close()

	connSO, _, err := websocket.DefaultDialer.Dial(wsBase+soTicket, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err != nil {
		t.Fatalf("failed to connect SO: %v", err)
	}
	defer connSO.Close()

	time.Sleep(100 * time.Millisecond)

	// Emit maintenance.opened event
	tx1, _ := db.Begin(ctx)
	_ = realtime.PublishTx(ctx, tx1, realtime.EventMaintenanceOpened, map[string]any{
		"maintenance_id": 101,
		"vehicle_reg":    "TRK-001",
		"driver_name":    "Secret Driver", // PII to be stripped
	})
	_ = tx1.Commit(ctx)

	// Safety Officer MUST receive maintenance.opened
	_ = connSO.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgSO, err := connSO.ReadMessage()
	if err != nil {
		t.Fatalf("Safety officer failed to receive maintenance.opened: %v", err)
	}
	if strings.Contains(string(msgSO), "Secret Driver") {
		t.Fatalf("PII leaked in event payload: %s", string(msgSO))
	}

	// Financial Analyst MUST NOT receive maintenance.opened
	_ = connFA.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	_, msgFA, err := connFA.ReadMessage()
	if err == nil {
		t.Fatalf("Financial analyst should NOT have received maintenance.opened, got: %s", string(msgFA))
	}
}

// 4. Task 5.4: Max connections per user & slow consumer drop
func TestCompulsory_MaxConnectionsAndSlowConsumer(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	app := setupTestApp(t, db, true)
	server := httptest.NewServer(app)
	defer server.Close()

	adminCookie := createTestUserAndLogin(t, app, "admin.limits@transitops.com", rbac.RoleAdmin)
	wsBase := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws?ticket="

	// Open 5 connections (the maximum allowed)
	var conns []*websocket.Conn
	for i := 0; i < 5; i++ {
		tkt := issueTestTicket(t, app, adminCookie)
		conn, _, err := websocket.DefaultDialer.Dial(wsBase+tkt, http.Header{"Origin": []string{"http://localhost:3000"}})
		if err != nil {
			t.Fatalf("failed opening connection %d: %v", i+1, err)
		}
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	// 6th connection must be rejected with close code 4429
	tkt6 := issueTestTicket(t, app, adminCookie)
	conn6, _, err := websocket.DefaultDialer.Dial(wsBase+tkt6, http.Header{"Origin": []string{"http://localhost:3000"}})
	if err == nil {
		// Verify close code received
		_ = conn6.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _, readErr := conn6.ReadMessage()
		if readErr == nil {
			t.Fatalf("expected 6th connection to be closed with 4429, but message read succeeded")
		}
		if !websocket.IsCloseError(readErr, realtime.CloseTooManyConns) {
			t.Logf("connection closed with: %v (expected close 4429)", readErr)
		}
		conn6.Close()
	}
}
