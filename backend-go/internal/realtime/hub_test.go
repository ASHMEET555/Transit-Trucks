package realtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
)

func TestHub_MissingOrDisallowedOrigin_Rejected403(t *testing.T) {
	pool := getDB(t)
	defer pool.Close()

	allowed := []string{"http://localhost:3000", "https://app.transitops.com"}
	hub := realtime.NewHub("postgresql://test", pool, allowed, nil)

	// Issue a valid ticket first
	ctx := context.Background()
	rawTicket, _, err := realtime.IssueTicket(ctx, pool, "test-user-123", "Fleet Manager")
	if err != nil {
		t.Fatalf("failed to issue ticket: %v", err)
	}

	// 1. Missing Origin header -> 403 Forbidden before upgrade
	reqNoOrigin := httptest.NewRequest("GET", "/ws/events?ticket="+rawTicket, nil)
	wNoOrigin := httptest.NewRecorder()
	err = hub.Upgrade(wNoOrigin, reqNoOrigin, nil)
	if err == nil {
		t.Fatal("expected upgrade error for missing Origin, got nil")
	}
	if wNoOrigin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing Origin, got %d", wNoOrigin.Code)
	}

	// 2. Disallowed Origin header -> 403 Forbidden before upgrade
	reqDisallowed := httptest.NewRequest("GET", "/ws/events?ticket="+rawTicket, nil)
	reqDisallowed.Header.Set("Origin", "http://evil-attacker.com")
	wDisallowed := httptest.NewRecorder()
	err = hub.Upgrade(wDisallowed, reqDisallowed, nil)
	if err == nil {
		t.Fatal("expected upgrade error for disallowed Origin, got nil")
	}
	if wDisallowed.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed Origin, got %d", wDisallowed.Code)
	}
}
