# Enterprise Deployment Guide

KiwiGuard is designed to be deployed as an OpenAI-compatible security gateway in front of enterprise LLM traffic.

## Recommended Topology

```text
Enterprise application
  -> KiwiGuard Gateway
  -> Upstream OpenAI-compatible LLM provider
```

Management and observability components:

```text
KiwiGuard Control API and Console
  -> PostgreSQL configuration store
  -> ClickHouse traffic and security event store
  -> Prometheus metrics
  -> OpenTelemetry traces
```

## Integration Pattern

Point application OpenAI-compatible clients at the KiwiGuard gateway instead of the upstream provider. Keep the application request shape unchanged where possible, then configure KiwiGuard to route traffic to the approved upstream model provider.

Typical enterprise flow:

1. Register upstream provider configuration.
2. Configure route mappings for model traffic.
3. Configure gateway clients and client limits.
4. Configure detector rules, including PII-style regular expressions.
5. Configure policy actions such as allow, monitor, or block.
6. Configure a vertical security verdict provider.
7. Enable traffic and security event capture according to retention policy.
8. Monitor activity and incidents in the Console and downstream observability systems.

## Configuration Store

PostgreSQL is the canonical configuration store. Enterprises should operate it as a managed, backed-up database with restricted network access.

Configuration data includes:

- routes and model mappings
- upstream provider metadata and credential references
- detector and policy configuration
- verdict provider configuration
- client limits
- observability and capture settings

Store credential references instead of raw API keys. Supported reference patterns include environment-backed refs and file-backed refs. Keep secret material in the enterprise secret manager or runtime environment.

## Traffic and Security Event Store

ClickHouse is the first-class event store for high-volume traffic and security logs.

Captured records can support:

- request and response audit workflows
- detector and rule hit analysis
- policy action review
- incident triage
- latency and upstream health analysis
- downstream SIEM or data platform integration

Retention settings should be aligned with company policy, privacy requirements, and regional data protection obligations.

## Verdict Provider

KiwiGuard can route requests and responses through a vertical security verdict provider. In the recommended baseline, verdict evaluation remains enabled even when local detector rules do not trigger, so the gateway can enforce specialized model-based safety checks.

Production verdict providers should have explicit timeout, fallback, and failure-mode settings. Security-critical deployments should prefer fail-closed behavior for unavailable verdict services unless the enterprise risk policy states otherwise.

## Console Usage

Use the Console to operate common workflows:

- review recent traffic events
- inspect mirrored request and response payloads where capture is enabled
- test regular expressions
- update policy and routing configuration
- review storage and retention settings
- inspect policy decisions, detector hits, and upstream status

Restrict Console and Control API access behind enterprise authentication, authorization, and network controls before production use.

## Observability

Gateway and control services expose Prometheus-compatible metrics at `/metrics`. Runtime boundaries can also emit OpenTelemetry spans when a tracer provider is installed by the embedding runtime.

Recommended production dashboards:

- gateway request rate, latency, and error rate
- upstream status and latency
- detector latency and hit rate
- verdict provider latency and failure rate
- blocked and monitored request rate
- event sink batch success, failure, spool depth, and overflow

## Production Readiness

Before production use, complete `docs/production-checklist.md`.
