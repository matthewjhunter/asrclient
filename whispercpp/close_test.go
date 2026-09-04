package whispercpp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/matthewjhunter/asrclient"
)

// connWatcher counts server-side connection closes so a test can assert
// that Close actually released the pooled connection rather than
// silently doing nothing.
type connWatcher struct {
	mu     sync.Mutex
	closed int
}

func (w *connWatcher) state(_ net.Conn, s http.ConnState) {
	if s == http.StateClosed {
		w.mu.Lock()
		w.closed++
		w.mu.Unlock()
	}
}

func (w *connWatcher) closedCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// TestClose_ReleasesIdleConnections is the contract Close documents:
// "releases idle connections". A client built by NewClient left
// hc.Transport nil, so Close's type assertion failed on the nil
// interface, CloseIdleConnections was never called, and the connection
// stayed pooled on http.DefaultTransport -- shared with every other
// caller in the process.
func TestClose_ReleasesIdleConnections(t *testing.T) {
	w := &connWatcher{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{"text":"hello world"}`))
	}))
	srv.Config.ConnState = w.state
	srv.Start()
	defer srv.Close()

	c := NewClient(WithEndpoint(srv.URL), WithTimeout(5*time.Second))
	if _, err := c.Transcribe(t.Context(), make([]byte, 3200), asrclient.Options{}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	before := w.closedCount()
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.closedCount() > before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("Close did not release the pooled connection (closed count stayed %d)", before)
}

// TestClient_OwnsItsTransport pins the mechanism: the client must hold
// its own *http.Transport, both so Close can reach it and so one
// consumer's connections are not pooled on the process-wide
// http.DefaultTransport alongside everyone else's.
func TestClient_OwnsItsTransport(t *testing.T) {
	c := NewClient()
	tr, ok := c.hc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport: got %T, want *http.Transport", c.hc.Transport)
	}
	if tr == http.DefaultTransport {
		t.Error("client shares http.DefaultTransport; it must own its transport")
	}
}
