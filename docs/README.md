# KiwiGuard Documentation

KiwiGuard is an OpenAI-compatible content-security gateway for enterprise LLM traffic. It sits between business applications and upstream model providers, evaluates inputs and outputs, mirrors traffic for audit workflows, and records structured security events for operations teams.

For the project overview and local development commands, see the root `README.md`.

## Languages

- English: `docs/README.md`
- Simplified Chinese: `docs/README.zh-CN.md`

## Enterprise Adoption Path

1. Run the local evaluation stack with `docs/quickstart.md`.
2. Review the enterprise gateway topology in `docs/enterprise-deployment.md`.
3. Prepare production controls with `docs/production-checklist.md`.
4. Review contributor standards in `docs/go-standards.md` before changing backend code.

## Documentation Index

| Topic | English | Simplified Chinese |
| --- | --- | --- |
| Local quickstart | `docs/quickstart.md` | `docs/quickstart.zh-CN.md` |
| Enterprise deployment | `docs/enterprise-deployment.md` | `docs/enterprise-deployment.zh-CN.md` |
| Production checklist | `docs/production-checklist.md` | `docs/production-checklist.zh-CN.md` |
| Go engineering standards | `docs/go-standards.md` | `docs/go-standards.zh-CN.md` |

## Enterprise Deployment Model

The default deployment model is a security gateway in the LLM call path:

```text
Enterprise application
  -> KiwiGuard Gateway
  -> Upstream OpenAI-compatible LLM provider
```

Operational services run beside the gateway:

```text
KiwiGuard Control API and Console
  -> PostgreSQL configuration store
  -> ClickHouse traffic and security event store
  -> Prometheus metrics and OpenTelemetry traces
```

## Core Enterprise Workflows

- Route OpenAI-compatible chat and response traffic through KiwiGuard.
- Inspect prompts and model outputs with configured rules and PII-style detectors.
- Send requests and responses through a vertical security verdict provider.
- Mirror request and response payloads for audit workflows when enabled by policy.
- Store structured traffic and security events in ClickHouse.
- Use the Console to review traffic, detector matches, policy actions, gateway status, upstream status, latency, and retention posture.

## Security Note

Do not place real secrets, production prompts, customer responses, or private deployment details in issues, pull requests, tests, screenshots, or public documentation. Follow the disclosure process in `SECURITY.md` for sensitive reports.
