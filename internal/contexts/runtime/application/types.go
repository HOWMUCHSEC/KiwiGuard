// Package runtime contains runtime configuration application use cases.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// ErrActiveRuntimeConfigNotFound reports that no active runtime config exists.
var ErrActiveRuntimeConfigNotFound = errors.New("active runtime config not found")

// RuntimeConfig is the storage-neutral active configuration shape consumed by runtime wiring.
type RuntimeConfig struct {
	Revision                     RuntimeRevision
	Routes                       []RouteConfig
	Providers                    []ProviderConfig
	ModelMappings                []ModelMappingConfig
	VerdictProviders             []VerdictProviderConfig
	RouteVerdictProviderBindings []RouteVerdictProviderBindingConfig
	Sinks                        []SinkConfig
	Retention                    []RetentionPolicyConfig
	PolicyBundles                []policy.Bundle
	RawCapture                   []RawCaptureConfig
	GatewayClients               []GatewayClientConfig
	RouteLimits                  []RouteLimitConfig
	ClientRouteLimitOverrides    []ClientRouteLimitOverrideConfig
}

// RuntimeRevision names the active configuration revision loaded into runtime assembly.
type RuntimeRevision struct {
	Number int64
}

// RouteConfig is the storage-neutral route contract consumed by runtime compilation.
type RouteConfig struct {
	Key            string
	Method         string
	Path           string
	ProviderKey    string
	RequestedModel string
	MappedModel    string
	UpstreamModel  string
	ExecutionMode  string
	FallbackAction string
	Disabled       bool
}

// ProviderConfig is the storage-neutral provider contract consumed by runtime compilation.
type ProviderConfig struct {
	Key           string
	BaseURL       string
	APIKey        string
	CredentialRef string
	Headers       map[string]string
	Timeout       time.Duration
	Disabled      bool
}

// ModelMappingConfig is the storage-neutral model mapping contract consumed by runtime compilation.
type ModelMappingConfig struct {
	Key            string
	RouteKey       string
	ProviderKey    string
	RequestedModel string
	MappedModel    string
	UpstreamModel  string
	Disabled       bool
}

// VerdictProviderConfig is the storage-neutral verdict provider contract consumed by runtime compilation.
type VerdictProviderConfig struct {
	Key           string
	Name          string
	Endpoint      string
	CredentialRef string
	Enabled       bool
	Timeout       time.Duration
}

// RouteVerdictProviderBindingConfig binds one route to a verdict provider and execution mode.
type RouteVerdictProviderBindingConfig struct {
	RouteKey           string
	VerdictProviderKey string
	ExecutionMode      string
	Disabled           bool
	Priority           int
}

// SinkConfig is the storage-neutral event-sink contract consumed by runtime assembly.
type SinkConfig struct {
	ID       string
	Key      string
	Kind     string
	Disabled bool
	Config   json.RawMessage
}

// RetentionPolicyConfig is the storage-neutral event-retention contract consumed by background maintenance.
type RetentionPolicyConfig struct {
	ID            string
	Key           string
	SinkKey       string
	EventType     string
	RetentionDays int
}

// RawCaptureConfig declares when raw request or response bodies may be mirrored for observability.
type RawCaptureConfig struct {
	ID            string
	RouteKey      string
	Direction     string
	Enabled       bool
	SampleRate    float64
	RedactionMode string
}

// GatewayClientConfig is the storage-neutral gateway client credential contract.
type GatewayClientConfig struct {
	ID        string
	Name      string
	Status    string
	KeyPrefix string
	KeyHash   string
}

// RouteLimitConfig is the storage-neutral default limit policy for one route.
type RouteLimitConfig struct {
	RouteKey              string
	RequestsPerWindow     int
	Window                time.Duration
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Disabled              bool
}

// ClientRouteLimitOverrideConfig is the storage-neutral per-client limit override for one route.
type ClientRouteLimitOverrideConfig struct {
	ClientID              string
	RouteKey              string
	RequestsPerWindow     int
	Window                time.Duration
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Disabled              bool
}

// CompiledRuntime is an immutable gateway runtime snapshot.
type CompiledRuntime struct {
	RevisionNumber int64
	SnapshotHash   string
	Gateway        GatewayRuntime
	LoadedAt       time.Time
}

// GatewayRuntime exposes compiled gateway state without leaking transport-specific types.
type GatewayRuntime interface {
	RouteCount() int
}

// RuntimeConfigLoader provides the active runtime aggregate to application use cases.
type RuntimeConfigLoader interface {
	LoadRuntimeConfig(context.Context) (RuntimeConfig, error)
}

// ActiveRevisionReader exposes the currently active runtime revision number.
type ActiveRevisionReader interface {
	ActiveRevisionNumber(context.Context) (int64, error)
}

// ConfigRepository groups the read ports needed to load and version active runtime configuration.
type ConfigRepository interface {
	RuntimeConfigLoader
	ActiveRevisionReader
}

// Compiler turns the active runtime aggregate into an immutable compiled gateway snapshot.
type Compiler interface {
	CompileRuntime(context.Context, RuntimeConfig) (CompiledRuntime, error)
}

// CompilerFunc adapts a function into a Compiler.
type CompilerFunc func(context.Context, RuntimeConfig) (CompiledRuntime, error)

// CompileRuntime compiles runtime config.
func (f CompilerFunc) CompileRuntime(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
	return f(ctx, cfg)
}

// State owns the compiled runtime snapshot currently published to serving code.
type State interface {
	Snapshot() CompiledRuntime
	Swap(CompiledRuntime) error
}

// Readiness records whether runtime configuration is currently safe to serve.
type Readiness interface {
	MarkConfigReady()
	MarkConfigDegraded(string)
}

// RetentionApplier applies retention policies for one active runtime config.
type RetentionApplier interface {
	ApplyRetentionPolicies(context.Context, RuntimeConfig) error
}
