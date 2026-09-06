---
domain: Architecture
status: Active
entry_points:
  - cmd/githook/main.go
dependencies: []
---

# Githook Documentation

Githook is a self-contained, host-local GitHub Actions artifact deployer. This index routes maintainers to the trust boundaries and operating workflow that code alone cannot explain.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Components, trust boundaries, and deployment invariants |
| [Operations](operations.md) | Installation, recovery, rotation, and failure handling |

## Reading Chains

- **Change receiver or worker behavior:** Architecture → `internal/githook/receiver.go` or `internal/githook/worker.go` → tests.
- **Change deployment or artifact validation:** Architecture → `internal/githook/artifact.go` → `internal/githook/deploy.go` → tests.
- **Install or recover a host:** Operations → `packaging/systemd/` → `cmd/githook/main.go`.
