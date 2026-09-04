# Githook

Githook is a small Go service that receives signed GitHub `workflow_run` notifications and deploys verified immutable `gnailuy.com` artifacts on the same host.

One binary runs as two daemons with separate trust domains:

- `githook serve` validates raw-body HMAC-SHA256 signatures, writes a durable embedded SQLite queue, and exposes local-only queue maintenance endpoints.
- `githook worker` waits behind that queue, processes exactly one request at a time, and re-reads authoritative run metadata and artifacts from GitHub before atomic activation.

The worker rejects permanent metadata or artifact-integrity failures immediately. Transient GitHub, network, deployment, and smoke-check failures use five bounded retries after 1, 2, 4, 8, and 16 minutes; exhausted requests remain inspectable as failed queue records.

The same binary also provides:

- `githook reconcile`, which discovers the newest eligible successful run and processes it through the same path.
- `githook deploy-run --sha <sha> <run-id>`, which replays one run through the same validation path.

The service listens only on loopback. Caddy forwards the exact public webhook path; queue inspection and maintenance remain local and require no separate authentication. Webhook responses use the dummy body `42` for both accepted and rejected requests.

Start with [`AGENT.md`](AGENT.md), then use [`.aidoc/INDEX.md`](.aidoc/INDEX.md) for architecture and operating context.

## Development

Requires Go 1.25 or newer. Run `go test ./...`, `go vet ./...`, and `go build ./cmd/githook` before review.

Live credentials and host service definitions are intentionally not stored in this repository. Production uses two root-owned mode-0600 environment files: `/etc/githook/receiver.env`, loaded only by the receiver for its webhook secret and listener settings, and `/etc/githook/worker.env`, loaded only by the worker for its GitHub token and deployment settings.
