# asrclient

[![CI](https://github.com/matthewjhunter/asrclient/actions/workflows/ci.yml/badge.svg)](https://github.com/matthewjhunter/asrclient/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/matthewjhunter/asrclient.svg)](https://pkg.go.dev/github.com/matthewjhunter/asrclient)
[![Go Report Card](https://goreportcard.com/badge/github.com/matthewjhunter/asrclient)](https://goreportcard.com/report/github.com/matthewjhunter/asrclient)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A pure-Go module for speech-to-text transcription, with one unified
`Backend` interface and three implementations:

- **Wyoming** — TCP wire protocol used by the Home Assistant voice
  ecosystem (e.g. `wyoming-faster-whisper`). JSON-header + binary-payload
  framing, implemented from the wire format directly.
- **OpenAI** — HTTPS multipart POST to `/v1/audio/transcriptions`, with
  Bearer-token auth and configurable endpoint.
- **whisper.cpp** — the same multipart protocol shape pointed at a local
  `whisper-server` over loopback HTTP, no auth.

```go
import (
    "github.com/matthewjhunter/asrclient"
    "github.com/matthewjhunter/asrclient/wyoming"
)

c := wyoming.NewClient("localhost:10300")
defer c.Close()

tr, err := c.Transcribe(ctx, pcm, asrclient.Options{Language: "en"})
// tr.Text, tr.Language, tr.DecodeDuration, tr.Segments
```

## Design

- **Pure Go, no CGo.** `CGO_ENABLED=0` builds clean; the race detector
  (which requires CGo) is opt-in via `task test:race`.
- **Protocol clients only.** Spawning `whisper-server`, port discovery,
  restart-on-crash, `/health` gating — none of that lives here. The
  whisper.cpp client expects a server already running and reachable.
  Keeping protocol and lifecycle separate is the reason the module
  exists as its own thing.
- **Stable, narrow surface.** `Backend`, `Options`, `Transcript`,
  `Segment` are the public types; backend constructors are
  `NewClient(...)` with optional `WithX(...)` options. No speculative
  fields — they're added when a real consumer needs them.

## Audio frame format (locked)

All callers and backends assume the same PCM shape:

| Field         | Value                       |
|---------------|-----------------------------|
| Sample rate   | 16 kHz                      |
| Channels      | mono                        |
| Sample format | int16 little-endian         |
| Frame size    | 80 ms / 1280 samples / 2560 bytes |

Constants live in `audio.go` (`SampleRateHz`, `Channels`, `SampleWidth`,
`FrameMS`, `FrameSamples`, `FrameBytes`). This matches the conventions
of the Wyoming voice-services ecosystem and openWakeWord, so consumers
that already produce frames in this format pay no resampling cost.

## Install

```sh
go get github.com/matthewjhunter/asrclient
```

## Backends

### Wyoming

```go
c := wyoming.NewClient("localhost:10300",
    wyoming.WithDialTimeout(5*time.Second))
defer c.Close()
```

One TCP connection per `Client`, opened lazily on the first call. On a
transport error the connection is dropped and the next call redials.
No background reconnect goroutine.

### OpenAI

```go
c := openai.NewClient(os.Getenv("OPENAI_API_KEY"),
    openai.WithModel("whisper-1"),
    openai.WithTimeout(30*time.Second))
defer c.Close()
```

API key is sent as Bearer auth; pass `""` to omit the header for
private deployments that accept anonymous traffic. TLS verification is
on by default; `WithTLSInsecureSkipVerify()` is available for local-LAN
testing only.

### whisper.cpp

```go
c := whispercpp.NewClient(
    whispercpp.WithEndpoint("http://127.0.0.1:8081/v1/audio/transcriptions"))
defer c.Close()
```

No auth header is sent. The model field is required by the protocol but
ignored by `whisper-server`; the default `"whisper-1"` is the
conventional placeholder.

## Module layout

```
asrclient/
├── client.go              # Backend, Options, Transcript, Segment
├── audio.go               # frame-format constants
├── wyoming/               # Wyoming wire protocol + Backend impl
├── openai/                # OpenAI HTTPS Backend
├── whispercpp/            # OpenAI-protocol Backend, loopback defaults
└── internal/
    └── httpcore/          # shared multipart/form-data POST core
```

`wyoming/` keeps zero non-stdlib imports beyond the parent package's
types — it can be lifted into its own module if a consumer ever needs
just the wire protocol without the rest of asrclient.

## Development

```sh
task            # list tasks
task test       # go test ./...
task test:race  # go test -race ./... (requires CGo)
task check      # vet + fmt + lint + test + vuln
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution guide and
[SECURITY.md](SECURITY.md) for vulnerability reporting.

## License

Apache-2.0 — see [LICENSE](LICENSE).
