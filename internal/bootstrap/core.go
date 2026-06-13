package bootstrap

import (
	"context"
	"errors"

	"github.com/howmuchsec/kiwiguard/internal/config"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
	gatewayhttp "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/http/openai"
	"github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/runtimecompiler"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/observability"
	"go.opentelemetry.io/otel/trace"
)

// Cleanup releases resources allocated for a runtime child.
type Cleanup = func(context.Context) error

// FactoryOptions supplies the infrastructure dependencies used to assemble production services.
type FactoryOptions struct {
	Config     config.Config
	Repository kgruntime.RuntimeConfigRepository
	Compiler   kgruntime.RuntimeCompiler
	Store      controlhttp.PolicyStore
	Notifier   controlhttp.ActivationNotifier
	Subscriber kgruntime.RevisionSubscriber

	TracerProvider trace.TracerProvider

	productionDeps      productionDepsOpener
	productionResources productionResourceOpener
}

// Factory builds production runtime handlers and workers.
type Factory struct {
	options      FactoryOptions
	configHealth *kgruntime.ReadinessState
	metrics      *observability.PrometheusMetrics
	telemetry    *observability.OpenTelemetry
}

// NewFactory assembles a production factory from application configuration.
func NewFactory(cfg config.Config) *Factory {
	return NewFactoryWithOptions(FactoryOptions{Config: cfg})
}

// NewFactoryWithOptions assembles a factory with caller-provided infrastructure overrides.
func NewFactoryWithOptions(opts FactoryOptions) *Factory {
	return &Factory{
		options:      opts,
		configHealth: kgruntime.NewReadinessState(),
		metrics:      observability.NewPrometheusMetrics(),
		telemetry:    observability.NewOpenTelemetry(opts.TracerProvider),
	}
}

func (f *Factory) validateRequiredDependencies() error {
	if f.options.Config.PostgresDSN == "" {
		return errors.New("postgres dsn is required")
	}
	if f.options.Config.ClickHouseAddr == "" {
		return errors.New("clickhouse address is required")
	}
	return nil
}

func (f *Factory) loadCompiledRuntime(ctx context.Context, writer events.Writer, gate gatewayhttp.AuditGate) (kgruntime.CompiledRuntime, error) {
	if f.options.Repository == nil {
		return kgruntime.CompiledRuntime{}, kgruntime.ErrActiveRuntimeConfigNotFound
	}
	return f.loadCompiledRuntimeFrom(ctx, f.options.Repository, writer, gate)
}

func (f *Factory) loadCompiledRuntimeFrom(ctx context.Context, repo kgruntime.RuntimeConfigRepository, writer events.Writer, gate gatewayhttp.AuditGate) (kgruntime.CompiledRuntime, error) {
	return f.loadCompiledRuntimeWithCompiler(ctx, repo, f.compilerWith(writer, gate))
}

func (f *Factory) compiler() kgruntime.RuntimeCompiler {
	return f.compilerWith(nil, nil)
}

func (f *Factory) compilerWith(writer events.Writer, gate gatewayhttp.AuditGate) kgruntime.RuntimeCompiler {
	if f.options.Compiler != nil {
		return f.options.Compiler
	}
	return runtimecompiler.Compiler{Options: runtimecompiler.CompileOptions{
		MaxBodyBytes:       f.options.Config.MaxBodyBytes,
		UpstreamTimeout:    f.options.Config.UpstreamTimeout,
		VerdictTimeout:     f.options.Config.VerdictTimeout,
		EventWriter:        writer,
		AuditGate:          gate,
		CredentialResolver: gatewayhttp.DefaultCredentialResolver(),
	}}
}

func noopCleanup(ctx context.Context) error {
	return nil
}
