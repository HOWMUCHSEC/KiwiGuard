package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("KIWIGUARD_HTTP_ADDR", "")
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://kiwiguard:kiwiguard@localhost:5432/kiwiguard?sslmode=disable")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "localhost:9000")
	t.Setenv("KIWIGUARD_CLICKHOUSE_DATABASE", "")
	t.Setenv("KIWIGUARD_CLICKHOUSE_USERNAME", "")
	t.Setenv("KIWIGUARD_CLICKHOUSE_PASSWORD", "")
	t.Setenv("KIWIGUARD_EVENT_SINK_TYPE", "")
	t.Setenv("KIWIGUARD_EVENT_QUEUE_CAPACITY", "")
	t.Setenv("KIWIGUARD_EVENT_BATCH_SIZE", "")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_DIR", "")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_MAX_BYTES", "")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_MAX_AGE", "")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_REPLAY_INTERVAL", "")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_BATCH_SIZE", "")
	t.Setenv("KIWIGUARD_LOG_LEVEL", "")
	t.Setenv("KIWIGUARD_UPSTREAM_TIMEOUT", "")
	t.Setenv("KIWIGUARD_VERDICT_TIMEOUT", "")
	t.Setenv("KIWIGUARD_SERVER_READ_HEADER_TIMEOUT", "")
	t.Setenv("KIWIGUARD_SERVER_READ_TIMEOUT", "")
	t.Setenv("KIWIGUARD_SERVER_WRITE_TIMEOUT", "")
	t.Setenv("KIWIGUARD_SERVER_IDLE_TIMEOUT", "")
	t.Setenv("KIWIGUARD_SHUTDOWN_TIMEOUT", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.ControlAddr != "127.0.0.1:8081" {
		t.Fatalf("ControlAddr = %q, want loopback default", cfg.ControlAddr)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.EventSinkType != "clickhouse" {
		t.Fatalf("EventSinkType = %q, want clickhouse", cfg.EventSinkType)
	}
	if cfg.ClickHouseDatabase != "kiwiguard" {
		t.Fatalf("ClickHouseDatabase = %q, want kiwiguard", cfg.ClickHouseDatabase)
	}
	if cfg.ClickHouseUsername != "default" {
		t.Fatalf("ClickHouseUsername = %q, want default", cfg.ClickHouseUsername)
	}
	if cfg.EventQueueCapacity != 1024 {
		t.Fatalf("EventQueueCapacity = %d, want 1024", cfg.EventQueueCapacity)
	}
	if cfg.EventBatchSize != 100 {
		t.Fatalf("EventBatchSize = %d, want 100", cfg.EventBatchSize)
	}
	if cfg.EventSpoolDir != "./data/event-spool" {
		t.Fatalf("EventSpoolDir = %q, want ./data/event-spool", cfg.EventSpoolDir)
	}
	if cfg.EventSpoolMaxBytes != 1073741824 {
		t.Fatalf("EventSpoolMaxBytes = %d, want 1073741824", cfg.EventSpoolMaxBytes)
	}
	if cfg.EventSpoolMaxAge != 24*time.Hour {
		t.Fatalf("EventSpoolMaxAge = %v, want 24h", cfg.EventSpoolMaxAge)
	}
	if cfg.EventSpoolReplayInterval != 5*time.Second {
		t.Fatalf("EventSpoolReplayInterval = %v, want 5s", cfg.EventSpoolReplayInterval)
	}
	if cfg.EventSpoolBatchSize != 100 {
		t.Fatalf("EventSpoolBatchSize = %d, want 100", cfg.EventSpoolBatchSize)
	}
	if cfg.UpstreamTimeout != 30*time.Second {
		t.Fatalf("UpstreamTimeout = %v, want 30s", cfg.UpstreamTimeout)
	}
	if cfg.VerdictTimeout != 5*time.Second {
		t.Fatalf("VerdictTimeout = %v, want 5s", cfg.VerdictTimeout)
	}
	if cfg.ServerReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ServerReadHeaderTimeout = %v, want 5s", cfg.ServerReadHeaderTimeout)
	}
	if cfg.ServerReadTimeout != 15*time.Second {
		t.Fatalf("ServerReadTimeout = %v, want 15s", cfg.ServerReadTimeout)
	}
	if cfg.ServerWriteTimeout != time.Minute {
		t.Fatalf("ServerWriteTimeout = %v, want 1m0s", cfg.ServerWriteTimeout)
	}
	if cfg.ServerIdleTimeout != 2*time.Minute {
		t.Fatalf("ServerIdleTimeout = %v, want 2m0s", cfg.ServerIdleTimeout)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want 10s", cfg.ShutdownTimeout)
	}
}

