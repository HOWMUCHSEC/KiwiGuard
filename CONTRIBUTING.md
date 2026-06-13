# Contributing to KiwiGuard

Thank you for helping improve KiwiGuard. This project aims to hold a high engineering bar because it sits in the security path for LLM applications.

## Development Setup

Prerequisites:

- Go 1.25 or newer.
- Docker with Compose support.
- Node.js 22 or newer.
- pnpm 10 or newer.

Install frontend dependencies:

```bash
pnpm -C web install
```

Start local infrastructure:

```bash
cp .env.example .env
docker compose -f deployments/docker-compose.yml --env-file .env up -d
```

## Local Verification

For routine development:

```bash
make test
```

Before opening a pull request:

```bash
make verify
```

Useful focused checks:

```bash
make test-go
make test-go-race
make test-go-cover
make bench-go
make lint-go
make vuln-go
make tidy-check
make test-web
make build-web
make docker-config
make standards-check
```

## Go Standards

All Go code must follow `docs/go-standards.md`.

Important expectations:

- Keep code simple, idiomatic, and easy to review.
- Document exported Go symbols.
- Keep error messages lowercase and wrap propagated errors with `%w`.
- Use `gofmt`.
- Prefer standard library solutions where practical.
- Keep the gateway hot path free of database reads.
- Preserve Go 1.25+ semantics, including `sync.WaitGroup.Go` where a wait group starts goroutines.

## Testing Expectations

Pull requests should include tests for behavior changes. For documentation and repository metadata changes, run the relevant standards checks and explain what was verified.

Security-sensitive behavior should use synthetic test data only.

## Pull Requests

Before requesting review:

- Keep the change scoped.
- Update documentation when behavior or workflows change.
- Run `make verify`, or list any check that could not be run and why.
- Do not include `.DS_Store`, local environment files, secrets, generated coverage files, or `web/node_modules`.

## Security Reports

Do not include vulnerability details, secrets, prompts, responses, or customer data in public issues or pull requests. Follow `SECURITY.md`.
