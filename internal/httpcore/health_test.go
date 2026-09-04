package httpcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveHealthURL(t *testing.T) {
	const endpoint = "http://host:13305/v1/audio/transcriptions"
	cases := []struct {
		name   string
		health string
		want   string
	}{
		{"absolute url used verbatim", "http://other:9/healthz", "http://other:9/healthz"},
		{"absolute https url", "https://other/health", "https://other/health"},
		// A path is resolved against the endpoint's origin, so the
		// caller does not repeat the host it already configured. This
		// is the common case: Lemonade serves health at a different
		// path prefix on the same origin.
		{"rooted path", "/api/v1/health", "http://host:13305/api/v1/health"},
		{"bare path gets rooted", "api/v1/health", "http://host:13305/api/v1/health"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveHealthURL(endpoint, tc.health)
			if err != nil {
				t.Fatalf("ResolveHealthURL: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveHealthURL_Errors(t *testing.T) {
	if _, err := ResolveHealthURL("://bad", "/health"); err == nil {
		t.Error("expected an error for an unparseable endpoint")
	}
	if _, err := ResolveHealthURL("http://host/x", ""); err == nil {
		t.Error("expected an error for an empty health value")
	}
}

// TestPingHealth_RequiresSuccessStatus is the semantic upgrade over
// PingHEAD. A configured health endpoint is a real endpoint, so a
// non-2xx answer means it is not healthy -- or not the health endpoint
// at all. PingHEAD deliberately accepts anything because it only claims
// reachability; PingHealth claims more and must check.
func TestPingHealth_RequiresSuccessStatus(t *testing.T) {
	cases := []struct {
		status  int
		wantErr bool
	}{
		{http.StatusOK, false},
		{http.StatusNoContent, false},
		{http.StatusNotFound, true},
		{http.StatusMethodNotAllowed, true},
		{http.StatusInternalServerError, true},
		{http.StatusServiceUnavailable, true},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Method: got %q want GET", r.Method)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			err := PingHealth(context.Background(), srv.Client(), srv.URL)
			if tc.wantErr && err == nil {
				t.Errorf("status %d: expected an error", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("status %d: %v", tc.status, err)
			}
		})
	}
}

func TestPingHealth_ReportsStatusInError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := PingHealth(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected an error")
	}
	// The status is the whole diagnostic: a 404 means the configured
	// health path is wrong, which is a different fix from a 503.
	if got := err.Error(); !contains(got, "404") {
		t.Errorf("error %q does not name the status code", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPingHealth_FailsOnNetworkError(t *testing.T) {
	// Port 1 should refuse fast. A transport error is a failure under
	// both probe modes -- nothing is listening to have an opinion.
	err := PingHealth(context.Background(),
		&http.Client{Timeout: 200 * time.Millisecond}, "http://127.0.0.1:1/health")
	if err == nil {
		t.Fatal("expected dial failure")
	}
}
