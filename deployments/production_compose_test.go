package deployments

import (
	"os"
	"strings"
	"testing"
)

func TestProductionComposeDefinesPublicBetaRuntimeServices(t *testing.T) {
	body, err := os.ReadFile("production-compose.yml")
	if err != nil {
		t.Fatalf("ReadFile(production-compose.yml) error = %v", err)
	}

	compose := string(body)
	for _, want := range []string{
		"kiwiguard-gateway:",
		"kiwiguard-control:",
		"kiwiguard-worker:",
		"kiwiguard-migrate:",
		"postgres:",
		"clickhouse:",
		"caddy:",
		"${KIWIGUARD_IMAGE:-ghcr.io/howmuchsec/kiwiguard:dev}",
		"command: [\"gateway\"]",
		"command: [\"control\"]",
		"command: [\"worker\"]",
		"command: [\"migrate\"]",
		"depends_on:",
		"condition: service_healthy",
		"condition: service_completed_successfully",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("production-compose.yml missing %q:\n%s", want, compose)
		}
	}
}

func TestProductionComposePersistsStateAndProtectsControlPlane(t *testing.T) {
	body, err := os.ReadFile("production-compose.yml")
	if err != nil {
		t.Fatalf("ReadFile(production-compose.yml) error = %v", err)
	}

	compose := string(body)
	for _, want := range []string{
		"postgres-data:",
		"clickhouse-data:",
		"kiwiguard-spool:",
		"KIWIGUARD_EVENT_SPOOL_DIR: /var/lib/kiwiguard/event-spool",
		"KIWIGUARD_CONTROL_AUTH_TOKEN: ${KIWIGUARD_CONTROL_AUTH_TOKEN:?set KIWIGUARD_CONTROL_AUTH_TOKEN}",
		"KIWIGUARD_BETA_OPENAI_API_KEY: ${KIWIGUARD_BETA_OPENAI_API_KEY:-}",
		"KIWIGUARD_CREDENTIAL_SECRET_OPENAI: ${KIWIGUARD_CREDENTIAL_SECRET_OPENAI:-}",
		"KIWIGUARD_CONTROL_ADDR: :8081",
		"KIWIGUARD_HTTP_ADDR: :8080",
		"127.0.0.1:8081:8081",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("production-compose.yml missing %q:\n%s", want, compose)
		}
	}
}

func TestProductionComposeHasHealthChecks(t *testing.T) {
	body, err := os.ReadFile("production-compose.yml")
	if err != nil {
		t.Fatalf("ReadFile(production-compose.yml) error = %v", err)
	}

	compose := string(body)
	for _, want := range []string{
		"pg_isready -U kiwiguard -d kiwiguard",
		"clickhouse-client --user kiwiguard --password",
		"test: [\"CMD\", \"/kiwiguard\", \"version\"]",
		"test: [\"CMD\", \"caddy\", \"validate\", \"--config\", \"/etc/caddy/Caddyfile\"]",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("production-compose.yml missing health check %q:\n%s", want, compose)
		}
	}
}

func TestProductionCaddyfileTerminatesTLSAndKeepsControlPrivate(t *testing.T) {
	body, err := os.ReadFile("Caddyfile.production")
	if err != nil {
		t.Fatalf("ReadFile(Caddyfile.production) error = %v", err)
	}

	caddyfile := string(body)
	for _, want := range []string{
		"{$KIWIGUARD_PUBLIC_GATEWAY_HOST}",
		"reverse_proxy kiwiguard-gateway:8080",
		"{$KIWIGUARD_PUBLIC_CONTROL_HOST:localhost}",
		"reverse_proxy kiwiguard-control:8081",
		"respond / 404",
	} {
		if !strings.Contains(caddyfile, want) {
			t.Fatalf("Caddyfile.production missing %q:\n%s", want, caddyfile)
		}
	}
}

func TestProductionEnvExampleDocumentsRequiredRuntimeConfiguration(t *testing.T) {
	body, err := os.ReadFile("../.env.production.example")
	if err != nil {
		t.Fatalf("ReadFile(../.env.production.example) error = %v", err)
	}

	env := string(body)
	for _, want := range []string{
		"KIWIGUARD_IMAGE=ghcr.io/howmuchsec/kiwiguard:dev",
		"KIWIGUARD_PUBLIC_GATEWAY_HOST=gateway.example.com",
		"KIWIGUARD_CONTROL_AUTH_TOKEN=replace-with-long-random-token",
		"KIWIGUARD_POSTGRES_PASSWORD=replace-with-long-random-password",
		"KIWIGUARD_CLICKHOUSE_PASSWORD=replace-with-long-random-password",
		"KIWIGUARD_BETA_OPENAI_API_KEY=",
		"KIWIGUARD_CREDENTIAL_SECRET_OPENAI=",
		"KIWIGUARD_EVENT_SPOOL_MAX_BYTES=1073741824",
		"KIWIGUARD_SERVER_READ_HEADER_TIMEOUT=5s",
	} {
		if !strings.Contains(env, want) {
			t.Fatalf(".env.production.example missing %q:\n%s", want, env)
		}
	}
}
