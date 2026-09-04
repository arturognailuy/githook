---
domain: Architecture
status: Active
entry_points:
  - internal/githook/service.go
  - internal/githook/worker.go
  - internal/githook/deploy.go
dependencies:
  - .aidoc/operations.md
---

# Architecture

Githook turns a signed `workflow_run` notification into a verified immutable release without trusting webhook claims or building source on the production host. One loopback queue service and one deployment worker keep the runtime small while preserving the public-input/deployment-authority boundary.

## Related Docs

| Document | Relationship |
|---|---|
| [Operations](operations.md) | Host installation, queue maintenance, and recovery workflow |
| [INDEX](INDEX.md) | Documentation discovery |

## Why Githook Uses Two Daemons

The queue service processes attacker-controlled Internet requests forwarded by one exact Caddy route. The deployment worker reads a GitHub token and changes the active release. Separate host identities prevent public input from inheriting deployment authority and prevent the privileged worker from becoming a network service.

The embedded SQLite file gives the low-volume queue durable transactions without Redis or a database server. SQLite remains an implementation detail owned by the queue service and worker; no separate database lifecycle exists.

The webhook is only a wake-up signal because signed payloads can be valid yet stale, duplicated, or delivered out of order. `Worker.ProcessRun` re-reads the run and artifact list from GitHub before deployment.

## What the Queue Service Owns

`Service.ServeHTTP` exposes the configured webhook path and local maintenance paths on one loopback listener. Caddy forwards only the webhook path, so maintenance operations never cross the public routing boundary.

`Receiver.ServeHTTP` verifies the raw-body HMAC-SHA256 signature before JSON parsing, enforces the request envelope, and deduplicates delivery and run identifiers. Every webhook response body is the same dummy value, `42`, while HTTP status codes retain operational meaning.

`Queue.Claim` permits only one `processing` record. The single worker therefore takes one request at a time, even if another worker process starts accidentally. Newer workflow runs are claimed first so a burst converges on the newest eligible site release.

## What the Worker Owns

`Worker.ProcessRun` accepts only the configured repository, workflow, push event, branch, successful conclusion, expected head SHA, and one matching unexpired artifact. `VerifyBundle` checks GitHub's outer digest, the manifest, the inner checksum, and archive path safety before `Deployer.Deploy` creates a release.

`Deployer.Deploy` activates a complete release with an atomic symlink replacement. Post-activation smoke failure restores the previous link rather than leaving a partly trusted release active.

## Invariants

- The queue service MUST NOT receive a GitHub API token or release-directory write access.
- The worker MUST NOT receive the webhook secret or expose an HTTP listener.
- The configured listener and all maintenance endpoints MUST remain loopback-only; Caddy MUST forward only the exact webhook path.
- A webhook payload MUST NOT directly authorize a deployment.
- At most one queue record may have `processing` status.
- Queue maintenance MUST NOT delete the request currently being processed.
- Release archives MUST contain only relative regular files and directories; links and traversal paths are rejected.
- A release directory MUST be immutable after creation, and activation MUST use one atomic link replacement.
- Duplicate delivery IDs, duplicate run IDs, and older releases MUST NOT replace newer deployed state.