func TestLoadFromEnvRejectsPublicControlAddressWithoutAuth(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_CONTROL_ADDR", ":8081")
	t.Setenv("KIWIGUARD_CONTROL_AUTH_TOKEN", "")
	t.Setenv("KIWIGUARD_CONTROL_INSECURE", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want public control auth error")
	}
	if !strings.Contains(err.Error(), "KIWIGUARD_CONTROL_AUTH_TOKEN") {
		t.Fatalf("LoadFromEnv() error = %q, want auth token guidance", err.Error())
	}
}

func TestLoadFromEnvAllowsPublicControlAddressWithAuthToken(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_CONTROL_ADDR", ":8081")
	t.Setenv("KIWIGUARD_CONTROL_AUTH_TOKEN", "control-secret")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.ControlAuthToken != "control-secret" {
		t.Fatalf("ControlAuthToken = %q, want configured token", cfg.ControlAuthToken)
	}
}

func TestLoadFromEnvLoadsEventSinkSettings(t *testing.T) {
	t.Setenv("KIWIGUARD_HTTP_ADDR", "")
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://kiwiguard:kiwiguard@localhost:5432/kiwiguard?sslmode=disable")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_CLICKHOUSE_DATABASE", "events")
	t.Setenv("KIWIGUARD_CLICKHOUSE_USERNAME", "kiwi")
	t.Setenv("KIWIGUARD_CLICKHOUSE_PASSWORD", "secret")
	t.Setenv("KIWIGUARD_EVENT_SINK_TYPE", "memory")
	t.Setenv("KIWIGUARD_EVENT_QUEUE_CAPACITY", "2048")
	t.Setenv("KIWIGUARD_EVENT_BATCH_SIZE", "250")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_DIR", "/var/lib/kiwiguard/spool")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_MAX_BYTES", "4096")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_MAX_AGE", "2h")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_REPLAY_INTERVAL", "750ms")
	t.Setenv("KIWIGUARD_EVENT_SPOOL_BATCH_SIZE", "25")
	t.Setenv("KIWIGUARD_LOG_LEVEL", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.EventSinkType != "memory" {
		t.Fatalf("EventSinkType = %q, want memory", cfg.EventSinkType)
	}
	if cfg.ClickHouseAddr != "clickhouse:9000" {
		t.Fatalf("ClickHouseAddr = %q, want clickhouse:9000", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseDatabase != "events" {
		t.Fatalf("ClickHouseDatabase = %q, want events", cfg.ClickHouseDatabase)
	}
	if cfg.ClickHouseUsername != "kiwi" {
		t.Fatalf("ClickHouseUsername = %q, want kiwi", cfg.ClickHouseUsername)
	}
	if cfg.ClickHousePassword != "secret" {
		t.Fatalf("ClickHousePassword = %q, want secret", cfg.ClickHousePassword)
	}
	if cfg.EventQueueCapacity != 2048 {
		t.Fatalf("EventQueueCapacity = %d, want 2048", cfg.EventQueueCapacity)
	}
	if cfg.EventBatchSize != 250 {
		t.Fatalf("EventBatchSize = %d, want 250", cfg.EventBatchSize)
	}
	if cfg.EventSpoolDir != "/var/lib/kiwiguard/spool" {
		t.Fatalf("EventSpoolDir = %q, want /var/lib/kiwiguard/spool", cfg.EventSpoolDir)
	}
	if cfg.EventSpoolMaxBytes != 4096 {
		t.Fatalf("EventSpoolMaxBytes = %d, want 4096", cfg.EventSpoolMaxBytes)
	}
	if cfg.EventSpoolMaxAge != 2*time.Hour {
		t.Fatalf("EventSpoolMaxAge = %v, want 2h", cfg.EventSpoolMaxAge)
	}
	if cfg.EventSpoolReplayInterval != 750*time.Millisecond {
		t.Fatalf("EventSpoolReplayInterval = %v, want 750ms", cfg.EventSpoolReplayInterval)
	}
	if cfg.EventSpoolBatchSize != 25 {
		t.Fatalf("EventSpoolBatchSize = %d, want 25", cfg.EventSpoolBatchSize)
	}
}

func TestLoadFromEnvLoadsGatewaySettings(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_HTTP_ADDR", ":18080")
	t.Setenv("KIWIGUARD_CONTROL_ADDR", ":18081")
	t.Setenv("KIWIGUARD_CONTROL_AUTH_TOKEN", "control-secret")
	t.Setenv("KIWIGUARD_UPSTREAM_BASE_URL", "http://upstream.test")
	t.Setenv("KIWIGUARD_UPSTREAM_API_KEY", "upstream-secret")
	t.Setenv("KIWIGUARD_VERDICT_ENDPOINT", "http://verdict.test/evaluate")
	t.Setenv("KIWIGUARD_MAX_BODY_BYTES", "2097152")
	t.Setenv("KIWIGUARD_POLICY_SNAPSHOT_PATH", "/tmp/policy.json")
	t.Setenv("KIWIGUARD_UPSTREAM_TIMEOUT", "750ms")
	t.Setenv("KIWIGUARD_VERDICT_TIMEOUT", "1500ms")
	t.Setenv("KIWIGUARD_SERVER_READ_HEADER_TIMEOUT", "2s")
	t.Setenv("KIWIGUARD_SERVER_READ_TIMEOUT", "3s")
	t.Setenv("KIWIGUARD_SERVER_WRITE_TIMEOUT", "4s")
	t.Setenv("KIWIGUARD_SERVER_IDLE_TIMEOUT", "5s")
	t.Setenv("KIWIGUARD_SHUTDOWN_TIMEOUT", "6s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.GatewayAddr != ":18080" {
		t.Fatalf("GatewayAddr = %q, want :18080", cfg.GatewayAddr)
	}
	if cfg.HTTPAddr != ":18080" {
		t.Fatalf("HTTPAddr = %q, want :18080", cfg.HTTPAddr)
	}
	if cfg.ControlAddr != ":18081" {
		t.Fatalf("ControlAddr = %q, want :18081", cfg.ControlAddr)
	}
	if cfg.UpstreamBaseURL != "http://upstream.test" {
		t.Fatalf("UpstreamBaseURL = %q, want http://upstream.test", cfg.UpstreamBaseURL)
	}
	if cfg.UpstreamAPIKey != "upstream-secret" {
		t.Fatalf("UpstreamAPIKey = %q, want upstream-secret", cfg.UpstreamAPIKey)
	}
	if cfg.VerdictEndpoint != "http://verdict.test/evaluate" {
		t.Fatalf("VerdictEndpoint = %q, want http://verdict.test/evaluate", cfg.VerdictEndpoint)
	}
	if cfg.MaxBodyBytes != 2097152 {
		t.Fatalf("MaxBodyBytes = %d, want 2097152", cfg.MaxBodyBytes)
	}
	if cfg.PolicySnapshotPath != "/tmp/policy.json" {
		t.Fatalf("PolicySnapshotPath = %q, want /tmp/policy.json", cfg.PolicySnapshotPath)
	}
	if cfg.UpstreamTimeout != 750*time.Millisecond {
		t.Fatalf("UpstreamTimeout = %v, want 750ms", cfg.UpstreamTimeout)
	}
	if cfg.VerdictTimeout != 1500*time.Millisecond {
		t.Fatalf("VerdictTimeout = %v, want 1.5s", cfg.VerdictTimeout)
	}
	if cfg.ServerReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ServerReadHeaderTimeout = %v, want 2s", cfg.ServerReadHeaderTimeout)
	}
	if cfg.ServerReadTimeout != 3*time.Second {
		t.Fatalf("ServerReadTimeout = %v, want 3s", cfg.ServerReadTimeout)
	}
	if cfg.ServerWriteTimeout != 4*time.Second {
		t.Fatalf("ServerWriteTimeout = %v, want 4s", cfg.ServerWriteTimeout)
	}
	if cfg.ServerIdleTimeout != 5*time.Second {
		t.Fatalf("ServerIdleTimeout = %v, want 5s", cfg.ServerIdleTimeout)
	}
	if cfg.ShutdownTimeout != 6*time.Second {
		t.Fatalf("ShutdownTimeout = %v, want 6s", cfg.ShutdownTimeout)
	}
}

func TestLoadFromEnvRejectsInvalidDuration(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_UPSTREAM_TIMEOUT", "definitely-not-a-duration")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid duration error")
	}
	if !strings.Contains(err.Error(), "KIWIGUARD_UPSTREAM_TIMEOUT") {
		t.Fatalf("LoadFromEnv() error = %q, want env var name", err.Error())
	}
}

