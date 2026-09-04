package whispercpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matthewjhunter/asrclient"
)

func TestClient_TranscribeOmitsAuth(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"local"}`))
	}))
	defer srv.Close()

	c := NewClient(WithEndpoint(srv.URL))
	defer c.Close()

	tr, err := c.Transcribe(context.Background(), []byte{0x00, 0x00}, asrclient.Options{})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if tr.Text != "local" {
		t.Errorf("Text: got %q", tr.Text)
	}
	if sawAuth != "" {
		t.Errorf("whispercpp must not send Authorization, got %q", sawAuth)
	}
}

func TestClient_PingDefaultsToHEAD(t *testing.T) {
	var sawMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(WithEndpoint(srv.URL))
	defer c.Close()

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if sawMethod != http.MethodHead {
		t.Errorf("method: got %q want HEAD (no health endpoint configured)", sawMethod)
	}
}

func TestClient_PingUsesHealthEndpointGET(t *testing.T) {
	var sawMethod, sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod, sawPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(
		WithEndpoint(srv.URL+"/v1/audio/transcriptions"),
		WithHealthEndpoint(srv.URL+"/api/v1/health"),
	)
	defer c.Close()

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if sawMethod != http.MethodGet {
		t.Errorf("method: got %q want GET", sawMethod)
	}
	if sawPath != "/api/v1/health" {
		t.Errorf("path: got %q want /api/v1/health", sawPath)
	}
}

func TestClient_TranscriberInterface(t *testing.T) {
	var _ asrclient.Transcriber = NewClient()
}
