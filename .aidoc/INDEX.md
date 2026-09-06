---
domain: Architecture
status: Active
entry_points:
  - cmd/githook/main.go
dependencies: []
---

# Githook Documentation

Githook is a self-contained, host-local GitHub Actions artifact deployer. This index is the entry point for Githook itself; a consuming system's repository owns the cross-system installation order and host-specific integration.

## Related Docs

| Document | Relationship |
|---|---|
| [Architecture](architecture.md) | Components, trust boundaries, and deployment invariants |
| [Host Bootstrap](host-bootstrap.md) | Install or recover Githook on an empty supported Linux host |
| [Operations](operations.md) | Run, inspect, rotate, replay, and recover an existing installation |

## Reading Chains

- **Understand or change Githook:** Architecture → relevant `internal/githook/` package → tests.
- **Install or recover Githook:** Architecture → Host Bootstrap → `packaging/config/` → `packaging/systemd/`.
- **Perform daily maintenance:** Operations → the referenced command or implementation entry point.
