---
domain: Workflows
status: Active
entry_points:
  - cmd/githook/main.go
  - packaging/config/runtime.conf.example
  - packaging/systemd/githook-receiver.service
dependencies:
  - .aidoc/architecture.md
---

# Host Bootstrap

Githook can be rebuilt on an empty supported Linux host from this repository plus host-owned credentials and deployment policy. The sequence preserves the receiver/worker privilege boundary and proves a real artifact deployment before the host is considered recovered.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Explains the trust, queue, and activation invariants |
| [Operations](operations.md) | Day-two maintenance after bootstrap succeeds |
| [INDEX](INDEX.md) | Documentation discovery and reading chains |

## Why Bootstrap Has Explicit Gates

A daemon that starts successfully can still be unsafe or unable to deploy. Bootstrap therefore proves configuration syntax, credential separation, loopback binding, queue durability, artifact verification, release permissions, smoke behavior, and restart persistence in order.

Hostnames, public routes, release paths, service accounts, and credentials are deployment inputs rather than Githook defaults. Keep those values in the host or infrastructure manager so this repository remains reusable and safe to publish.

## Prerequisites

- A supported Linux host with a non-root service account, key-only administrative access, current security updates, and a host firewall.
- Go 1.25 or newer for a source build, or a binary built from a reviewed repository commit.
- A static web server that can read the chosen active-release path without write access.
- One GitHub repository workflow that builds an immutable artifact matching the manifest and checksum contract in `VerifyBundle`.
- A human operator who can generate and enter a new webhook secret and a GitHub credential scoped only to read Actions runs and artifacts for the configured repository. Credentials are not repository files and must not be supplied through chat, logs, or source control.

## Install Repository-Owned Files

1. Build `./cmd/githook` from a reviewed commit after `go test ./...`, `go vet ./...`, and `go build ./cmd/githook` pass.
2. Install the binary as `~/.local/bin/githook` for the service account.
3. Copy `packaging/config/runtime.conf.example` to `~/.config/githook/runtime.conf`. Replace repository, workflow name and path, branch, artifact prefix, smoke URLs, and optional deployment paths with host-owned values.
4. Create `/etc/githook/receiver.conf` and `/etc/githook/worker.conf` from the templates in `packaging/config/`, then have the human operator enter newly generated values through a trusted host credential workflow. The receiver file receives only the webhook secret; the worker file receives only the GitHub token. Restrict both files to root and a dedicated read-only credentials group that contains the service account. Do not ask an AI agent to read, copy, or relay either value.
5. Copy `packaging/systemd/` into the service account's `~/.config/systemd/user/`. If releases are outside the default user-owned directory, add that exact release root to the worker and reconciliation units' `ReadWritePaths` through host-owned drop-ins.
6. Reload the user manager, enable and start the receiver and worker, and enable the reconciliation timer. Enable lingering for the service account so the user manager starts at boot without an interactive session.

## Connect the Public Adapter

Bind `GITHOOK_LISTEN` to a loopback address. Configure the external reverse proxy to forward only `POST` on the exact `GITHOOK_WEBHOOK_PATH`; do not forward the listener root or maintenance paths.

Configure the repository's one existing webhook with that public HTTPS route, JSON content type, TLS verification, the receiver's matching secret, and only workflow-run events. Never put the webhook secret or GitHub token in command arguments, logs, unit files, source control, or the non-secret runtime file.

## Prove the Installation

1. Validate unit syntax and confirm the receiver, worker, and reconciliation timer are enabled and active.
2. Confirm the listener exists only on loopback and the queue database is private to the service account.
3. Confirm an unsigned request is rejected and a signed GitHub ping is accepted without creating deployable queue work.
4. Trigger or re-run a successful configured workflow. Confirm in GitHub that the run's event, branch, workflow identity, head SHA, conclusion, and artifact match the runtime policy.
5. Confirm the completed webhook is accepted, one queue record is processed, one immutable release is created, and the active symlink advances only after all smoke URLs succeed.
6. Replay the completed delivery and confirm deduplication prevents a second deployment.
7. Restart both daemons and reboot the host. Confirm the services return without login, the queue is healthy, the active release still serves, and the reconciliation timer remains scheduled.

## Recover Without a Webhook

Use `githook deploy-run --sha <full-sha> <run-id>` under the worker configuration when a known successful run must be replayed. The command performs the same authoritative GitHub lookup, artifact verification, extraction, activation, and smoke path as queued work.

Use `githook reconcile` when the host must discover the newest eligible successful run from the configured workflow and branch. Reconciliation is also the boot/daily safety net for a missed webhook; it does not make webhook payloads authoritative.

## Failure and Rollback Checks

An invalid signature must fail before JSON is trusted. An invalid repository, workflow, branch, conclusion, SHA, digest, manifest, checksum, archive path, or artifact count must not activate a release.

A smoke failure must restore the previous active symlink. Preserve the failed queue record and diagnostic error, keep the previous immutable release, and fix configuration or inputs before replaying; do not bypass verification by copying extracted files into the active tree.

## Recovery Completion Criteria

- The receiver has only the webhook secret and queue access; the worker has only the GitHub token and deployment access.
- Public routing exposes exactly one webhook method/path, while all maintenance endpoints remain local.
- A real successful workflow deploys exactly once, the smoke checks pass, and the queue drains.
- The static server can read but not modify releases; completed release directories are immutable.
- Service restart and host reboot recover the receiver, worker, timer, active release, and public health check without operator login.
