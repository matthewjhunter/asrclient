# Contributing

Open an issue before sending a PR for substantial changes — easier to
agree on the approach up front than to rework it in review.

## Local development

```sh
task            # list tasks
task test       # go test ./...
task test:race  # go test -race ./... (requires CGo)
task check      # vet + fmt + lint + test + vuln
```

`task check` must pass before a PR will be merged. Requires Go 1.25.10
or newer.

## Conventions

- Run `task fmt:fix` before committing.
- **Pure Go, no CGo.** The module must build cleanly with
  `CGO_ENABLED=0`. The race detector (which requires CGo) is opt-in
  via `task test:race`; CI runs both.
- **Protocol clients only.** Spawning servers, port discovery,
  `/health` gating, restart-on-crash — those belong in the consumer,
  not here. Keeping protocol and lifecycle separate is the reason this
  module exists.
- **Narrow public surface.** Don't grow `Backend`, `Options`,
  `Transcript`, or `Segment` speculatively. Add fields when a real
  consumer needs them.

## API stability

The v0.x series is **not API-stable** — see the README. Breaking
renames and shape changes are expected as the interface settles.
Bug-for-bug compatibility is not a goal in v0.x. Tests should cover
behavior, not exact field names or method signatures of the public
API more than necessary.

When proposing API changes, include the rationale: which consumer
needs it, why the current shape doesn't work, what other backends
would have to do to support it.

## Adding a backend

1. Create a sub-package (e.g. `mything/`).
2. Implement the `asrclient.Backend` interface (`Transcribe`,
   `Healthy`, `Close`).
3. Add a constructor `NewClient(...) *Client` plus `Option` /
   `WithX(...)` helpers for configuration.
4. Add a compile-time assertion: `var _ asrclient.Backend = (*Client)(nil)`.
5. Write tests against an in-process `httptest.Server` (or `net.Pipe`
   for protocol-level work) — no live external services in CI.

## Reporting issues

Bugs and feature requests: open a [GitHub issue][issues].

Security vulnerabilities: see [SECURITY.md](SECURITY.md). Don't open
public issues for security problems.

[issues]: https://github.com/matthewjhunter/asrclient/issues
