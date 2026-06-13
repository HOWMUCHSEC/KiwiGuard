# KiwiGuard

KiwiGuard is an OpenAI-compatible LLM guardrail gateway written in Go. It inspects LLM inputs and outputs, evaluates configurable security policies, routes traffic through a vertical security verdict provider, and exposes a management console for policy and operations workflows.

KiwiGuard is in an early open-source foundation stage. The current codebase is suitable for architecture review, local development, and early integration experiments. Production deployments still need environment-specific controls for authentication, authorization, retention, network isolation, and operational monitoring.

## What Is Included

- OpenAI-compatible gateway endpoints for chat completions and responses.
- Regex and built-in PII-style detectors.
- Immutable policy snapshot evaluation for gateway hot paths.
- Default vertical security model verdict routing.
- SSE streaming inspection.
- PostgreSQL-backed configuration schema.
- Asynchronous ClickHouse traffic and security event pipeline.
- Control API contracts and a React + TypeScript console.
- CI quality gates for tests, race detection, linting, vulnerability checks, frontend verification, and Docker Compose validation.

## Architecture

KiwiGuard uses a split config and event pipeline architecture:

- The gateway hot path lives in Go on top of `net/http`.
- Policy bundles compile into immutable snapshots before request handling.
- Detector findings and verdict results drive allow, block, and monitor decisions.
- PostgreSQL is the canonical configuration store.
- ClickHouse is the first-class traffic and security event sink.
- Gateway request handling writes through an asynchronous event boundary and does not query databases on the hot path.
- Gateway and control services expose Prometheus-compatible metrics at `/metrics`.
- Gateway, control, and event sink boundaries emit OpenTelemetry spans when a tracer provider is configured by the embedding runtime.
- The control API and web console provide policy, route, verdict, and regex-test workflows.

For enterprise deployment guidance, start with `docs/README.md`.

## Local Services

Copy the example environment file:

```bash
cp .env.example .env
```

Start PostgreSQL and ClickHouse:

```bash
docker compose -f deployments/docker-compose.yml --env-file .env up -d
```

Validate the Compose stack:

```bash
make docker-config
```

## Development

Prerequisites:

- Go 1.25 or newer.
- Docker with Compose support.
- Node.js 22 or newer.
- pnpm 10 or newer.

Install frontend dependencies:

```bash
pnpm -C web install
```

Run routine checks:

```bash
make test
```

Run maintainer-grade verification:

```bash
make verify
```

Build the Go service container image:

```bash
make docker-image
make docker-image-smoke IMAGE=ghcr.io/howmuchsec/kiwiguard:dev
```

The default image is `ghcr.io/howmuchsec/kiwiguard:dev`. Override `IMAGE_REPOSITORY`, `IMAGE_TAG`, `IMAGE`, or `DOCKER_BUILD_PLATFORM` for release and multi-architecture workflows.

Validate the production Compose deployment package:

```bash
make docker-production-config
```

Production deployment guidance is available in `docs/production-checklist.md` and `docs/production-checklist.zh-CN.md`.

Focused checks:

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
make build-go
make docker-config
make standards-check
```

Inspect CLI commands:

```bash
go run ./cmd/kiwiguard --help
go run ./cmd/kiwiguard --dry-run serve
```

Launch the local development environment:

```bash
make dev-env
```

This starts PostgreSQL, ClickHouse, a mock OpenAI-compatible LLM API, KiwiGuard gateway/control/worker services, and the web console. Defaults use `http://127.0.0.1:18080` for the gateway, `http://127.0.0.1:18081` for the control API, `http://127.0.0.1:18082` for the mock LLM API, and `http://127.0.0.1:5173` for the console.

Scrape operational metrics from either runtime HTTP service:

```bash
curl http://127.0.0.1:18080/metrics
curl http://127.0.0.1:18081/metrics
```

The metrics include HTTP request totals and latency, gateway traffic outcomes, detector/verdict latency, event sink batch results, and durable spool depth/capacity/overflow gauges.

OpenTelemetry tracing is centralized in `internal/observability`. The default CLI runtime uses the Go OpenTelemetry global tracer provider, which is no-op unless an embedding runtime or future exporter configuration installs a provider. Current spans cover gateway/control HTTP lifecycles, gateway event enqueue, and event sink batch writes.

In another terminal, send a request through the gateway and verify ClickHouse captured input/output event metadata:

```bash
make dev-client-smoke
```

To run a real OpenAI-compatible upstream smoke, start the dev environment with the credential available to the KiwiGuard process, then run the beta smoke from another terminal:

```bash
KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make dev-env

KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make beta-openai-smoke
```

The smoke rewires the dev `dev-openai` provider to use `credential_ref`, creates a temporary gateway client, sends one authenticated chat completion through KiwiGuard, and revokes the client. Use `KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF` or `BETA_OPENAI_CREDENTIAL_REF` to test refs such as `file:/run/secrets/openai-api-key` or `secret/openai`.

Stop the storage containers:

```bash
make dev-env-stop
```

## Runtime Configuration

KiwiGuard reads runtime configuration from environment variables. Required storage settings are `KIWIGUARD_POSTGRES_DSN` for configuration data and `KIWIGUARD_CLICKHOUSE_ADDR` for traffic and security event logs.

Timeouts use Go duration syntax:

| Variable | Default | Purpose |
| --- | --- | --- |
| `KIWIGUARD_UPSTREAM_TIMEOUT` | `30s` | Maximum time for an upstream model call. |
| `KIWIGUARD_VERDICT_TIMEOUT` | `5s` | Maximum time for each vertical security verdict call. |
| `KIWIGUARD_SERVER_READ_HEADER_TIMEOUT` | `5s` | Maximum time to read request headers. |
| `KIWIGUARD_SERVER_READ_TIMEOUT` | `15s` | Maximum time to read the full request. |
| `KIWIGUARD_SERVER_WRITE_TIMEOUT` | `60s` | Maximum time to write a response. |
| `KIWIGUARD_SERVER_IDLE_TIMEOUT` | `120s` | Maximum keep-alive idle time. |
| `KIWIGUARD_SHUTDOWN_TIMEOUT` | `10s` | Maximum graceful shutdown drain time. |

Provider records should store `credential_ref` values instead of raw upstream API keys. At runtime, KiwiGuard resolves `env:NAME` from the exact environment variable `NAME`, bare refs such as `secret/openai` from `KIWIGUARD_CREDENTIAL_SECRET_OPENAI`, and `file:/path/to/key` from a local secret file. Resolved secrets are kept in memory only and are not written back to PostgreSQL.

## Repository Standards

Contributors should read `CONTRIBUTING.md` before opening pull requests. Go code must follow `docs/go-standards.md`.

Security-sensitive reports belong in the disclosure process described in `SECURITY.md`, not in public issues. Do not include real secrets, prompts, responses, customer data, or private deployment details in issues, pull requests, or tests.

Release notes are tracked in `CHANGELOG.md`.

## License

KiwiGuard is licensed under the Apache License 2.0. See `LICENSE`.
