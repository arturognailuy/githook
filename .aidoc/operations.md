---
domain: Workflows
status: Active
entry_points:
  - cmd/githook/main.go
  - packaging/systemd/githook-receiver.service
  - packaging/systemd/githook-worker.service
dependencies:
  - .aidoc/architecture.md
---

# Operations

Githook runs from user-owned files under a user-level systemd manager. Only the receiver and worker credentials live under root-controlled `/etc/githook`; runtime state and releases stay in the service user's home directory.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Security and serialization boundaries the host configuration preserves |
| [INDEX](INDEX.md) | Documentation discovery |

## Why Runtime Files Stay User-Owned

The deployment host is an artifact consumer rather than a source checkout or build host. Installation copies a reviewed binary and user-level service definitions onto the host, while GitHub Actions remains the build environment for release artifacts. Root-owned storage is reserved for credentials; giving ordinary state, units, or deployed content root ownership would add privilege without protecting a secret.

The queue service has no application authentication because it listens only on loopback. A separately managed reverse proxy exposes exactly the configured webhook path; every maintenance path remains reachable only by a local operator. Githook accepts forwarded requests independently of the proxy implementation and does not own proxy configuration. Exposing the whole listener would violate the security model.

## Installation Layout

Install the binary as `~/.local/bin/githook` and the unit templates from `packaging/systemd/` under `~/.config/systemd/user/`. Copy `packaging/config/runtime.conf.example` to `~/.config/githook/runtime.conf` and set the repository, workflow, branch, artifact prefix, smoke URLs, and any host-specific paths. The queue database defaults to `~/.githook/githook.db`; release directories and the `current` symlink default to `~/.local/share/githook/`. `OpenQueue` creates the private queue directory when needed. Repository ignore rules reject common SQLite database, journal, and WAL filenames, so runtime queue state cannot be added accidentally. A static-file server may read the configured release tree but does not belong to Githook and needs no write access.

Create `/etc/githook/receiver.conf` from `packaging/config/receiver.conf.example` and `/etc/githook/worker.conf` from `packaging/config/worker.conf.example`. The receiver file contains only `GITHOOK_WEBHOOK_SECRET`; the worker file contains only `GITHUB_TOKEN`. Keep both files root-owned, grant read access only to the service account through a dedicated group, and keep `/etc/githook` non-writable by that account. Each unit reads only its own credential file.

Run `systemctl --user daemon-reload`, enable and start `githook-receiver.service` and `githook-worker.service`, then enable `githook-reconcile.timer`. Enable lingering for the service account so its user manager starts at boot without an interactive login. Host-specific path permissions belong in user-level systemd drop-ins, not the credential files.

The external reverse proxy forwards only `POST /hooks/github` (or the configured webhook path) to the loopback listener. The proxy may be Caddy, Nginx, IIS, or another HTTP proxy; its configuration belongs to the consuming deployment, not this project. The proxy must not expose maintenance paths or receive write access to the queue, release directories, or credential files.

## Configuration Ownership

`cmd/githook.main` defines host-neutral defaults for the webhook path, loopback address, queue path, branch, artifact prefix, and release paths. Repository and workflow identity are required configuration. `~/.config/githook/runtime.conf` owns non-secret deployment policy, while `/etc/githook/*.conf` contains only credentials. This keeps credential files from becoming duplicate application configuration.

`Worker.Run` is the source of truth for claim recovery, retry classification, and retry timing; `Queue` is the source of truth for serialization and retained failure state.

## How to Inspect and Edit Pending Work

The loopback queue service provides local maintenance operations for listing, peeking, dropping, or coalescing queued work. `Service.ServeHTTP` is the source of truth for the methods and paths; queue deletion never removes the request currently being processed.

Stop the worker before maintenance when an operator needs a stable pending set. Restarting the worker requeues a request left in `processing`.

## How to Recover

`githook deploy-run --sha <full-sha> <run-id>` performs the same authoritative verification and deployment path for manual replay. Reconciliation can independently deploy a newer eligible run without deleting failed evidence.

Rotate the webhook secret by installing the new receiver credential before updating the one existing GitHub webhook, then prove a signed `ping`. Rotate the GitHub token independently because the queue service never uses it.

Retain previous immutable release directories until a newer release survives smoke and restart checks. A failed smoke check restores the previous active symlink; host recovery may also repoint `current` to a retained release.
