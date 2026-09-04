---
domain: Workflows
status: Active
entry_points:
  - cmd/githook/main.go
  - internal/githook/service.go
dependencies:
  - .aidoc/architecture.md
---

# Operations

Githook is installed as one versioned binary executed by a loopback queue service and one deployment worker. Host-owned configuration supplies paths and credentials; repository files never contain secrets or live service definitions.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Security and serialization boundaries the host configuration preserves |
| [INDEX](INDEX.md) | Documentation discovery |

## Why Operations Stay Host-Owned

The production VM is a static-site appliance rather than a source checkout or build host. Installation copies a reviewed binary and service configuration onto the VM, while GitHub Actions remains the only build environment for site artifacts.

The queue service has no application authentication because it listens only on loopback. Caddy exposes exactly the configured webhook path on `direct.gnailuy.com`; every maintenance path remains reachable only by a local operator. Exposing the whole listener would violate the security model.

## What Operators Configure

The queue-service identity needs `GITHOOK_WEBHOOK_SECRET`, `GITHOOK_DATABASE`, `GITHOOK_REPOSITORY`, `GITHOOK_WEBHOOK_PATH`, and a loopback `GITHOOK_LISTEN`. The deployment-worker identity needs a read-only `GITHUB_TOKEN`, access to the same embedded queue, release/current paths, workflow policy, and local/public smoke URLs.

The worker is a single long-running daemon. `Worker.Run` recovers an interrupted claim after restart, waits when no job is available, claims one request, completes or retries it, and then waits again. The queue enforces one processing request even if service supervision accidentally starts a second worker.

The Caddy route forwards only `POST /hooks/github/gnailuy.com` to `127.0.0.1:20182`. Every other public host, path, or method returns the configured dummy response without reaching local maintenance operations.

## How to Inspect and Edit Pending Work

The loopback queue service provides these local-only operations:

| Method and path | Effect |
|---|---|
| `GET /maintenance/queue` | List queued and currently processing requests, newest first |
| `GET /maintenance/queue/peek` | Show the next queued request without claiming it |
| `DELETE /maintenance/queue/<delivery-id>` | Drop one queued request |
| `DELETE /maintenance/queue` | Clear all queued requests |
| `POST /maintenance/queue/keep-one` | Clear queued requests except the newest run |

Queue deletion never removes the request currently being processed. Stopping the worker before maintenance gives an operator a stable pending set when that distinction matters.

## How to Recover

Restarting the worker requeues a request left in `processing`. `githook deploy-run --sha <full-sha> <run-id>` performs the same authoritative verification and deployment path for manual replay.

Rotate the webhook secret by installing the new queue-service secret before updating the one existing GitHub webhook, then prove a signed `ping`. Rotate the GitHub token independently because the queue service never uses it.

Retain previous immutable release directories until a newer release survives smoke and restart checks. A failed smoke check restores the previous active symlink; host recovery may also repoint `current` to a retained release.
