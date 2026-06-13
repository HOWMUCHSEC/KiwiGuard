// Package runtime owns active runtime state, watchers, workers, and runtime ports.
package runtime

import appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"

// ErrActiveRuntimeConfigNotFound reports that no active runtime config exists.
var ErrActiveRuntimeConfigNotFound = appruntime.ErrActiveRuntimeConfigNotFound

// RuntimeConfig is the storage-neutral active configuration shape consumed by runtime wiring.
type RuntimeConfig = appruntime.RuntimeConfig

// RuntimeRevision aliases the application-layer revision contract exposed by runtime.
type RuntimeRevision = appruntime.RuntimeRevision

// RouteConfig aliases the application-layer route contract exposed by runtime.
type RouteConfig = appruntime.RouteConfig

// ProviderConfig aliases the application-layer provider contract exposed by runtime.
type ProviderConfig = appruntime.ProviderConfig

// ModelMappingConfig aliases the application-layer model mapping contract exposed by runtime.
type ModelMappingConfig = appruntime.ModelMappingConfig

// VerdictProviderConfig aliases the application-layer verdict provider contract exposed by runtime.
type VerdictProviderConfig = appruntime.VerdictProviderConfig

// RouteVerdictProviderBindingConfig aliases the route-to-verdict binding contract exposed by runtime.
type RouteVerdictProviderBindingConfig = appruntime.RouteVerdictProviderBindingConfig

// SinkConfig aliases the application-layer event sink contract exposed by runtime.
type SinkConfig = appruntime.SinkConfig

// RetentionPolicyConfig aliases the application-layer retention contract exposed by runtime.
type RetentionPolicyConfig = appruntime.RetentionPolicyConfig

// RawCaptureConfig aliases the application-layer raw-capture policy contract exposed by runtime.
type RawCaptureConfig = appruntime.RawCaptureConfig

// GatewayClientConfig aliases the application-layer gateway client contract exposed by runtime.
type GatewayClientConfig = appruntime.GatewayClientConfig

// RouteLimitConfig aliases the application-layer default route limit contract exposed by runtime.
type RouteLimitConfig = appruntime.RouteLimitConfig

// ClientRouteLimitOverrideConfig aliases the application-layer per-client route limit contract exposed by runtime.
type ClientRouteLimitOverrideConfig = appruntime.ClientRouteLimitOverrideConfig

// CompiledRuntime is an immutable gateway runtime snapshot.
type CompiledRuntime = appruntime.CompiledRuntime

// RuntimeConfigRepository aliases the application-layer port that reads active runtime configuration.
type RuntimeConfigRepository = appruntime.ConfigRepository

// RuntimeConfigLoader aliases the application-layer loader for the active runtime aggregate.
type RuntimeConfigLoader = appruntime.RuntimeConfigLoader

// ActiveRevisionReader aliases the application-layer reader for the active revision token.
type ActiveRevisionReader = appruntime.ActiveRevisionReader

// RuntimeCompiler aliases the application-layer compiler for immutable gateway snapshots.
type RuntimeCompiler = appruntime.Compiler

// RuntimeCompilerFunc adapts a function into a RuntimeCompiler.
type RuntimeCompilerFunc = appruntime.CompilerFunc
