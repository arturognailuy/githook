# Agent Instructions

- Read `.aidoc/INDEX.md` before changing runtime behavior.
- Preserve the queue-service/worker privilege boundary: the loopback queue service owns only the webhook secret and embedded queue; the worker owns only GitHub artifact-read and release activation authority.
- Treat webhook payloads as notifications. Re-read deployment facts from GitHub before downloading or activating anything.
- Keep the configured listener and every queue maintenance endpoint loopback-only; Caddy may forward only the exact webhook path. Keep secrets out of source, arguments, logs, fixtures, and documentation.
- Run `go test ./...`, `go vet ./...`, and `go build ./cmd/githook` before opening a pull request.
- Update `.aidoc/` whenever architecture, security invariants, or operator workflows change.
