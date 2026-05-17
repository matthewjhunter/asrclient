# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities privately through GitHub's
[private vulnerability reporting][gh-pvr] for this repository. Do not
open a public issue or pull request for security problems.

A maintainer will acknowledge your report and coordinate a fix and
disclosure timeline with you.

[gh-pvr]: https://github.com/matthewjhunter/asrclient/security/advisories/new

## Supported Versions

The latest tagged release receives security fixes. Older releases may
receive backports at the maintainer's discretion.

## Scope

In scope: the `asrclient` library and its three backends (`wyoming`,
`openai`, `whispercpp`) and the shared `internal/httpcore` package.

Out of scope: vulnerabilities in upstream services (the OpenAI API,
`wyoming-faster-whisper`, `whisper-server` itself). Those should be
reported to their respective projects. If a server-side vulnerability
materially affects this client, please still let us know so we can
warn users or work around it.

## Operational notes

A few things consumers should be aware of when integrating asrclient:

- **`WithTLSInsecureSkipVerify` is an opt-in escape hatch.** It exists
  for local-LAN testing only. Do not enable it for production traffic
  that crosses untrusted networks.
- **API keys.** The `openai` backend takes the API key as a constructor
  argument. The library never logs or persists it; key management is
  the caller's responsibility.
- **whisper.cpp loopback default.** `whispercpp.NewClient()` defaults
  to `http://127.0.0.1:8080`. Confirm your consumer either keeps
  loopback or explicitly opts in to a remote endpoint via
  `WithEndpoint()`; the library does not warn about insecure HTTP to
  non-loopback addresses.
- **Wyoming has no transport security.** The protocol is plain TCP.
  Do not run a Wyoming server over an untrusted network without a TLS
  tunnel.