func TestLoadFromEnvRejectsInvalidNumericSettings(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "max body bytes", key: "KIWIGUARD_MAX_BODY_BYTES"},
		{name: "event queue capacity", key: "KIWIGUARD_EVENT_QUEUE_CAPACITY"},
		{name: "event batch size", key: "KIWIGUARD_EVENT_BATCH_SIZE"},
		{name: "event spool max bytes", key: "KIWIGUARD_EVENT_SPOOL_MAX_BYTES"},
		{name: "event spool batch size", key: "KIWIGUARD_EVENT_SPOOL_BATCH_SIZE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
			t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
			t.Setenv(tt.key, "not-a-number")

			_, err := LoadFromEnv()
			if err == nil {
				t.Fatalf("LoadFromEnv() error = nil, want invalid %s error", tt.key)
			}
		})
	}
}

func TestLoadFromEnvRejectsInvalidOperationalDurations(t *testing.T) {
	tests := []string{
		"KIWIGUARD_EVENT_SPOOL_MAX_AGE",
		"KIWIGUARD_EVENT_SPOOL_REPLAY_INTERVAL",
		"KIWIGUARD_VERDICT_TIMEOUT",
		"KIWIGUARD_SERVER_READ_HEADER_TIMEOUT",
		"KIWIGUARD_SERVER_READ_TIMEOUT",
		"KIWIGUARD_SERVER_WRITE_TIMEOUT",
		"KIWIGUARD_SERVER_IDLE_TIMEOUT",
		"KIWIGUARD_SHUTDOWN_TIMEOUT",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
			t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
			t.Setenv(key, "not-a-duration")

			_, err := LoadFromEnv()
			if err == nil {
				t.Fatalf("LoadFromEnv() error = nil, want invalid %s error", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("LoadFromEnv() error = %q, want env var name %s", err.Error(), key)
			}
		})
	}
}

