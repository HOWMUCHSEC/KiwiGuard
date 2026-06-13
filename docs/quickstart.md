# Local Quickstart

This guide helps security and platform teams run KiwiGuard locally, send a sample LLM request through the gateway, and confirm that traffic and security events are captured.

## Prerequisites

- Go 1.25 or newer
- Docker with Compose support
- Node.js 22 or newer
- pnpm 10 or newer

## Start the Evaluation Stack

Copy the example environment file:

```bash
cp .env.example .env
```

Install web dependencies:

```bash
pnpm -C web install
```

Start PostgreSQL, ClickHouse, the mock LLM API, KiwiGuard services, and the web console:

```bash
make dev-env
```

Default local endpoints:

| Service | URL |
| --- | --- |
| Gateway | `http://127.0.0.1:18080` |
| Control API | `http://127.0.0.1:18081` |
| Mock LLM API | `http://127.0.0.1:18082` |
| Console | `http://127.0.0.1:5173` |

## Send a Sample Request

In another terminal, send a synthetic client request through KiwiGuard:

```bash
make dev-client-smoke
```

The smoke script sends OpenAI-compatible traffic through the gateway and checks that ClickHouse receives structured event metadata.

## Inspect Results

Open the Console and review:

- traffic events
- request and response mirror fields
- detector hits
- policy actions
- gateway and upstream status
- latency fields
- storage and retention posture

Metrics are available from the runtime services:

```bash
curl http://127.0.0.1:18080/metrics
curl http://127.0.0.1:18081/metrics
```

## Stop the Stack

Stop local storage services:

```bash
make dev-env-stop
```

## Use a Real OpenAI-Compatible Provider

For an integration smoke against a real upstream provider, export credentials only into your local shell and run:

```bash
KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make dev-env
```

Then run:

```bash
KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make beta-openai-smoke
```

Use synthetic prompts only. Do not use customer data in local smoke tests.
