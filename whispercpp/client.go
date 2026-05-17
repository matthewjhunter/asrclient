// Package whispercpp is an asrclient.Backend that talks to a
// whisper.cpp `whisper-server` over loopback HTTP. The server is
// expected to be supervised by the consumer (port discovery, health
// gating, restart-on-crash) — this package is just the protocol client.
package whispercpp

import (
	"context"
	"net/http"
	"time"

	"github.com/matthewjhunter/asrclient"
	"github.com/matthewjhunter/asrclient/internal/httpcore"
)

// Defaults for a loopback whisper-server. Override via WithEndpoint
// (e.g. once the supervised server's port is known) or WithTimeout.
const (
	DefaultEndpoint = "http://127.0.0.1:8080/v1/audio/transcriptions"
	// DefaultModel is the placeholder model name. whisper-server
	// ignores the model field, but the OpenAI-compatible protocol
	// still requires it; "whisper-1" is the conventional value.
	DefaultModel   = "whisper-1"
	DefaultTimeout = 30 * time.Second
)

// Client is the whisper.cpp Backend implementation.
type Client struct {
	endpoint string
	model    string
	hc       *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint overrides the transcription URL.
func WithEndpoint(s string) Option { return func(c *Client) { c.endpoint = s } }

// WithModel overrides the model name sent in the multipart payload.
func WithModel(s string) Option { return func(c *Client) { c.model = s } }

// WithTimeout sets the per-request timeout on the default http.Client.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.hc.Timeout = d }
}

// WithHTTPClient swaps in a caller-provided http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// NewClient constructs a Client. The default points at
// http://127.0.0.1:8080/v1/audio/transcriptions; consumers that
// supervise a whisper-server with a discovered port should pass
// WithEndpoint after the port is known.
func NewClient(opts ...Option) *Client {
	c := &Client{
		endpoint: DefaultEndpoint,
		model:    DefaultModel,
		hc:       &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Transcribe implements asrclient.Backend.
func (c *Client) Transcribe(ctx context.Context, audio []byte, opts asrclient.Options) (asrclient.Transcript, error) {
	return httpcore.PostTranscription(ctx, c.hc, httpcore.Request{
		Endpoint: c.endpoint,
		Model:    c.model,
		Language: opts.Language,
		Prompt:   opts.InitialPrompt,
		PCMAudio: audio,
	})
}

// Healthy implements asrclient.Backend.
func (c *Client) Healthy(ctx context.Context) error {
	return httpcore.HealthyHEAD(ctx, c.hc, c.endpoint)
}

// Close releases idle connections.
func (c *Client) Close() error {
	if t, ok := c.hc.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

var _ asrclient.Backend = (*Client)(nil)
