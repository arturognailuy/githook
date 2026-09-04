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

The production VM is a static-site appliance rather than a source checkout or build host. Installation copies a reviewed binary and user-level service definitions onto the VM, while GitHub Actions remains the only build environment for site artifacts. Root-owned storage is reserved for credentials; giving ordinary state, units, or web content root ownership would add privilege without protecting a secret.

The queue service has no application authentication because it listens only on loopback. Caddy exposes exactly the configured webhook path on `direct.gnailuy.com`; every maintenance path remains reachable only by a local operator. Exposing the whole listener would violate the security model.

## Installation Layout

Install the binary as `~/.local/bin/githook` and the unit templates from `packaging/systemd/` under `~/.config/systemd/user/`. The queue database defaults to `~/.githook/githook.db`; release directories and the `current` symlink default to `~/www/gnailuy/`. `OpenQueue` creates the private queue directory when needed. Repository ignore rules reject common SQLite database, journal, and WAL filenames, so runtime queue state cannot be added accidentally. Caddy needs read and directory-traversal access to the web root but no write access.

Create `/etc/githook/receiver.conf` from `packaging/config/receiver.conf.example` and `/etc/githook/worker.conf` from `packaging/config/worker.conf.example`. The receiver file contains only `GITHOOK_WEBHOOK_SECRET`; the worker file contains only `GITHUB_TOKEN`. Keep both files root-owned, grant read access only to the service account through a dedicated group, and keep `/etc/githook` non-writable by that account. Each unit reads only its own credential file.

Run `systemctl --user daemon-reload`, enable and start `githook-receiver.service` and `githook-worker.service`, then enable `githook-reconcile.timer`. Enable lingering for the service account so its user manager starts at boot without an interactive login. The unit files hold non-secret defaults; host-specific overrides belong in user-level systemd drop-ins, not the credential files.

The Caddy route forwards only `POST /hooks/github/gnailuy.com` to the loopback listener. Caddy can read the user-owned web root through a narrowly scoped group or filesystem ACL. Caddy must not receive write access to the queue, release directories, or credential files.

## Configuration Ownership

`cmd/githook.main` defines ordinary defaults for the repository, workflow, branch, webhook path, loopback address, queue path, and release paths. The systemd templates set only operational values that need an explicit host policy, such as smoke URLs. This keeps non-secret and derivable values out of `/etc/githook` and prevents credential files from becoming duplicate application configuration.

`Worker.Run` is the source of truth for claim recovery, retry classification, and retry timing; `Queue` is the source of truth for serialization and retained failure state.

## How to Inspect and Edit Pending Work

The loopback queue service provides local maintenance operations for listing, peeking, dropping, or coalescing queued work. `Service.ServeHTTP` is the source of truth for the methods and paths; queue deletion never removes the request currently being processed.

Stop the worker before maintenance when an operator needs a stable pending set. Restarting the worker requeues a request left in `processing`.

## How to Recover

`githook deploy-run --sha <full-sha> <run-id>` performs the same authoritative verification and deployment path for manual replay. Reconciliation can independently deploy a newer eligible run without deleting failed evidence.

Rotate the webhook secret by installing the new receiver credential before updating the one existing GitHub webhook, then prove a signed `ping`. Rotate the GitHub token independently because the queue service never uses it.

Retain previous immutable release directories until a newer release survives smoke and restart checks. A failed smoke check restores the previous active symlink; host recovery may also repoint `current` to a retained release.
