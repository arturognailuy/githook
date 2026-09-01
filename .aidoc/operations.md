---
domain: Workflows
status: Active
entry_points:
  - cmd/githook/main.go
dependencies:
  - .aidoc/architecture.md
---

# Operations

Githook is installed as one versioned binary executed by separate receiver and worker services. Host-owned configuration supplies paths and credentials; repository files never contain secrets or live service definitions.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Security boundaries the host configuration preserves |
| [INDEX](INDEX.md) | Documentation discovery |

## Why Operations Stay Host-Owned

The production VM is a static-site appliance rather than a source checkout or build host. Installation copies a reviewed binary and service configuration onto the VM, while GitHub Actions remains the only build environment for site artifacts.

## What Operators Configure

The receiver identity needs `GITHOOK_WEBHOOK_SECRET`, `GITHOOK_DATABASE`, `GITHOOK_REPOSITORY`, and a loopback `GITHOOK_LISTEN`. The worker identity needs a read-only `GITHUB_TOKEN`, the same database through constrained queue permissions, release/current paths, workflow policy, and local/public smoke URLs.

The Caddy route forwards only the exact public webhook path to `127.0.0.1:20182`. Cloudflare may proxy the public hostname: Cloudflare terminates the client connection and forwards the request to the configured origin, so the origin receiver still handles the webhook. A direct DNS hostname is a fallback only if a Cloudflare WAF or challenge policy cannot exempt the exact webhook path.

## How to Recover

Restarting the worker requeues jobs left in `processing`. `githook deploy-run --sha <full-sha> <run-id>` performs the same authoritative verification and deployment path for manual replay.

Rotate the webhook secret by installing the new receiver secret before updating the one existing GitHub webhook, then prove a signed `ping`. Rotate the GitHub token independently because the receiver never uses it.

Retain previous immutable release directories until a newer release survives smoke and restart checks. A failed smoke check restores the previous active symlink; host recovery may also repoint `current` to a retained release.
