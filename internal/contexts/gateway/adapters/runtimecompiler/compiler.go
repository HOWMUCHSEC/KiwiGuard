// Package runtimecompiler compiles application runtime configuration for the HTTP gateway adapter.
package runtimecompiler

import (
	"context"
	"errors"
	"fmt"
	"time"

	gatewayhttp "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/http/openai"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

type (
	// RuntimeConfig is the storage-neutral active runtime configuration.
	RuntimeConfig = appruntime.RuntimeConfig
	// RuntimeRevision identifies an active configuration revision.
	RuntimeRevision = appruntime.RuntimeRevision
	// RouteConfig describes an enabled gateway route loaded from configuration storage.
	RouteConfig = appruntime.RouteConfig
	// ProviderConfig describes an upstream model provider.
	ProviderConfig = appruntime.ProviderConfig
	// ModelMappingConfig describes a model mapping record.
	ModelMappingConfig = appruntime.ModelMappingConfig
	// VerdictProviderConfig describes a vertical verdict provider.
	VerdictProviderConfig = appruntime.VerdictProviderConfig
	// RouteVerdictProviderBindingConfig selects the verdict provider for a route.
	RouteVerdictProviderBindingConfig = appruntime.RouteVerdictProviderBindingConfig
	// RawCaptureConfig describes when raw request and response bodies may be mirrored.
	RawCaptureConfig = appruntime.RawCaptureConfig
	// GatewayClientConfig describes a gateway beta client key.
	GatewayClientConfig = appruntime.GatewayClientConfig
	// RouteLimitConfig describes default gateway limits for a route.
	RouteLimitConfig = appruntime.RouteLimitConfig
	// ClientRouteLimitOverrideConfig describes gateway limits for one client on one route.
	ClientRouteLimitOverrideConfig = appruntime.ClientRouteLimitOverrideConfig
	// CompileOptions carries dependencies and deadlines into gateway runtime compilation.
	CompileOptions = gatewayhttp.CompileOptions
)

// CompileGatewayRuntime converts active runtime records into a gateway configuration.
func CompileGatewayRuntime(input appruntime.RuntimeConfig, opts CompileOptions) (appruntime.CompiledRuntime, error) {
	if input.Revision.Number <= 0 {
		return appruntime.CompiledRuntime{}, errors.New("compile gateway runtime: revision number is required")
	}

	snapshot, err := policy.CompileSnapshot(input.PolicyBundles)
	if err != nil {
		return appruntime.CompiledRuntime{}, fmt.Errorf("compile gateway runtime policy snapshot: %w", err)
	}

	cfg, err := gatewayhttp.BuildConfig(input, opts, snapshot)
	if err != nil {
		return appruntime.CompiledRuntime{}, err
	}

	return appruntime.CompiledRuntime{
		RevisionNumber: input.Revision.Number,
		SnapshotHash:   snapshot.Hash(),
		Gateway:        gatewayhttp.NewCompiledGatewayRuntime(cfg),
		LoadedAt:       time.Now(),
	}, nil
}

// Compiler compiles runtime config with fixed gateway adapter options.
type Compiler struct {
	Options CompileOptions
}

// CompileRuntime compiles runtime config into a gateway runtime.
func (c Compiler) CompileRuntime(ctx context.Context, cfg appruntime.RuntimeConfig) (appruntime.CompiledRuntime, error) {
	return CompileGatewayRuntime(cfg, c.Options)
}
