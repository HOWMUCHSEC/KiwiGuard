package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains KiwiGuard runtime configuration.
type Config struct {
	HTTPAddr                 string
	GatewayAddr              string
	ControlAddr              string
	ControlAuthToken         string
	ControlInsecure          bool
	UpstreamBaseURL          string
	UpstreamAPIKey           string
	VerdictEndpoint          string
	MaxBodyBytes             int64
	PolicySnapshotPath       string
	PostgresDSN              string
	ClickHouseAddr           string
	ClickHouseDatabase       string
	ClickHouseUsername       string
	ClickHousePassword       string
	EventSinkType            string
	EventQueueCapacity       int
	EventBatchSize           int
	EventSpoolDir            string
	EventSpoolMaxBytes       int64
	EventSpoolMaxAge         time.Duration
	EventSpoolReplayInterval time.Duration
	EventSpoolBatchSize      int
	LogLevel                 string
	UpstreamTimeout          time.Duration
	VerdictTimeout           time.Duration
	ServerReadHeaderTimeout  time.Duration
	ServerReadTimeout        time.Duration
	ServerWriteTimeout       time.Duration
	ServerIdleTimeout        time.Duration
	ShutdownTimeout          time.Duration
}

// LoadFromEnv reads runtime configuration from environment variables.
func LoadFromEnv() (Config, error) {
	maxBodyBytes, err := getEnvInt64("KIWIGUARD_MAX_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	eventQueueCapacity, err := getEnvInt("KIWIGUARD_EVENT_QUEUE_CAPACITY", 1024)
	if err != nil {
		return Config{}, err
	}
	eventBatchSize, err := getEnvInt("KIWIGUARD_EVENT_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	eventSpoolMaxBytes, err := getEnvInt64("KIWIGUARD_EVENT_SPOOL_MAX_BYTES", 1<<30)
	if err != nil {
		return Config{}, err
	}
	eventSpoolMaxAge, err := getEnvDuration("KIWIGUARD_EVENT_SPOOL_MAX_AGE", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	eventSpoolReplayInterval, err := getEnvDuration("KIWIGUARD_EVENT_SPOOL_REPLAY_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	eventSpoolBatchSize, err := getEnvInt("KIWIGUARD_EVENT_SPOOL_BATCH_SIZE", eventBatchSize)
	if err != nil {
		return Config{}, err
	}
	upstreamTimeout, err := getEnvDuration("KIWIGUARD_UPSTREAM_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	verdictTimeout, err := getEnvDuration("KIWIGUARD_VERDICT_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	serverReadHeaderTimeout, err := getEnvDuration("KIWIGUARD_SERVER_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	serverReadTimeout, err := getEnvDuration("KIWIGUARD_SERVER_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	serverWriteTimeout, err := getEnvDuration("KIWIGUARD_SERVER_WRITE_TIMEOUT", time.Minute)
	if err != nil {
		return Config{}, err
	}
	serverIdleTimeout, err := getEnvDuration("KIWIGUARD_SERVER_IDLE_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := getEnvDuration("KIWIGUARD_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	controlInsecure, err := getEnvBool("KIWIGUARD_CONTROL_INSECURE", false)
	if err != nil {
		return Config{}, err
	}
	gatewayAddr := getEnv("KIWIGUARD_HTTP_ADDR", ":8080")
	cfg := Config{
		HTTPAddr:                 gatewayAddr,
		GatewayAddr:              gatewayAddr,
		ControlAddr:              getEnv("KIWIGUARD_CONTROL_ADDR", "127.0.0.1:8081"),
		ControlAuthToken:         os.Getenv("KIWIGUARD_CONTROL_AUTH_TOKEN"),
		ControlInsecure:          controlInsecure,
		UpstreamBaseURL:          os.Getenv("KIWIGUARD_UPSTREAM_BASE_URL"),
		UpstreamAPIKey:           os.Getenv("KIWIGUARD_UPSTREAM_API_KEY"),
		VerdictEndpoint:          os.Getenv("KIWIGUARD_VERDICT_ENDPOINT"),
		MaxBodyBytes:             maxBodyBytes,
		PolicySnapshotPath:       os.Getenv("KIWIGUARD_POLICY_SNAPSHOT_PATH"),
		PostgresDSN:              os.Getenv("KIWIGUARD_POSTGRES_DSN"),
		ClickHouseAddr:           os.Getenv("KIWIGUARD_CLICKHOUSE_ADDR"),
		ClickHouseDatabase:       getEnv("KIWIGUARD_CLICKHOUSE_DATABASE", "kiwiguard"),
		ClickHouseUsername:       getEnv("KIWIGUARD_CLICKHOUSE_USERNAME", "default"),
		ClickHousePassword:       os.Getenv("KIWIGUARD_CLICKHOUSE_PASSWORD"),
		EventSinkType:            getEnv("KIWIGUARD_EVENT_SINK_TYPE", "clickhouse"),
		EventQueueCapacity:       eventQueueCapacity,
		EventBatchSize:           eventBatchSize,
		EventSpoolDir:            getEnv("KIWIGUARD_EVENT_SPOOL_DIR", "./data/event-spool"),
		EventSpoolMaxBytes:       eventSpoolMaxBytes,
		EventSpoolMaxAge:         eventSpoolMaxAge,
		EventSpoolReplayInterval: eventSpoolReplayInterval,
		EventSpoolBatchSize:      eventSpoolBatchSize,
		LogLevel:                 getEnv("KIWIGUARD_LOG_LEVEL", "info"),
		UpstreamTimeout:          upstreamTimeout,
		VerdictTimeout:           verdictTimeout,
		ServerReadHeaderTimeout:  serverReadHeaderTimeout,
		ServerReadTimeout:        serverReadTimeout,
		ServerWriteTimeout:       serverWriteTimeout,
		ServerIdleTimeout:        serverIdleTimeout,
		ShutdownTimeout:          shutdownTimeout,
	}

	if cfg.PostgresDSN == "" {
		return Config{}, errors.New("KIWIGUARD_POSTGRES_DSN is required")
	}
	if cfg.ClickHouseAddr == "" {
		return Config{}, errors.New("KIWIGUARD_CLICKHOUSE_ADDR is required")
	}
	if controlAddressIsPublic(cfg.ControlAddr) && cfg.ControlAuthToken == "" && !cfg.ControlInsecure {
		return Config{}, errors.New("KIWIGUARD_CONTROL_AUTH_TOKEN is required when KIWIGUARD_CONTROL_ADDR is public; set KIWIGUARD_CONTROL_INSECURE=true only for isolated development")
	}

	return cfg, nil
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func controlAddressIsPublic(addr string) bool {
	if addr == "" {
		return false
	}
	if strings.HasPrefix(addr, ":") {
		return true
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host, _, _ = strings.Cut(addr, ":")
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "localhost", "::1":
		return false
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return !ip.IsLoopback()
	}
	return host != ""
}
