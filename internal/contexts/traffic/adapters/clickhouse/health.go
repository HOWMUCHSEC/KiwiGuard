package clickhouse

import (
	"context"
	"fmt"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const trafficEventsTable = "kiwiguard_traffic_events"

var requiredTrafficEventColumns = map[string]string{
	"event_id":               "String",
	"event_schema_version":   "LowCardinality(String)",
	"event_time":             "DateTime64(3, 'UTC')",
	"request_id":             "String",
	"correlation_id":         "String",
	"config_revision_number": "Int64",
	"snapshot_hash":          "String",
	"route_id":               "String",
	"provider_id":            "String",
	"verdict_provider_id":    "String",
	"policy_bundle_ids":      "Array(String)",
	"http_method":            "LowCardinality(String)",
	"api_path":               "String",
	"endpoint_kind":          "LowCardinality(String)",
	"requested_model":        "String",
	"mapped_model":           "String",
	"upstream_model":         "String",
	"gateway_status":         "UInt16",
	"upstream_status":        "UInt16",
	"error_type":             "LowCardinality(String)",
	"block_reason":           "String",
	"fallback_triggered":     "Bool",
	"risk_level":             "LowCardinality(String)",
	"verdict_categories":     "Array(String)",
	"confidence":             "Float64",
	"detector_categories":    "Array(String)",
	"matched_span_count":     "UInt32",
	"streaming_chunk_count":  "UInt32",
	"redaction_count":        "UInt32",
	"termination_reason":     "LowCardinality(String)",
	"gateway_latency_ms":     "UInt32",
	"detector_latency_ms":    "UInt32",
	"verdict_latency_ms":     "UInt32",
	"upstream_latency_ms":    "UInt32",
	"queue_delay_ms":         "UInt32",
	"total_latency_ms":       "UInt32",
	"sink_status":            "LowCardinality(String)",
	"retry_count":            "UInt16",
	"spool_status":           "LowCardinality(String)",
	"dropped":                "Bool",
	"drop_reason":            "LowCardinality(String)",
	"raw_capture_policy_id":  "String",
	"retention_policy_id":    "String",
	"capture_reference":      "String",
	"route":                  "String",
	"provider":               "String",
	"direction":              "LowCardinality(String)",
	"verdict":                "LowCardinality(String)",
	"action":                 "LowCardinality(String)",
	"detector_hits":          "Array(String)",
	"policy_rule_ids":        "Array(String)",
	"latency_ms":             "UInt32",
	"streaming_terminated":   "Bool",
	"raw_capture_enabled":    "Bool",
	"request_hash":           "String",
	"response_hash":          "String",
	"request_payload":        "String",
	"response_payload":       "String",
	"metadata_json":          "String",
}

// Options defines the connection settings for the native ClickHouse client.
type Options struct {
	Addr     string
	Database string
	Username string
	Password string
}

// Probe is the minimal ClickHouse API needed for readiness checks.
type Probe interface {
	Ping(context.Context) error
	QueryRow(context.Context, string, ...any) driver.Row
}

// Open initializes the native ClickHouse client from runtime connection options.
func Open(options Options) (ch.Conn, error) {
	if options.Addr == "" {
		return nil, fmt.Errorf("clickhouse addr is required")
	}
	if options.Database == "" {
		options.Database = "kiwiguard"
	}
	if options.Username == "" {
		options.Username = "default"
	}

	conn, err := ch.Open(&ch.Options{
		Addr: []string{options.Addr},
		Auth: ch.Auth{
			Database: options.Database,
			Username: options.Username,
			Password: options.Password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}
	return conn, nil
}

// ProbeHealth verifies ClickHouse is reachable and the traffic events schema is ready.
func ProbeHealth(ctx context.Context, conn Probe) error {
	if conn == nil {
		return fmt.Errorf("clickhouse probe is required")
	}
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping clickhouse: %w", err)
	}
	if err := CheckSchema(ctx, conn); err != nil {
		return fmt.Errorf("check clickhouse schema: %w", err)
	}
	return nil
}

// CheckSchema verifies the active database has the traffic events table and insert columns.
func CheckSchema(ctx context.Context, conn Probe) error {
	if conn == nil {
		return fmt.Errorf("clickhouse probe is required")
	}

	var tableCount uint64
	if err := conn.QueryRow(ctx, `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`, trafficEventsTable).Scan(&tableCount); err != nil {
		return fmt.Errorf("query clickhouse traffic event table: %w", err)
	}
	if tableCount == 0 {
		return fmt.Errorf("%s table is required", trafficEventsTable)
	}

	var columnCount uint64
	if err := conn.QueryRow(ctx, `
		SELECT count()
		FROM system.columns
		WHERE database = currentDatabase()
			AND table = ?
			AND concat(name, ':', type) IN (?)
	`, trafficEventsTable, requiredColumnKeys()).Scan(&columnCount); err != nil {
		return fmt.Errorf("query clickhouse traffic event columns: %w", err)
	}
	if columnCount != uint64(len(requiredTrafficEventColumns)) {
		return fmt.Errorf("%s columns are invalid: got %d required columns, want %d", trafficEventsTable, columnCount, len(requiredTrafficEventColumns))
	}
	return nil
}

func requiredColumnKeys() []string {
	columns := make([]string, 0, len(requiredTrafficEventColumns))
	for name, columnType := range requiredTrafficEventColumns {
		columns = append(columns, name+":"+columnType)
	}
	return columns
}
