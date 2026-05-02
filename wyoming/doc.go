// Package wyoming implements the Wyoming wire protocol (JSON-header
// plus optional binary-payload TCP framing, used by Home Assistant–
// ecosystem voice services like wyoming-faster-whisper) and provides
// a Client type that satisfies asrclient.Backend.
//
// The wire-protocol surface (Event, ReadEvent, WriteEvent, Conn, the
// typed event constructors) is exposed for callers that want raw
// access — for example a wakeword consumer that streams detect events
// rather than transcripts.
package wyoming
