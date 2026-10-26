package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

func TestCompulsory_PanicRecoveryAndRequestID(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	// Create test handler that panics
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("deliberate test panic: database wire broken")
	})

	// Wrap in recover, request-id, access log pipeline
	pipeline := middleware.Recover(logger)(
		middleware.RequestID()(
			middleware.AccessLog(logger)(panickingHandler),
		),
	)

	req := httptest.NewRequest("GET", "/api/v1/test-panic", nil)
	req.Header.Set("X-Request-ID", "req-panic-test-999")
	w := httptest.NewRecorder()

	pipeline.ServeHTTP(w, req)

	// 1. Must return HTTP 500
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

	// 3. Log buffer must contain request_id and panic details
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "req-panic-test-999") {
		t.Fatalf("expected request_id in recovered log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "deliberate test panic") {
		t.Fatalf("expected panic detail in server log, got: %s", logOutput)
	}
}

func TestCompulsory_LogScrubSecretNeverLogged(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	cfg := Config{
		Secret:         "a-very-long-and-secure-session-secret-key-32-bytes",
		AllowedOrigins: []string{"http://localhost:3000"},
		Env:            "local",
		CookieDomain:   "transitops.com",
		Logger:         logger,
	}
	app := New(db, cfg)

	secretPassword := "SuperConfidentialPassword987!"
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "scrub.test@transitops.com",
		"password": secretPassword,
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-scrub-test-111")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	logOutput := logBuf.String()

	// 1. Password must NEVER appear in logs
	if strings.Contains(logOutput, secretPassword) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: password appeared in log output! Output: %s", logOutput)
	}

	// 2. Structured access log fields must be present
	if !strings.Contains(logOutput, "req-scrub-test-111") {
		t.Errorf("request_id missing from access log: %s", logOutput)
	}
	if !strings.Contains(logOutput, "route") || !strings.Contains(logOutput, "/api/v1/auth/login") {
		t.Errorf("route pattern missing from access log: %s", logOutput)
	}
}
