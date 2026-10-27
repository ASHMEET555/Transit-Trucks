package main

import (
	"net/http"
	"testing"
	"time"
)

func TestServer_TimeoutConfig(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv := NewHTTPServer("8080", dummyHandler)

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ReadHeaderTimeout = 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout = 60s, got %v", srv.IdleTimeout)
	}
}
