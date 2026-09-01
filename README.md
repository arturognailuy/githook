# Githook

Githook is a Go service that receives signed GitHub `workflow_run` notifications and deploys verified immutable `gnailuy.com` artifacts on the same host.

One binary exposes separate trust domains:

- `githook serve` validates raw-body HMAC-SHA256 signatures and writes a durable SQLite queue.
- `githook worker` re-reads authoritative run metadata and artifacts from GitHub before atomic activation.
- `githook reconcile` discovers the newest eligible successful run and processes it through the same path.
- `githook deploy-run --sha <sha> <run-id>` replays one run through the same validation path.

Start with [`AGENT.md`](AGENT.md), then use [`.aidoc/INDEX.md`](.aidoc/INDEX.md) for architecture and operating context.

## Development

Requires Go 1.25 or newer. Run `go test ./...`, `go vet ./...`, and `go build ./cmd/githook` before review.

Live credentials and host service definitions are intentionally not stored in this repository.
