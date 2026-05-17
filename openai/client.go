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
func WithTLSInsecureSkipVerify() Option {
	return func(c *Client) {
		c.hc.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // opt-in escape hatch; see SECURITY.md
		}
	}
}

// NewClient constructs a Client. apiKey is sent as Bearer auth; pass ""
// to omit the header (some private deployments accept anonymous traffic).
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		endpoint: DefaultEndpoint,
		apiKey:   apiKey,
		model:    DefaultModel,
		hc:       &http.Client{Timeout: DefaultTimeout},
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

// Ping implements asrclient.Transcriber.
func (c *Client) Ping(ctx context.Context) error {
	return httpcore.PingHEAD(ctx, c.hc, c.endpoint)
}

// Close releases idle connections.
func (c *Client) Close() error {
	if t, ok := c.hc.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

var _ asrclient.Transcriber = (*Client)(nil)
