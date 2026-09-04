package openai

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestPing_UsesConfiguredHealthEndpoint: with a health path set, Ping
// must GET that path and not touch the transcription endpoint.
func TestPing_UsesConfiguredHealthEndpoint(t *testing.T) {
	var healthHits, otherHits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" && r.Method == http.MethodGet {
			healthHits.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		otherHits.Add(1)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := NewClient("",
		WithEndpoint(srv.URL+"/v1/audio/transcriptions"),
		WithHealthEndpoint("/api/v1/health"))
	defer func() { _ = c.Close() }()

	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if healthHits.Load() != 1 {
		t.Errorf("health endpoint hits: got %d want 1", healthHits.Load())
	}
	if otherHits.Load() != 0 {
		t.Errorf("transcription endpoint was probed %d times; want 0", otherHits.Load())
	}
}

// TestPing_HealthEndpointFailsOnNon2xx is the behavior change that
// makes configuring it worthwhile: a 405 on the health path is a
// failure, where the HEAD fallback would call it success.
func TestPing_HealthEndpointFailsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := NewClient("", WithEndpoint(srv.URL+"/v1/audio/transcriptions"),
		WithHealthEndpoint("/api/v1/health"))
	defer func() { _ = c.Close() }()

	if err := c.Ping(t.Context()); err == nil {
		t.Fatal("Ping: got nil, want an error for a 405 health response")
	}
}

// TestPing_FallsBackToHEAD pins the compatibility promise: unset means
// the old behavior, including treating a 405 as a successful ping.
func TestPing_FallsBackToHEAD(t *testing.T) {
	var method atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method.Store(r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := NewClient("", WithEndpoint(srv.URL+"/v1/audio/transcriptions"))
	defer func() { _ = c.Close() }()

	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v -- unset health endpoint must keep the old any-reply behavior", err)
	}
	if got := method.Load(); got != http.MethodHead {
		t.Errorf("Method: got %v want HEAD", got)
	}
}

// TestPing_AbsoluteHealthURL: health may live on another host.
func TestPing_AbsoluteHealthURL(t *testing.T) {
	var hits atomic.Int64
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer health.Close()

	c := NewClient("",
		WithEndpoint("http://127.0.0.1:1/v1/audio/transcriptions"),
		WithHealthEndpoint(health.URL+"/healthz"))
	defer func() { _ = c.Close() }()

	if err := c.Ping(t.Context()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("health hits: got %d want 1", hits.Load())
	}
}
