# Security Policy

## Supported Versions

KiwiGuard is currently in an early public foundation stage. Until the first stable release, security fixes are applied to the `main` branch.

| Version | Supported |
| ------- | --------- |
| main    | Yes       |
| < 1.0   | No stable release branch yet |

## Reporting a Vulnerability

Do not open public issues for vulnerabilities, exploitable behavior, secrets, prompt data, model responses, customer traffic, or private deployment details.

Report security-sensitive findings through the private disclosure channel configured by the project maintainers. If no private channel is available in the repository metadata, open a minimal public issue asking for a private security contact without including technical exploit details.

Please include:

- Affected version or commit.
- A concise description of the issue.
- Reproduction steps using synthetic data only.
- Potential impact.
- Suggested mitigation, if known.

## Sensitive Data Handling

KiwiGuard can process LLM prompts, model responses, detector findings, policy decisions, and traffic metadata. These records may contain sensitive information depending on deployment configuration.

When reporting bugs or contributing tests:

- Do not include real API keys, credentials, tokens, prompts, responses, or customer data.
- Use synthetic examples such as `alice@example.com`, `sk-test-example`, or fake payment-card test values.
- Redact hostnames, tenant names, account IDs, and internal URLs when they are not required for reproduction.

## Security Scope

KiwiGuard provides guardrail infrastructure for LLM traffic inspection and policy enforcement. It does not guarantee complete prevention of prompt injection, data leakage, unsafe model output, or model misuse. Deployers remain responsible for network security, authentication, authorization, logging retention, model selection, and operational controls.
