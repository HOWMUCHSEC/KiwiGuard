.PHONY: test test-go test-go-race test-go-cover coverage-check test-scripts bench-go fmt-go fmt-go-check vet-go lint-go vuln-go actionlint tidy-check check-web test-web build-go build-web build docker-image docker-image-smoke docker-config docker-production-config standards-check verify-go-coverage verify dev-env dev-env-infra dev-env-seed dev-env-stop dev-mock-llm dev-client-smoke dev-client-limits-smoke dev-clickhouse-outage-smoke beta-openai-smoke

GOLANGCI_LINT_VERSION := v2.6.2
GOVULNCHECK_VERSION := v1.3.0
ACTIONLINT_VERSION := v1.7.12
GOIMPORTS_VERSION := v0.38.0
GO_COVERAGE_MIN ?= 75.0
BIN_DIR ?= bin
KIWIGUARD_BIN ?= $(BIN_DIR)/kiwiguard
IMAGE_REPOSITORY ?= ghcr.io/howmuchsec/kiwiguard
IMAGE_TAG ?= dev
IMAGE ?= $(IMAGE_REPOSITORY):$(IMAGE_TAG)
DOCKER_BUILD_PLATFORM ?= linux/amd64
DOCKER_BUILD_ARGS ?=
PROD_ENV_FILE ?= .env.production.example
DEV_ENV_FILE ?= $(firstword $(wildcard .env) .env.example)
DEV_GATEWAY_ADDR ?= :18080
DEV_CONTROL_ADDR ?= :18081
DEV_MOCK_LLM_ADDR ?= 127.0.0.1:18082
DEV_CONSOLE_ADDR ?= 127.0.0.1
DEV_CONSOLE_PORT ?= 5173
DEV_POSTGRES_DSN ?= postgres://kiwiguard:kiwiguard@localhost:5432/kiwiguard?sslmode=disable
DEV_CLICKHOUSE_ADDR ?= localhost:9000
DEV_CLICKHOUSE_DATABASE ?= kiwiguard
DEV_CLICKHOUSE_USERNAME ?= kiwiguard
DEV_CLICKHOUSE_PASSWORD ?= kiwiguard
DEV_CLIENT_LIMITS_ROUTE_KEY ?= chat-completions
BETA_OPENAI_ROUTE_KEY ?= chat-completions
BETA_OPENAI_PROVIDER_KEY ?= dev-openai
BETA_OPENAI_CREDENTIAL_REF ?= $(if $(KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF),$(KIWIGUARD_BETA_OPENAI_CREDENTIAL_REF),env:KIWIGUARD_BETA_OPENAI_API_KEY)

test: test-go check-web

test-go:
	go test ./...

test-go-race:
	go test -race ./... -coverprofile=coverage.out -covermode=atomic

test-go-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

coverage-check:
	./scripts/check-go-coverage.sh coverage.out $(GO_COVERAGE_MIN)

test-scripts:
	./scripts/check-go-coverage_test.sh
	./scripts/dev-env-scripts_test.sh

bench-go:
	go test ./internal/domain/detection ./internal/domain/policy ./internal/contexts/gateway/adapters/http/openai -run '^$$' -bench . -benchtime=1x -benchmem

fmt-go:
	go run golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION) -w $$(git ls-files --cached --others --exclude-standard '*.go' | while IFS= read -r file; do test -f "$$file" && printf '%s\n' "$$file"; done)

fmt-go-check:
	@go_files="$$(git ls-files --cached --others --exclude-standard '*.go' | while IFS= read -r file; do test -f "$$file" && printf '%s\n' "$$file"; done)"; \
	files="$$(go run golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION) -l $$go_files)"; \
	test -z "$$files" || (echo "go files need goimports; run make fmt-go" >&2; echo "$$files"; exit 1)

vet-go:
	go vet ./...

lint-go:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

vuln-go:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

actionlint:
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

