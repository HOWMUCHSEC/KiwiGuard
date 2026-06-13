// Package openai adapts compiled runtime configuration to the HTTP gateway transport.
package openai

import (
	"time"

	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

type (
	RuntimeConfig                     = appruntime.RuntimeConfig
	RuntimeRevision                   = appruntime.RuntimeRevision
	RouteConfig                       = appruntime.RouteConfig
	ProviderConfig                    = appruntime.ProviderConfig
	ModelMappingConfig                = appruntime.ModelMappingConfig
	VerdictProviderConfig             = appruntime.VerdictProviderConfig
	RouteVerdictProviderBindingConfig = appruntime.RouteVerdictProviderBindingConfig
	RawCaptureConfig                  = appruntime.RawCaptureConfig
	GatewayClientConfig               = appruntime.GatewayClientConfig
	RouteLimitConfig                  = appruntime.RouteLimitConfig
	ClientRouteLimitOverrideConfig    = appruntime.ClientRouteLimitOverrideConfig
	CompiledRuntime                   = appruntime.CompiledRuntime
)

// CompileOptions carries dependencies and deadlines into gateway HTTP runtime compilation.
type CompileOptions struct {
	MaxBodyBytes       int64
	UpstreamTimeout    time.Duration
	VerdictTimeout     time.Duration
	EventWriter        events.Writer
	AuditGate          AuditGate
	CredentialResolver CredentialResolver
}
