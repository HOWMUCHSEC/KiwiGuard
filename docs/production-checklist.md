# Production Checklist

Use this checklist before deploying KiwiGuard into a production LLM traffic path.

## Runtime and Network

- Run Gateway, Control API, Console, worker, PostgreSQL, and ClickHouse in approved network segments.
- Restrict inbound Gateway access to approved applications.
- Restrict Control API and Console access to administrators.
- Terminate TLS at an approved load balancer, ingress, proxy, or service mesh.
- Set explicit request, upstream, verdict, and shutdown timeouts.

## Authentication and Authorization

- Put the Control API and Console behind enterprise authentication.
- Restrict administrative actions by role.
- Rotate gateway client credentials.
- Do not share upstream model provider keys with application teams.

## Secrets

- Store upstream credentials in an enterprise secret manager or runtime secret mount.
- Use `credential_ref` values in KiwiGuard configuration.
- Do not store raw provider API keys in PostgreSQL rows, Git, issue trackers, logs, or screenshots.

## Configuration Store

- Operate PostgreSQL with backups, monitoring, and restricted network access.
- Run migrations during controlled rollout windows.
- Validate configuration changes in a lower environment before production activation.
- Keep a rollback path for policy and routing revisions.

## Traffic and Security Logs

- Operate ClickHouse with capacity planning for expected LLM traffic volume.
- Define retention periods for mirrored request and response payloads.
- Disable raw capture where privacy policy does not allow payload storage.
- Verify event sink health and durable spool behavior during ClickHouse outages.
- Define downstream export patterns for SIEM or data platforms where required.

## Policies and Detectors

- Start with monitored rules before enforcing block actions on business-critical routes.
- Use synthetic data to validate PII-style regular expressions.
- Review false positives and false negatives before broad rollout.
- Version policy changes and document approval ownership.

## Verdict Provider

- Configure explicit verdict timeouts.
- Define fail-open or fail-closed behavior per route and risk class.
- Monitor verdict latency and error rate.
- Test verdict provider outage scenarios before production launch.

## Observability

- Scrape `/metrics` from Gateway and Control API services.
- Forward OpenTelemetry traces when a tracer provider is configured.
- Alert on gateway error rate, upstream failures, verdict failures, event sink failures, and spool overflow.
- Keep operational dashboards for traffic volume, policy decisions, latency, and storage health.

## Privacy and Compliance

- Classify whether prompts and responses may contain regulated data.
- Align raw capture and retention with privacy, legal, and contractual requirements.
- Use synthetic prompts and responses in tests, demos, screenshots, and public issues.
- Document incident-response ownership for blocked traffic and suspected data exposure.

## Release Readiness

- Run `make verify`.
- Run `make docker-config`.
- Run `make docker-production-config`.
- Build the backend binary with `make build-go`.
- Build the web console with `make build-web`.
- Smoke test a synthetic request through the gateway.
