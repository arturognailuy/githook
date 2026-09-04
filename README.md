# Githook

Githook receives signed GitHub Actions notifications and deploys verified immutable `gnailuy.com` artifacts.

See [`.aidoc/INDEX.md`](.aidoc/INDEX.md) for architecture, installation, and operations.

## Development

Requires Go 1.25 or newer. Run `go test ./...`, `go vet ./...`, and `go build ./cmd/githook` before review.
