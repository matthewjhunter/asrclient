package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matthewjhunter/asrclient"
)

// TestClient_TranscribeDelegatesToHTTP verifies the openai Client wires
// its config fields into the underlying HTTP request the way callers
// expect. The shared multipart shape is covered by httpcore tests; here
// we just confirm endpoint, API key, and model are threaded through.
func TestClient_TranscribeDelegatesToHTTP(t *testing.T) {
	var sawEndpoint, sawAuth, sawModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawEndpoint = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(1 << 20)
		sawModel = r.FormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient("sk-abc",
		WithEndpoint(srv.URL+"/v1/audio/transcriptions"),
		WithModel("whisper-large"),
	)
	defer c.Close()

	tr, err := c.Transcribe(context.Background(), []byte{0x00, 0x00}, asrclient.Options{})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if tr.Text != "ok" {
		t.Errorf("Text: got %q", tr.Text)
	}
	if sawEndpoint != "/v1/audio/transcriptions" {
		t.Errorf("endpoint: got %q", sawEndpoint)
	}
	if sawAuth != "Bearer sk-abc" {
		t.Errorf("auth: got %q", sawAuth)
	}
	if sawModel != "whisper-large" {
		t.Errorf("model: got %q", sawModel)
	}
}

func TestClient_PingDelegatesToHEAD(t *testing.T) {
	var sawMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("", WithEndpoint(srv.URL))
	defer c.Close()

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if sawMethod != http.MethodHead {
		t.Errorf("method: got %q want HEAD", sawMethod)
	}
}

func TestClient_PingUsesHealthEndpointGET(t *testing.T) {
	var sawMethod, sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod, sawPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("",
		WithEndpoint(srv.URL+"/v1/audio/transcriptions"),
		WithHealthEndpoint(srv.URL+"/health"),
	)
	defer c.Close()

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if sawMethod != http.MethodGet {
		t.Errorf("method: got %q want GET", sawMethod)
	}
	if sawPath != "/health" {
		t.Errorf("path: got %q want /health", sawPath)
	}
}

func TestClient_TranscriberInterface(t *testing.T) {
	var _ asrclient.Transcriber = NewClient("k")
}