func TestLoadFromEnvRejectsInvalidControlInsecureBool(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_CONTROL_INSECURE", "sometimes")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want invalid bool error")
	}
	if !strings.Contains(err.Error(), "KIWIGUARD_CONTROL_INSECURE") {
		t.Fatalf("LoadFromEnv() error = %q, want env var name", err.Error())
	}
}

func TestLoadFromEnvAllowsPublicControlAddressWhenExplicitlyInsecure(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://example")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "clickhouse:9000")
	t.Setenv("KIWIGUARD_CONTROL_ADDR", "0.0.0.0:8081")
	t.Setenv("KIWIGUARD_CONTROL_AUTH_TOKEN", "")
	t.Setenv("KIWIGUARD_CONTROL_INSECURE", "true")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if !cfg.ControlInsecure {
		t.Fatal("ControlInsecure = false, want true")
	}
}

func TestConfigGroupedViews(t *testing.T) {
	cfg := Config{
		PostgresDSN:              "postgres://example",
		GatewayAddr:              ":18080",
		ControlAddr:              "127.0.0.1:18081",
		ControlAuthToken:         "control-secret",
		ControlInsecure:          true,
		UpstreamBaseURL:          "http://upstream.test",
		UpstreamAPIKey:           "upstream-secret",
		VerdictEndpoint:          "http://verdict.test/evaluate",
		MaxBodyBytes:             2097152,
		PolicySnapshotPath:       "/tmp/policy.json",
		ClickHouseAddr:           "clickhouse:9000",
		ClickHouseDatabase:       "events",
		ClickHouseUsername:       "kiwi",
		ClickHousePassword:       "secret",
		EventSinkType:            "clickhouse",
		EventQueueCapacity:       2048,
		EventBatchSize:           250,
		EventSpoolDir:            "/var/lib/kiwiguard/spool",
		EventSpoolMaxBytes:       4096,
		EventSpoolMaxAge:         2 * time.Hour,
		EventSpoolReplayInterval: 750 * time.Millisecond,
		EventSpoolBatchSize:      25,
		UpstreamTimeout:          750 * time.Millisecond,
		VerdictTimeout:           1500 * time.Millisecond,
		ServerReadHeaderTimeout:  2 * time.Second,
		ServerReadTimeout:        3 * time.Second,
		ServerWriteTimeout:       4 * time.Second,
		ServerIdleTimeout:        5 * time.Second,
		ShutdownTimeout:          6 * time.Second,
	}

	httpCfg := cfg.HTTPServer()
	if httpCfg.ReadHeaderTimeout != cfg.ServerReadHeaderTimeout {
		t.Fatalf("HTTPServer().ReadHeaderTimeout = %v, want %v", httpCfg.ReadHeaderTimeout, cfg.ServerReadHeaderTimeout)
	}
	if httpCfg.ReadTimeout != cfg.ServerReadTimeout {
		t.Fatalf("HTTPServer().ReadTimeout = %v, want %v", httpCfg.ReadTimeout, cfg.ServerReadTimeout)
	}
	if httpCfg.WriteTimeout != cfg.ServerWriteTimeout {
		t.Fatalf("HTTPServer().WriteTimeout = %v, want %v", httpCfg.WriteTimeout, cfg.ServerWriteTimeout)
	}
	if httpCfg.IdleTimeout != cfg.ServerIdleTimeout {
		t.Fatalf("HTTPServer().IdleTimeout = %v, want %v", httpCfg.IdleTimeout, cfg.ServerIdleTimeout)
	}
	if httpCfg.ShutdownTimeout != cfg.ShutdownTimeout {
		t.Fatalf("HTTPServer().ShutdownTimeout = %v, want %v", httpCfg.ShutdownTimeout, cfg.ShutdownTimeout)
	}

	storageCfg := cfg.Storage()
	if storageCfg.PostgresDSN != cfg.PostgresDSN {
		t.Fatalf("Storage().PostgresDSN = %q, want %q", storageCfg.PostgresDSN, cfg.PostgresDSN)
	}
	if storageCfg.ClickHouseAddr != cfg.ClickHouseAddr {
		t.Fatalf("Storage().ClickHouseAddr = %q, want %q", storageCfg.ClickHouseAddr, cfg.ClickHouseAddr)
	}
	if storageCfg.ClickHouseDatabase != cfg.ClickHouseDatabase {
		t.Fatalf("Storage().ClickHouseDatabase = %q, want %q", storageCfg.ClickHouseDatabase, cfg.ClickHouseDatabase)
	}
	if storageCfg.ClickHouseUsername != cfg.ClickHouseUsername {
		t.Fatalf("Storage().ClickHouseUsername = %q, want %q", storageCfg.ClickHouseUsername, cfg.ClickHouseUsername)
	}
	if storageCfg.ClickHousePassword != cfg.ClickHousePassword {
		t.Fatalf("Storage().ClickHousePassword = %q, want %q", storageCfg.ClickHousePassword, cfg.ClickHousePassword)
	}

	eventsCfg := cfg.Events()
	if eventsCfg.SinkType != cfg.EventSinkType {
		t.Fatalf("Events().SinkType = %q, want %q", eventsCfg.SinkType, cfg.EventSinkType)
	}
	if eventsCfg.QueueCapacity != cfg.EventQueueCapacity {
		t.Fatalf("Events().QueueCapacity = %d, want %d", eventsCfg.QueueCapacity, cfg.EventQueueCapacity)
	}
	if eventsCfg.BatchSize != cfg.EventBatchSize {
		t.Fatalf("Events().BatchSize = %d, want %d", eventsCfg.BatchSize, cfg.EventBatchSize)
	}
	if eventsCfg.SpoolDir != cfg.EventSpoolDir {
		t.Fatalf("Events().SpoolDir = %q, want %q", eventsCfg.SpoolDir, cfg.EventSpoolDir)
	}
	if eventsCfg.SpoolMaxBytes != cfg.EventSpoolMaxBytes {
		t.Fatalf("Events().SpoolMaxBytes = %d, want %d", eventsCfg.SpoolMaxBytes, cfg.EventSpoolMaxBytes)
	}
	if eventsCfg.SpoolMaxAge != cfg.EventSpoolMaxAge {
		t.Fatalf("Events().SpoolMaxAge = %v, want %v", eventsCfg.SpoolMaxAge, cfg.EventSpoolMaxAge)
	}
	if eventsCfg.SpoolReplayInterval != cfg.EventSpoolReplayInterval {
		t.Fatalf("Events().SpoolReplayInterval = %v, want %v", eventsCfg.SpoolReplayInterval, cfg.EventSpoolReplayInterval)
	}
	if eventsCfg.SpoolBatchSize != cfg.EventSpoolBatchSize {
		t.Fatalf("Events().SpoolBatchSize = %d, want %d", eventsCfg.SpoolBatchSize, cfg.EventSpoolBatchSize)
	}

	gatewayCfg := cfg.Gateway()
	if gatewayCfg.Addr != cfg.GatewayAddr {
		t.Fatalf("Gateway().Addr = %q, want %q", gatewayCfg.Addr, cfg.GatewayAddr)
	}
	if gatewayCfg.UpstreamBaseURL != cfg.UpstreamBaseURL {
		t.Fatalf("Gateway().UpstreamBaseURL = %q, want %q", gatewayCfg.UpstreamBaseURL, cfg.UpstreamBaseURL)
	}
	if gatewayCfg.UpstreamAPIKey != cfg.UpstreamAPIKey {
		t.Fatalf("Gateway().UpstreamAPIKey = %q, want %q", gatewayCfg.UpstreamAPIKey, cfg.UpstreamAPIKey)
	}
	if gatewayCfg.VerdictEndpoint != cfg.VerdictEndpoint {
		t.Fatalf("Gateway().VerdictEndpoint = %q, want %q", gatewayCfg.VerdictEndpoint, cfg.VerdictEndpoint)
	}
	if gatewayCfg.MaxBodyBytes != cfg.MaxBodyBytes {
		t.Fatalf("Gateway().MaxBodyBytes = %d, want %d", gatewayCfg.MaxBodyBytes, cfg.MaxBodyBytes)
	}
	if gatewayCfg.PolicySnapshotPath != cfg.PolicySnapshotPath {
		t.Fatalf("Gateway().PolicySnapshotPath = %q, want %q", gatewayCfg.PolicySnapshotPath, cfg.PolicySnapshotPath)
	}
	if gatewayCfg.UpstreamTimeout != cfg.UpstreamTimeout {
		t.Fatalf("Gateway().UpstreamTimeout = %v, want %v", gatewayCfg.UpstreamTimeout, cfg.UpstreamTimeout)
	}
	if gatewayCfg.VerdictTimeout != cfg.VerdictTimeout {
		t.Fatalf("Gateway().VerdictTimeout = %v, want %v", gatewayCfg.VerdictTimeout, cfg.VerdictTimeout)
	}

	controlCfg := cfg.Control()
	if controlCfg.Addr != cfg.ControlAddr {
		t.Fatalf("Control().Addr = %q, want %q", controlCfg.Addr, cfg.ControlAddr)
	}
	if controlCfg.AuthToken != cfg.ControlAuthToken {
		t.Fatalf("Control().AuthToken = %q, want %q", controlCfg.AuthToken, cfg.ControlAuthToken)
	}
	if controlCfg.Insecure != cfg.ControlInsecure {
		t.Fatalf("Control().Insecure = %v, want %v", controlCfg.Insecure, cfg.ControlInsecure)
	}
}

