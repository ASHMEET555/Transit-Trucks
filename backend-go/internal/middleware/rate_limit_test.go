package middleware_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
)

func TestRateLimit_UntrustedPeerSpoofedXFF(t *testing.T) {
	_, trustedCIDR, _ := net.ParseCIDR("10.0.0.1/32")
	trusted := []*net.IPNet{trustedCIDR}

	handler := middleware.RateLimit(2, time.Minute, trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Peer 192.168.1.100 is untrusted (does not match 10.0.0.1/32)
	// First request from peer with spoofed XFF: 1.1.1.1
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.100:12345"
	req1.Header.Set("X-Forwarded-For", "1.1.1.1")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("req1: expected 200, got %d", w1.Code)
	}

	// Second request from same peer with spoofed XFF: 2.2.2.2
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	req2.Header.Set("X-Forwarded-For", "2.2.2.2")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("req2: expected 200, got %d", w2.Code)
	}

	// Third request from same peer with spoofed XFF: 3.3.3.3
	// Since peer is untrusted, RemoteAddr (192.168.1.100) is used, so third request must be 429!
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.100:12345"
	req3.Header.Set("X-Forwarded-For", "3.3.3.3")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("req3: expected 429 RATE_LIMIT_EXCEEDED, got %d (XFF spoofing was mistakenly trusted)", w3.Code)
	}
}
