# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

`asrclient` is a pure-Go module exposing a unified `Transcriber` interface for
streaming and one-shot speech-to-text transcription, with implementations
for the three protocols dicta and similar consumers care about:

- **Wyoming** (TCP, JSON-header + binary-payload framing, used by
  Home Assistant–ecosystem servers like `wyoming-faster-whisper`)
- **OpenAI** (HTTPS multipart/form-data POST to
  `/v1/audio/transcriptions`, with API key auth)
- **whisper.cpp** (the same multipart/form-data POST, but pointed at a
  loopback `whisper-server` with no auth and TLS-verify-off by default)

The module is consumed by `dicta` and reserved for any future voice
project that needs the same protocol set.

## Hard constraints

- **Pure Go, no CGo.** `CGO_ENABLED=0` builds clean. The `Taskfile.yml`
  enforces this by default; only `test:race` overrides because the race
  detector requires CGo.
- **No subprocess lifecycle.** `whispercpp` here is *only* the HTTP
  client. Spawning `whisper-server`, port discovery, restart-on-crash,
  `/health` gating — all of that lives in the consumer (e.g. dicta's
  `internal/whispersupervisor`). Keeping protocol and lifecycle
  separate is the reason this module exists as a separate thing.
- **Stable surface.** The `Transcriber` interface and `Options` /
  `Transcript` types are the public API. Don't grow them speculatively.
  Add fields when a real consumer needs them.

## Audio frame format (locked)

All callers and backends assume the same PCM shape:

- 16 kHz sample rate
- mono
- int16 little-endian
- 80 ms / 1280-sample / 2560-byte frames

Constants live in `audio.go`. This matches the conventions of the
Wyoming ecosystem and openWakeWord, so a consumer that already uses
this shape pays no resampling cost.

## Module layout

```
asrclient/
├── client.go              # Transcriber, Options, Transcript, Segment
├── audio.go               # frame-format constants
├── wyoming/               # Wyoming wire protocol + Transcriber impl
├── openai/                # OpenAI HTTPS Transcriber
├── whispercpp/            # OpenAI-protocol Transcriber, loopback defaults
└── internal/
    └── httpcore/          # shared multipart/form-data POST core
```

`wyoming/` keeps zero non-stdlib imports beyond the parent package's
types — it could be lifted into its own module if a consumer ever needs
just the wire protocol without the rest of asrclient.
