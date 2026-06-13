package clickhouse

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestWriterPersistsTrafficEventToClickHouse(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "clickhouse/clickhouse-server:24.12",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"CLICKHOUSE_DB":                        "kiwiguard",
				"CLICKHOUSE_USER":                      "kiwiguard",
				"CLICKHOUSE_PASSWORD":                  "kiwiguard",
				"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT": "1",
			},
			WaitingFor: wait.ForListeningPort("9000/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("GenericContainer() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		if err := container.Terminate(cleanupCtx); err != nil {
			t.Errorf("container.Terminate() error = %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container.Host() error = %v", err)
	}
	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("container.MappedPort() error = %v", err)
	}

	conn, err := ch.Open(&ch.Options{
		Addr: []string{net.JoinHostPort(host, port.Port())},
		Auth: ch.Auth{
			Database: "kiwiguard",
			Username: "kiwiguard",
			Password: "kiwiguard",
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("clickhouse.Open() error = %v", err)
	}
	if err := waitForClickHouse(ctx, conn, 30*time.Second); err != nil {
		t.Fatalf("wait for ClickHouse: %v", err)
	}

	applyClickHouseSchema(t, ctx, conn)

	eventTime := time.Unix(1_780_000_000, 123_000_000).UTC()
	writer := NewWriter(conn)
	if err := writer.WriteBatch(ctx, []events.Event{{
		EventID:              "evt-clickhouse-1",
		SchemaVersion:        "v1",
		EventTime:            eventTime,
		RequestID:            "req-clickhouse-1",
		CorrelationID:        "corr-clickhouse-1",
		ConfigRevisionNumber: 42,
		SnapshotHash:         "snapshot-hash",
		RouteID:              "route-openai",
		ProviderID:           "provider-openai",
		VerdictProviderID:    "verdict-provider",
		PolicyBundleIDs:      []string{"builtin", "user-maintained"},
		HTTPMethod:           "POST",
		APIPath:              "/v1/chat/completions",
		EndpointKind:         "chat_completions",
		RequestedModel:       "gpt-requested",
		MappedModel:          "gpt-mapped",
		UpstreamModel:        "gpt-upstream",
		Direction:            events.Direction("output"),
		GatewayStatus:        200,
		UpstreamStatus:       201,
		Verdict:              "allow",
		Action:               events.Action("allow"),
		RequestHash:          "request-hash",
		ResponseHash:         "response-hash",
		RequestPayload:       `{"model":"gpt-requested"}`,
		ResponsePayload:      `{"output_text":"safe"}`,
		TotalLatency:         37 * time.Millisecond,
	}}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	var got struct {
		eventID          string
		requestedModel   string
		mappedModel      string
		upstreamModel    string
		gatewayStatus    uint16
		upstreamStatus   uint16
		verdictProvider  string
		action           string
		direction        string
		requestHash      string
		responseHash     string
		requestPayload   string
		responsePayload  string
		totalLatencyMS   uint32
		policyBundleIDs  []string
		configRevisionNo int64
	}
	err = conn.QueryRow(ctx, `
		SELECT
			event_id,
			requested_model,
			mapped_model,
			upstream_model,
			gateway_status,
			upstream_status,
			verdict_provider_id,
			action,
			direction,
			request_hash,
			response_hash,
			request_payload,
			response_payload,
			total_latency_ms,
			policy_bundle_ids,
			config_revision_number
		FROM kiwiguard_traffic_events
		WHERE event_id = ?
	`, "evt-clickhouse-1").Scan(
		&got.eventID,
		&got.requestedModel,
		&got.mappedModel,
		&got.upstreamModel,
		&got.gatewayStatus,
		&got.upstreamStatus,
		&got.verdictProvider,
		&got.action,
		&got.direction,
		&got.requestHash,
		&got.responseHash,
		&got.requestPayload,
		&got.responsePayload,
		&got.totalLatencyMS,
		&got.policyBundleIDs,
		&got.configRevisionNo,
	)
	if err != nil {
		t.Fatalf("query inserted event: %v", err)
	}

	if got.eventID != "evt-clickhouse-1" {
		t.Fatalf("event_id = %q, want evt-clickhouse-1", got.eventID)
	}
	if got.requestedModel != "gpt-requested" {
		t.Fatalf("requested_model = %q, want gpt-requested", got.requestedModel)
	}
	if got.mappedModel != "gpt-mapped" {
		t.Fatalf("mapped_model = %q, want gpt-mapped", got.mappedModel)
	}
	if got.upstreamModel != "gpt-upstream" {
		t.Fatalf("upstream_model = %q, want gpt-upstream", got.upstreamModel)
	}
	if got.gatewayStatus != 200 {
		t.Fatalf("gateway_status = %d, want 200", got.gatewayStatus)
	}
	if got.upstreamStatus != 201 {
		t.Fatalf("upstream_status = %d, want 201", got.upstreamStatus)
	}
	if got.verdictProvider != "verdict-provider" {
		t.Fatalf("verdict_provider_id = %q, want verdict-provider", got.verdictProvider)
	}
	if got.action != "allow" {
		t.Fatalf("action = %q, want allow", got.action)
	}
	if got.direction != "output" {
		t.Fatalf("direction = %q, want output", got.direction)
	}
	if got.requestHash != "request-hash" {
		t.Fatalf("request_hash = %q, want request-hash", got.requestHash)
	}
	if got.responseHash != "response-hash" {
		t.Fatalf("response_hash = %q, want response-hash", got.responseHash)
	}
	if got.requestPayload != `{"model":"gpt-requested"}` {
		t.Fatalf("request_payload = %q, want payload", got.requestPayload)
	}
	if got.responsePayload != `{"output_text":"safe"}` {
		t.Fatalf("response_payload = %q, want payload", got.responsePayload)
	}
	if got.totalLatencyMS != 37 {
		t.Fatalf("total_latency_ms = %d, want 37", got.totalLatencyMS)
	}
	if got.configRevisionNo != 42 {
		t.Fatalf("config_revision_number = %d, want 42", got.configRevisionNo)
	}
	if strings.Join(got.policyBundleIDs, ",") != "builtin,user-maintained" {
		t.Fatalf("policy_bundle_ids = %v, want [builtin user-maintained]", got.policyBundleIDs)
	}
}

func applyClickHouseSchema(t *testing.T, ctx context.Context, conn ch.Conn) {
	t.Helper()

	body, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatalf("ReadFile(schema.sql) error = %v", err)
	}

	for _, statement := range strings.Split(string(body), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := conn.Exec(ctx, statement); err != nil {
			t.Fatalf("apply schema statement %q: %v", summarizeSQL(statement), err)
		}
	}
}

func waitForClickHouse(ctx context.Context, conn ch.Conn, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := conn.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return fmt.Errorf("ping did not succeed within %s: %w", timeout, lastErr)
	}
	return fmt.Errorf("ping did not succeed within %s", timeout)
}

func summarizeSQL(statement string) string {
	fields := strings.Fields(statement)
	if len(fields) <= 6 {
		return statement
	}
	return fmt.Sprintf("%s ...", strings.Join(fields[:6], " "))
}
