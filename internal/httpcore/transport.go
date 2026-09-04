package httpcore

import (
	"net/http"
	"time"
)

// NewTransport returns a transport for a client to own.
//
// Clients must not be left with a nil http.Client.Transport. A nil
// Transport means net/http silently substitutes http.DefaultTransport,
// which has two consequences a library should not impose on its
// consumers:
//
//   - Close cannot release anything. The idiomatic
//     `Transport.(*http.Transport)` assertion fails on the nil
//     interface, so CloseIdleConnections is never reached and Close
//     becomes a no-op.
//   - Connections pool process-wide. A consumer's transcription
//     sockets sit in the same pool as every other DefaultTransport
//     user in the program, so its idle connections outlive the client
//     and nobody can reclaim them without disturbing unrelated code.
//
// The returned transport is a clone of http.DefaultTransport, so it
// keeps the standard library's proxy handling, dial and TLS handshake
// timeouts, and HTTP/2 upgrade rather than reimplementing them badly.
func NewTransport() *http.Transport {
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		return dt.Clone()
	}
	// Defensive: something replaced http.DefaultTransport with a type
	// we cannot clone. Better a plain transport we own than a nil one.
	return &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// NewHTTPClient returns an http.Client with the given timeout and a
// transport of its own. See NewTransport for why the transport is never
// left nil.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: NewTransport()}
}