func TestControlAddressIsPublic(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{name: "empty", addr: "", want: false},
		{name: "wildcard shorthand", addr: ":8081", want: true},
		{name: "localhost", addr: "localhost:8081", want: false},
		{name: "ipv4 loopback", addr: "127.0.0.1:8081", want: false},
		{name: "ipv6 loopback", addr: "[::1]:8081", want: false},
		{name: "ipv4 wildcard", addr: "0.0.0.0:8081", want: true},
		{name: "hostname", addr: "control.internal:8081", want: true},
		{name: "host without port", addr: "control.internal", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := controlAddressIsPublic(tt.addr); got != tt.want {
				t.Fatalf("controlAddressIsPublic(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

func TestEnvParsingHelpersReturnFallbacksAndErrors(t *testing.T) {
	t.Setenv("KIWIGUARD_TEST_INT", "")
	if got, err := getEnvInt("KIWIGUARD_TEST_INT", 7); err != nil || got != 7 {
		t.Fatalf("getEnvInt(empty) = %d, %v; want fallback 7", got, err)
	}
	t.Setenv("KIWIGUARD_TEST_INT", "bad")
	if _, err := getEnvInt("KIWIGUARD_TEST_INT", 7); err == nil {
		t.Fatal("getEnvInt(bad) error = nil, want parse error")
	}

	t.Setenv("KIWIGUARD_TEST_INT64", "")
	if got, err := getEnvInt64("KIWIGUARD_TEST_INT64", 9); err != nil || got != 9 {
		t.Fatalf("getEnvInt64(empty) = %d, %v; want fallback 9", got, err)
	}
	t.Setenv("KIWIGUARD_TEST_INT64", "bad")
	if _, err := getEnvInt64("KIWIGUARD_TEST_INT64", 9); err == nil {
		t.Fatal("getEnvInt64(bad) error = nil, want parse error")
	}
}

func TestLoadFromEnvRequiresPostgresDSN(t *testing.T) {
	t.Setenv("KIWIGUARD_HTTP_ADDR", "")
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "localhost:9000")
	t.Setenv("KIWIGUARD_LOG_LEVEL", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want missing Postgres error")
	}
}

func TestLoadFromEnvRequiresClickHouseAddr(t *testing.T) {
	t.Setenv("KIWIGUARD_HTTP_ADDR", "")
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "postgres://kiwiguard:kiwiguard@localhost:5432/kiwiguard?sslmode=disable")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "")
	t.Setenv("KIWIGUARD_LOG_LEVEL", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want missing ClickHouse error")
	}
}
