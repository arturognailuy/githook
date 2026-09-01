---
domain: Architecture
status: Active
entry_points:
  - internal/githook/receiver.go
  - internal/githook/worker.go
  - internal/githook/deploy.go
dependencies:
  - .aidoc/operations.md
---

# Architecture

Githook turns a signed `workflow_run` notification into a verified immutable release without trusting webhook claims or building source on the production host. Separate receiver and worker processes deliberately prevent public input from inheriting deployment authority.

## Related Docs

| Document | Relationship |
|---|---|
| [Operations](operations.md) | Host installation and recovery workflow |
| [INDEX](INDEX.md) | Documentation discovery |

## Why Githook Is Split

The receiver must process attacker-controlled Internet requests, while the worker must read private API credentials and change the active release. Running `serve` and `worker` as separate identities limits a receiver compromise to bounded queue input and prevents the worker from becoming a public network service.

The webhook is only a wake-up signal because signed payloads can be valid yet stale, duplicated, or delivered out of order. `Worker.ProcessRun` re-reads the run and artifact list from GitHub before deployment.

## What Githook Owns

`Receiver.ServeHTTP` verifies the raw-body HMAC-SHA256 signature before JSON parsing, enforces the request envelope, and deduplicates delivery and run identifiers in SQLite. Signed `ping` requests prove reachability without creating work.

`Worker.ProcessRun` accepts only the configured repository, workflow, push event, branch, successful conclusion, expected head SHA, and one matching unexpired artifact. `VerifyBundle` checks GitHub's outer digest, the manifest, the inner checksum, and archive path safety before `Deployer.Deploy` creates a release.

`Deployer.Deploy` activates a complete release with an atomic symlink replacement. Post-activation smoke failure restores the previous link rather than leaving a partly trusted release active.

## Invariants

- The receiver MUST NOT receive a GitHub API token or release-directory write access.
- The worker MUST NOT receive the webhook secret or expose an HTTP listener.
- A webhook payload MUST NOT directly authorize a deployment.
- Release archives MUST contain only relative regular files and directories; links and traversal paths are rejected.
- A release directory MUST be immutable after creation, and activation MUST use one atomic link replacement.
- Duplicate delivery IDs, duplicate run IDs, and older releases MUST NOT replace newer deployed state.
