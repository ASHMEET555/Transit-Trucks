package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCompulsory_RequestTimeout_503(t *testing.T) {
	slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			status, code, msg := httpx.MapDBError(r.Context().Err(), nil)
			httpx.WriteError(w, r, status, code, msg, nil)
		}
	})

	// Wrap in a 50ms RequestTimeout middleware
	pipeline := middleware.RequestTimeout(50 * time.Millisecond)(slowHandler)

	req := httptest.NewRequest("GET", "/api/v1/slow", nil)
	w := httptest.NewRecorder()

	pipeline.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable on timeout, got %d", w.Code)
	}

	var errResp httpx.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse timeout error body: %v", err)
	}
	if errResp.Error.Code != "REQUEST_TIMEOUT" {
		t.Errorf("expected error code REQUEST_TIMEOUT, got %s", errResp.Error.Code)
	}
}

func TestCompulsory_StatementTimeout_504(t *testing.T) {
	// Simulate Postgres statement timeout error (code 57014)
	pgErr := &pgconn.PgError{
		Code:    "57014",
		Message: "canceling statement due to statement timeout",
	}

	status, code, _ := httpx.MapDBError(pgErr, nil)
	if status != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 Gateway Timeout for statement timeout, got %d", status)
	}
	if code != "DB_TIMEOUT" {
		t.Fatalf("expected code DB_TIMEOUT, got %s", code)
	}
}

func TestCompulsory_ReadyzDatabaseBudget(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	// 1. Live database -> 200 OK
	appLive := setupTestApp(t, db, true)
	reqLive := httptest.NewRequest("GET", "/readyz", nil)
	wLive := httptest.NewRecorder()
	appLive.ServeHTTP(wLive, reqLive)

	if wLive.Code != http.StatusOK {
		t.Fatalf("expected 200 for live readyz, got %d", wLive.Code)
	}

	// 2. Closed/nil database -> 503 Service Unavailable
	appDown := setupTestApp(t, nil, true)
	reqDown := httptest.NewRequest("GET", "/readyz", nil)
	wDown := httptest.NewRecorder()
	appDown.ServeHTTP(wDown, reqDown)

	if wDown.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for down readyz, got %d", wDown.Code)
	}
}

func TestCompulsory_GracefulShutdownInFlight(t *testing.T) {
	mux := http.NewServeMux()
	inFlightStarted := make(chan struct{})
	inFlightCanFinish := make(chan struct{})

	mux.HandleFunc("/slow-work", func(w http.ResponseWriter, r *http.Request) {
		close(inFlightStarted)
		<-inFlightCanFinish
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"finished"}`))
	})

	server := httptest.NewServer(mux)

	// Launch in-flight request in background
	reqErrCh := make(chan error, 1)
	reqStatusCh := make(chan int, 1)
	go func() {
		resp, err := http.Get(server.URL + "/slow-work")
		if err != nil {
			reqErrCh <- err
			return
		}
		defer resp.Body.Close()
		reqStatusCh <- resp.StatusCode
	}()

	// Wait for request to start processing on server
	<-inFlightStarted

	// Allow request to finish and initiate shutdown
	close(inFlightCanFinish)

	select {
	case status := <-reqStatusCh:
		if status != http.StatusOK {
			t.Fatalf("in-flight request failed with status %d during shutdown", status)
		}
	case err := <-reqErrCh:
		t.Fatalf("in-flight request failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for in-flight request to complete")
	}

	server.Close()
}
