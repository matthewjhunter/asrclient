package httpcore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrEmptyHealthEndpoint is returned by ResolveHealthURL when asked to
// resolve an empty value. Callers treat an unset health endpoint as
// "fall back to PingHEAD" and should not reach here.
var ErrEmptyHealthEndpoint = errors.New("asrclient/httpcore: empty health endpoint")

// ResolveHealthURL turns a caller-configured health endpoint into an
// absolute URL.
//
// An absolute URL (one with a scheme) is used verbatim, for a health
// service that lives somewhere else entirely. Anything else is treated
// as a path and resolved against the transcription endpoint's origin,
// which is the common case: a server that transcribes at
// /v1/audio/transcriptions typically reports health at a different path
// on the same host, such as Lemonade's /api/v1/health or
// whisper-server's /health. Resolving it here means the caller
// configures the path once and never repeats the host.
func ResolveHealthURL(endpoint, health string) (string, error) {
	if health == "" {
		return "", ErrEmptyHealthEndpoint
	}
	if u, err := url.Parse(health); err == nil && u.IsAbs() {
		return health, nil
	}
	base, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("asrclient/httpcore: parse endpoint: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("asrclient/httpcore: endpoint %q is not an absolute URL", endpoint)
	}
	if !strings.HasPrefix(health, "/") {
		health = "/" + health
	}
	ref, err := url.Parse(health)
	if err != nil {
		return "", fmt.Errorf("asrclient/httpcore: parse health path: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}

// PingHealth issues a GET to a configured health endpoint and requires
// a 2xx response.
//
// This is deliberately stricter than PingHEAD. PingHEAD claims only
// that something is listening, so it accepts any reply -- including the
// 405 a server returns when it does not route HEAD on the transcription
// path. A health endpoint is a real endpoint that the caller named, so
// anything but success means something is wrong: a 404 says the
// configured path is not there, a 503 says the service is up and not
// ready. Reporting either as healthy would defeat the point of
// configuring it.
func PingHealth(ctx context.Context, hc *http.Client, healthURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("asrclient/httpcore: build health GET: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("asrclient/httpcore: health GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain a bounded amount so the connection can be reused; health
	// bodies are small, and an unread body pins the connection.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("asrclient/httpcore: health endpoint %s returned %d %s",
			healthURL, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return nil
}