check-web:
	IBM_TELEMETRY_DISABLED=true pnpm -C web typecheck
	IBM_TELEMETRY_DISABLED=true pnpm -C web check:architecture
	IBM_TELEMETRY_DISABLED=true pnpm -C web check:comments

test-web: check-web

build-go:
	mkdir -p $(BIN_DIR)
	go build -o $(KIWIGUARD_BIN) ./cmd/kiwiguard

build-web:
	IBM_TELEMETRY_DISABLED=true pnpm -C web build

build: build-go build-web

docker-image:
	docker image build --platform $(DOCKER_BUILD_PLATFORM) --tag $(IMAGE) $(DOCKER_BUILD_ARGS) .

docker-image-smoke: docker-image
	docker run --rm --platform $(DOCKER_BUILD_PLATFORM) $(IMAGE) --help

docker-config:
	docker compose -f deployments/docker-compose.yml config

docker-production-config:
	docker compose -f deployments/production-compose.yml --env-file $(PROD_ENV_FILE) config

dev-env-infra:
	docker compose -f deployments/docker-compose.yml --env-file $(DEV_ENV_FILE) up -d --wait
	docker compose -f deployments/docker-compose.yml exec -T clickhouse clickhouse-client --user $(DEV_CLICKHOUSE_USERNAME) --password $(DEV_CLICKHOUSE_PASSWORD) --multiquery < internal/contexts/traffic/adapters/clickhouse/schema.sql

dev-env-seed:
	KIWIGUARD_POSTGRES_DSN="$(DEV_POSTGRES_DSN)" KIWIGUARD_CLICKHOUSE_ADDR=$(DEV_CLICKHOUSE_ADDR) go run ./cmd/kiwiguard migrate
	docker compose -f deployments/docker-compose.yml exec -T postgres psql "$(DEV_POSTGRES_DSN)" -v ON_ERROR_STOP=1 -v dev_mock_llm_url=http://$(DEV_MOCK_LLM_ADDR) -f - < scripts/dev-seed-config.sql

dev-mock-llm:
	./scripts/dev-mock-llm-api.sh --addr $(DEV_MOCK_LLM_ADDR)

dev-client-smoke:
	./scripts/dev-llm-client.sh --gateway-url http://127.0.0.1$(DEV_GATEWAY_ADDR) --control-url http://127.0.0.1$(DEV_CONTROL_ADDR) --clickhouse-addr $(DEV_CLICKHOUSE_ADDR) --clickhouse-database $(DEV_CLICKHOUSE_DATABASE) --clickhouse-user $(DEV_CLICKHOUSE_USERNAME) --clickhouse-password $(DEV_CLICKHOUSE_PASSWORD)

dev-client-limits-smoke:
	./scripts/dev-client-limits-smoke.sh --gateway-url http://127.0.0.1$(DEV_GATEWAY_ADDR) --control-url http://127.0.0.1$(DEV_CONTROL_ADDR) --route-key $(DEV_CLIENT_LIMITS_ROUTE_KEY)

dev-clickhouse-outage-smoke:
	./scripts/dev-clickhouse-outage-smoke.sh --gateway-url http://127.0.0.1$(DEV_GATEWAY_ADDR) --control-url http://127.0.0.1$(DEV_CONTROL_ADDR) --clickhouse-addr $(DEV_CLICKHOUSE_ADDR) --clickhouse-database $(DEV_CLICKHOUSE_DATABASE) --clickhouse-user $(DEV_CLICKHOUSE_USERNAME) --clickhouse-password $(DEV_CLICKHOUSE_PASSWORD)

beta-openai-smoke:
	./scripts/beta-openai-smoke.sh --gateway-url http://127.0.0.1$(DEV_GATEWAY_ADDR) --control-url http://127.0.0.1$(DEV_CONTROL_ADDR) --postgres-dsn "$(DEV_POSTGRES_DSN)" --route-key $(BETA_OPENAI_ROUTE_KEY) --provider-key $(BETA_OPENAI_PROVIDER_KEY) --credential-ref $(BETA_OPENAI_CREDENTIAL_REF)

