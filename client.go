package asrclient

import (
	"context"
	"time"
)

// Backend transcribes a single utterance of PCM audio into text.
//
// Implementations are not required to be safe for concurrent calls
// to Transcribe — callers serializing utterances per session is the
// expected pattern. Healthy and Close MAY be called concurrently with
// Transcribe.
type Backend interface {
	// Transcribe consumes one utterance worth of PCM audio in the
	// module's locked frame format (see audio.go) and returns the
	// transcript. Implementations may stream the input internally
	// (Wyoming chunks at the 80 ms cadence) or upload it whole
	// (HTTP multipart) — that detail is hidden from the caller.
	Transcribe(ctx context.Context, audio []byte, opts Options) (Transcript, error)

	// Healthy probes the backend. A nil return means ready; a
	// non-nil return means the backend is currently unable to
	// service requests.
	Healthy(ctx context.Context) error

	// Close releases any persistent resources (sockets, HTTP
	// keep-alive pools, in-flight goroutines).
	Close() error
}

// Options controls a single Transcribe call.
type Options struct {
	// Language is the ISO-639-1 language hint, or "" for auto-detect.
	Language string

	// InitialPrompt biases the decoder toward specific terminology
	// or formatting; semantics depend on the backend.
	InitialPrompt string

	// Temperature is the decoder sampling temperature; 0 means
	// "let the backend pick its default" (typically deterministic).
	Temperature float32
}

// Transcript is a successful transcription result.
type Transcript struct {
	// Text is the recognized utterance, post-decoding. Empty Text
	// is a valid result when the audio contained no speech.
	Text string

	// Language is the ISO-639-1 code the backend identified, or
	// empty if the backend did not report one.
	Language string

	// Duration is the wall-clock time the backend spent decoding,
	// when the backend reports it. Zero otherwise.
	Duration time.Duration

	// Segments is the optional time-aligned segmentation. Nil if
	// the backend does not return per-segment timestamps.
	Segments []Segment
}

// Segment is one timestamped slice of a transcription.
type Segment struct {
	Text  string
	Start time.Duration
	End   time.Duration
}
