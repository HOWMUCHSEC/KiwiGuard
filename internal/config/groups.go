package config

import "time"

// HTTPServerConfig groups server deadline and shutdown settings.
type HTTPServerConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// StorageConfig groups durable storage endpoints.
type StorageConfig struct {
	PostgresDSN        string
	ClickHouseAddr     string
	ClickHouseDatabase string
	ClickHouseUsername string
	ClickHousePassword string
}

// EventsConfig groups asynchronous event sink and durable spool settings.
type EventsConfig struct {
	SinkType            string
	QueueCapacity       int
	BatchSize           int
	SpoolDir            string
	SpoolMaxBytes       int64
	SpoolMaxAge         time.Duration
	SpoolReplayInterval time.Duration
	SpoolBatchSize      int
}

// GatewayConfig groups gateway upstream and verdict settings.
type GatewayConfig struct {
	Addr               string
	UpstreamBaseURL    string
	UpstreamAPIKey     string
	VerdictEndpoint    string
	MaxBodyBytes       int64
	PolicySnapshotPath string
	UpstreamTimeout    time.Duration
	VerdictTimeout     time.Duration
}

// ControlConfig groups control-plane listener and authentication settings.
type ControlConfig struct {
	Addr      string
	AuthToken string
	Insecure  bool
}

// HTTPServer returns grouped server deadline and shutdown settings.
func (c Config) HTTPServer() HTTPServerConfig {
	return HTTPServerConfig{
		ReadHeaderTimeout: c.ServerReadHeaderTimeout,
		ReadTimeout:       c.ServerReadTimeout,
		WriteTimeout:      c.ServerWriteTimeout,
		IdleTimeout:       c.ServerIdleTimeout,
		ShutdownTimeout:   c.ShutdownTimeout,
	}
}

// Storage returns grouped durable storage settings.
func (c Config) Storage() StorageConfig {
	return StorageConfig{
		PostgresDSN:        c.PostgresDSN,
		ClickHouseAddr:     c.ClickHouseAddr,
		ClickHouseDatabase: c.ClickHouseDatabase,
		ClickHouseUsername: c.ClickHouseUsername,
		ClickHousePassword: c.ClickHousePassword,
	}
}

// Events returns grouped asynchronous event sink settings.
func (c Config) Events() EventsConfig {
	return EventsConfig{
		SinkType:            c.EventSinkType,
		QueueCapacity:       c.EventQueueCapacity,
		BatchSize:           c.EventBatchSize,
		SpoolDir:            c.EventSpoolDir,
		SpoolMaxBytes:       c.EventSpoolMaxBytes,
		SpoolMaxAge:         c.EventSpoolMaxAge,
		SpoolReplayInterval: c.EventSpoolReplayInterval,
		SpoolBatchSize:      c.EventSpoolBatchSize,
	}
}

// Gateway returns grouped gateway upstream and verdict settings.
func (c Config) Gateway() GatewayConfig {
	return GatewayConfig{
		Addr:               c.GatewayAddr,
		UpstreamBaseURL:    c.UpstreamBaseURL,
		UpstreamAPIKey:     c.UpstreamAPIKey,
		VerdictEndpoint:    c.VerdictEndpoint,
		MaxBodyBytes:       c.MaxBodyBytes,
		PolicySnapshotPath: c.PolicySnapshotPath,
		UpstreamTimeout:    c.UpstreamTimeout,
		VerdictTimeout:     c.VerdictTimeout,
	}
}

// Control returns grouped control-plane listener and authentication settings.
func (c Config) Control() ControlConfig {
	return ControlConfig{
		Addr:      c.ControlAddr,
		AuthToken: c.ControlAuthToken,
		Insecure:  c.ControlInsecure,
	}
}