dev-env: dev-env-infra dev-env-seed
	KIWIGUARD_HTTP_ADDR=$(DEV_GATEWAY_ADDR) KIWIGUARD_CONTROL_ADDR=$(DEV_CONTROL_ADDR) KIWIGUARD_CONTROL_INSECURE=true KIWIGUARD_POSTGRES_DSN="$(DEV_POSTGRES_DSN)" KIWIGUARD_CLICKHOUSE_ADDR=$(DEV_CLICKHOUSE_ADDR) KIWIGUARD_CLICKHOUSE_DATABASE=$(DEV_CLICKHOUSE_DATABASE) KIWIGUARD_CLICKHOUSE_USERNAME=$(DEV_CLICKHOUSE_USERNAME) KIWIGUARD_CLICKHOUSE_PASSWORD=$(DEV_CLICKHOUSE_PASSWORD) KIWIGUARD_EVENT_BATCH_SIZE=1 KIWIGUARD_UPSTREAM_TIMEOUT=5s KIWIGUARD_VERDICT_TIMEOUT=3s KIWIGUARD_LOG_LEVEL=debug ./scripts/dev-mock-llm-api.sh --addr $(DEV_MOCK_LLM_ADDR) & \
	mock_pid=$$!; \
	KIWIGUARD_HTTP_ADDR=$(DEV_GATEWAY_ADDR) KIWIGUARD_CONTROL_ADDR=$(DEV_CONTROL_ADDR) KIWIGUARD_CONTROL_INSECURE=true KIWIGUARD_POSTGRES_DSN="$(DEV_POSTGRES_DSN)" KIWIGUARD_CLICKHOUSE_ADDR=$(DEV_CLICKHOUSE_ADDR) KIWIGUARD_CLICKHOUSE_DATABASE=$(DEV_CLICKHOUSE_DATABASE) KIWIGUARD_CLICKHOUSE_USERNAME=$(DEV_CLICKHOUSE_USERNAME) KIWIGUARD_CLICKHOUSE_PASSWORD=$(DEV_CLICKHOUSE_PASSWORD) KIWIGUARD_EVENT_BATCH_SIZE=1 KIWIGUARD_UPSTREAM_TIMEOUT=5s KIWIGUARD_VERDICT_TIMEOUT=3s KIWIGUARD_LOG_LEVEL=debug go run ./cmd/kiwiguard serve & \
	app_pid=$$!; \
	KIWIGUARD_CONSOLE_API_TARGET=http://127.0.0.1$(DEV_CONTROL_ADDR) pnpm -C web dev --host $(DEV_CONSOLE_ADDR) --port $(DEV_CONSOLE_PORT) & \
	web_pid=$$!; \
	trap 'kill $$mock_pid $$app_pid $$web_pid 2>/dev/null || true; wait $$mock_pid $$app_pid $$web_pid 2>/dev/null || true' INT TERM EXIT; \
	echo "KiwiGuard gateway: http://127.0.0.1$(DEV_GATEWAY_ADDR)"; \
	echo "KiwiGuard control API: http://127.0.0.1$(DEV_CONTROL_ADDR)"; \
	echo "KiwiGuard console: http://$(DEV_CONSOLE_ADDR):$(DEV_CONSOLE_PORT)"; \
	echo "Mock LLM API: http://$(DEV_MOCK_LLM_ADDR)"; \
	wait

dev-env-stop:
	docker compose -f deployments/docker-compose.yml down

standards-check:
	./scripts/check-repository-standards.sh

verify-go-coverage:
	$(MAKE) test-go-race
	$(MAKE) coverage-check

verify: standards-check actionlint fmt-go-check tidy-check vet-go lint-go vuln-go test-scripts verify-go-coverage bench-go check-web build-web docker-config docker-production-config
