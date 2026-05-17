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

func TestClient_TranscriberInterface(t *testing.T) {
	var _ asrclient.Transcriber = NewClient()
}
