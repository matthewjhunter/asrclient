// Package openai is an asrclient.Transcriber that talks to the public
// OpenAI /v1/audio/transcriptions endpoint (or any drop-in
// OpenAI-compatible service). TLS verification is on by default; the
// API key is sent as Bearer auth.
package openai

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/matthewjhunter/asrclient"
	"github.com/matthewjhunter/asrclient/internal/httpcore"
)

// Defaults for the public OpenAI /v1/audio/transcriptions endpoint.
// Override via WithEndpoint, WithModel, and WithTimeout (or
// WithHTTPClient for full control).
const (
	DefaultEndpoint = "https://api.openai.com/v1/audio/transcriptions"
	DefaultModel    = "whisper-1"
	DefaultTimeout  = 30 * time.Second
)

// Client is the OpenAI Transcriber implementation.
type Client struct {
	endpoint string
	apiKey   string
	model    string
	hc       *http.Client

	// healthEndpoint is empty unless WithHealthEndpoint was used;
	// empty means Ping falls back to a HEAD on endpoint.
	healthEndpoint string
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint overrides the transcription URL.
func WithEndpoint(s string) Option { return func(c *Client) { c.endpoint = s } }

// WithModel overrides the model name sent in the multipart payload.
func WithModel(s string) Option { return func(c *Client) { c.model = s } }

// WithTimeout sets the per-request timeout on the default http.Client.
// Has no effect when WithHTTPClient is also used.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.hc.Timeout = d }
}

// WithHTTPClient swaps in a caller-provided http.Client. Use this when
// you need custom TLS, proxies, or instrumentation.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// WithTLSInsecureSkipVerify disables certificate verification. Local-LAN
// testing only — see asrclient module security guidance.
// It mutates the client's existing transport rather than replacing it,
// so proxy handling, dial and handshake timeouts, and HTTP/2 survive.
// Replacing the transport outright silently dropped all of those.
func WithTLSInsecureSkipVerify() Option {
	return func(c *Client) {
		tr, ok := c.hc.Transport.(*http.Transport)
		if !ok {
			// A caller-supplied client whose transport we do not
			// understand: give it one we do rather than reaching in.
			tr = httpcore.NewTransport()
			c.hc.Transport = tr
		}
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{} //nolint:gosec // InsecureSkipVerify set below; opt-in escape hatch, see SECURITY.md
		}
		tr.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // opt-in escape hatch; see SECURITY.md
	}
}

// WithHealthEndpoint points Ping at a dedicated health endpoint instead
// of probing the transcription path.
//
// Pass a path -- "/api/v1/health" for Lemonade Server, "/health" for
// whisper-server -- and it is resolved against the transcription
// endpoint's origin. Pass an absolute URL to probe a different host.
//
// This changes what a successful Ping means. Unset, Ping issues a HEAD
// to the transcription endpoint and counts any reply as success,
// because that only claims something is listening: many servers do not
// route HEAD on that path and answer 405, which is still proof of life
// (and which they typically log as an error on every probe). Set, Ping
// issues a GET and requires 2xx, so it asserts the service says it is
// ready -- and stops writing an error line into the server's log every
// time it is called.
func WithHealthEndpoint(s string) Option { return func(c *Client) { c.healthEndpoint = s } }

// NewClient constructs a Client. apiKey is sent as Bearer auth; pass ""
// to omit the header (some private deployments accept anonymous traffic).
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		endpoint: DefaultEndpoint,
		apiKey:   apiKey,
		model:    DefaultModel,
		hc:       httpcore.NewHTTPClient(DefaultTimeout),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Transcribe implements asrclient.Transcriber.
func (c *Client) Transcribe(ctx context.Context, audio []byte, opts asrclient.Options) (asrclient.Transcript, error) {
	return httpcore.PostTranscription(ctx, c.hc, httpcore.Request{
		Endpoint: c.endpoint,
		APIKey:   c.apiKey,
		Model:    c.model,
		Language: opts.Language,
		Prompt:   opts.InitialPrompt,
		PCMAudio: audio,
	})
}

// Ping implements asrclient.Transcriber. It probes the configured
// health endpoint when one is set (GET, 2xx required) and otherwise
// falls back to a HEAD against the transcription endpoint. See
// WithHealthEndpoint for why the two differ in strictness.
func (c *Client) Ping(ctx context.Context) error {
	if c.healthEndpoint != "" {
		healthURL, err := httpcore.ResolveHealthURL(c.endpoint, c.healthEndpoint)
		if err != nil {
			return err
		}
		return httpcore.PingHealth(ctx, c.hc, healthURL)
	}
	return httpcore.PingHEAD(ctx, c.hc, c.endpoint)
}

// Close releases the connections this client pooled. It is a no-op for
// a caller-supplied http.Client whose transport is not an
// *http.Transport -- that transport's lifecycle belongs to the caller.
func (c *Client) Close() error {
	if t, ok := c.hc.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

var _ asrclient.Transcriber = (*Client)(nil)
