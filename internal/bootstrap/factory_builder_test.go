package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	eventassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/events"
	observabilityassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/observability"
	"github.com/howmuchsec/kiwiguard/internal/config"
	"github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/runtimecompiler"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestFactoryFailsGatewayStartupWithoutPostgresDSN(t *testing.T) {
	factory := NewFactory(config.Config{ClickHouseAddr: "localhost:9000"})

	_, _, err := factory.GatewayHandler(context.Background())
	if err == nil {
		t.Fatal("GatewayHandler() error = nil, want missing postgres error")
	}
}

func TestFactoryFailsGatewayStartupWithoutClickHouseAddr(t *testing.T) {
	factory := NewFactory(config.Config{PostgresDSN: "postgres://example"})

	_, _, err := factory.GatewayHandler(context.Background())
	if err == nil {
		t.Fatal("GatewayHandler() error = nil, want missing clickhouse error")
	}
}

func TestFactoryControlAllowsMissingActiveConfig(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: config.Config{
			PostgresDSN:    "postgres://example",
			ClickHouseAddr: "localhost:9000",
		},
		Repository: &missingRuntimeRepository{},
	})

	handler, cleanup, err := factory.ControlHandler(context.Background(), "test")
	if err != nil {
		t.Fatalf("ControlHandler() error = %v", err)
	}
	if handler == nil {
		t.Fatal("ControlHandler() handler is nil")
	}
	if cleanup == nil {
		t.Fatal("ControlHandler() cleanup is nil")
	}
}

func TestFactoryControlHandlerReportsSharedConfigHealth(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
	})
	factory.configHealth.MarkConfigDegraded("runtime_config_compile_failed")

	handler, cleanup, err := factory.ControlHandler(context.Background(), "test")
	if err != nil {
		t.Fatalf("ControlHandler() error = %v", err)
	}
	defer func() {
		if err := cleanup(context.Background()); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var body struct {
		Checks map[string]struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Checks["config"].Status != "degraded" || body.Checks["config"].Reason != "runtime_config_compile_failed" {
		t.Fatalf("config check = %+v, want degraded compile failure", body.Checks["config"])
	}
}

func TestFactoryControlHandlerExposesHTTPMetrics(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
	})

	handler, cleanup, err := factory.ControlHandler(context.Background(), "test")
	if err != nil {
		t.Fatalf("ControlHandler() error = %v", err)
	}
	defer func() {
		if err := cleanup(context.Background()); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `kiwiguard_http_requests_total{method="GET",route="/api/healthz",service="control",status="200"} 1`) {
		t.Fatalf("metrics body does not include control HTTP metrics:\n%s", rec.Body.String())
	}
}

func TestFactoryControlHandlerCreatesOpenTelemetrySpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:         requiredConfig(),
		Repository:     &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		TracerProvider: tracerProvider,
	})

	handler, cleanup, err := factory.ControlHandler(context.Background(), "test")
	if err != nil {
		t.Fatalf("ControlHandler() error = %v", err)
	}
	defer func() {
		if err := cleanup(context.Background()); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/healthz", nil))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "control GET /api/healthz" {
		t.Fatalf("span name = %q, want control health request", spans[0].Name)
	}
}

func TestFileSpoolStatusProviderReportsBacklog(t *testing.T) {
	dir := t.TempDir()
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []events.Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	provider, err := newFileSpoolStatusProvider(config.Config{
		EventSpoolDir:      dir,
		EventSpoolMaxBytes: 1 << 20,
		EventSpoolMaxAge:   time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("newFileSpoolStatusProvider() error = %v", err)
	}

	status := provider.SpoolStatus()
	if !status.Enabled || status.Status != "backlogged" || status.Reason != "event_spool_backlog" {
		t.Fatalf("status = %+v, want backlogged enabled status", status)
	}
	if status.Depth != 1 || status.Bytes == 0 || status.MaxBytes != 1<<20 {
		t.Fatalf("spool stats = %+v, want depth, bytes, and max bytes", status)
	}
	if status.OldestAgeSeconds < 0 {
		t.Fatalf("OldestAgeSeconds = %v, want non-negative", status.OldestAgeSeconds)
	}
}

func TestFileSpoolStatusProviderReportsEmptyHealthySpool(t *testing.T) {
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	provider := newFileSpoolStatusProviderFromSpool(spool, nil)

	status := provider.SpoolStatus()
	if !status.Enabled || status.Status != "ok" || status.Reason != "" {
		t.Fatalf("status = %+v, want healthy empty spool", status)
	}
}

func TestFileSpoolStatusProviderUsesSharedSpoolInstance(t *testing.T) {
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	provider := newFileSpoolStatusProviderFromSpool(spool, nil)

	if err := spool.Append(context.Background(), []events.Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	status := provider.SpoolStatus()
	if status.Depth != 1 || status.Status != "backlogged" {
		t.Fatalf("status = %+v, want shared spool backlog", status)
	}
}

func TestFileSpoolStatusProviderObservesPrometheusStats(t *testing.T) {
	dir := t.TempDir()
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	if err := spool.Append(context.Background(), []events.Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	metrics := observability.NewPrometheusMetrics()

	provider, err := newFileSpoolStatusProvider(config.Config{
		EventSpoolDir:      dir,
		EventSpoolMaxBytes: 1 << 20,
		EventSpoolMaxAge:   time.Hour,
	}, metrics)
	if err != nil {
		t.Fatalf("newFileSpoolStatusProvider() error = %v", err)
	}

	_ = provider.SpoolStatus()

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "kiwiguard_event_spool_depth 1") {
		t.Fatalf("metrics body does not include spool depth:\n%s", rec.Body.String())
	}
}

func TestFileSpoolStatusProviderReportsPersistedOverflow(t *testing.T) {
	dir := t.TempDir()
	writer, err := events.NewFileSpool(events.FileSpoolOptions{Dir: dir, MaxBytes: 64})
	if err != nil {
		t.Fatalf("NewFileSpool(writer) error = %v", err)
	}
	err = writer.Append(context.Background(), []events.Event{{EventID: "evt-1", RequestPayload: "payload larger than the tiny spool budget"}})
	if !errors.Is(err, events.ErrSpoolFull) {
		t.Fatalf("Append() error = %v, want ErrSpoolFull", err)
	}

	provider, err := newFileSpoolStatusProvider(config.Config{
		EventSpoolDir:      dir,
		EventSpoolMaxBytes: 64,
		EventSpoolMaxAge:   time.Hour,
	}, nil)
	if err != nil {
		t.Fatalf("newFileSpoolStatusProvider() error = %v", err)
	}

	status := provider.SpoolStatus()
	if status.Status != "degraded" || status.Reason != "event_spool_overflow" || status.OverflowCount != 1 {
		t.Fatalf("status = %+v, want degraded overflow status", status)
	}
}

func TestFileSpoolStatusProviderDisabledAndCreationError(t *testing.T) {
	if status := (*fileSpoolStatusProvider)(nil).SpoolStatus(); status.Enabled || status.Status != "disabled" {
		t.Fatalf("nil provider status = %+v, want disabled", status)
	}

	filePath := t.TempDir() + "/spool-file"
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := newFileSpoolStatusProvider(config.Config{EventSpoolDir: filePath}, nil)
	if err == nil {
		t.Fatal("newFileSpoolStatusProvider() error = nil, want directory error")
	}
}

func TestFactoryGatewayRequiresActiveConfig(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: config.Config{
			PostgresDSN:    "postgres://example",
			ClickHouseAddr: "localhost:9000",
		},
		Repository: &missingRuntimeRepository{},
	})

	_, _, err := factory.GatewayHandler(context.Background())
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("GatewayHandler() error = %v, want %v", err, ErrActiveRuntimeConfigNotFound)
	}
}

func TestFactoryGatewayHandlerReturnsCompileError(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{}},
	})

	_, _, err := factory.GatewayHandler(context.Background())
	if err == nil {
		t.Fatal("GatewayHandler() error = nil, want compile error")
	}
	if !strings.Contains(err.Error(), "compile active runtime") {
		t.Fatalf("GatewayHandler() error = %v, want wrapped compile error", err)
	}
}

func TestFactoryGatewayHandlerRejectsInvalidCompiledRuntime(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		Compiler: RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			return CompiledRuntime{}, nil
		}),
	})

	_, _, err := factory.GatewayHandler(context.Background())
	if err == nil {
		t.Fatal("GatewayHandler() error = nil, want active runtime state error")
	}
	if !strings.Contains(err.Error(), "create active runtime state") {
		t.Fatalf("GatewayHandler() error = %v, want active runtime state error", err)
	}
}

func TestFactoryGatewayHandlerReportsCompiledServerError(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		Compiler: RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			return CompiledRuntime{
				RevisionNumber: cfg.Revision.Number,
				Gateway:        fakeGatewayRuntime{routeKey: "chat"},
			}, nil
		}),
	})

	_, _, err := factory.GatewayHandler(context.Background())
	if err == nil {
		t.Fatal("GatewayHandler() error = nil, want compiled server error")
	}
	if !strings.Contains(err.Error(), "create gateway handler") {
		t.Fatalf("GatewayHandler() error = %v, want gateway handler context", err)
	}
}

func TestFactoryWorkerRequiresActiveConfig(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &missingRuntimeRepository{},
	})

	_, _, err := factory.Worker(context.Background())
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("Worker() error = %v, want %v", err, ErrActiveRuntimeConfigNotFound)
	}
}

func TestFactoryWorkerRejectsInvalidCompiledRuntime(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		Compiler: RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			return CompiledRuntime{}, nil
		}),
	})

	_, _, err := factory.Worker(context.Background())
	if err == nil {
		t.Fatal("Worker() error = nil, want active runtime state error")
	}
	if !strings.Contains(err.Error(), "create active runtime state") {
		t.Fatalf("Worker() error = %v, want active runtime state error", err)
	}
}

func TestFactoryWorkerReturnsCompileError(t *testing.T) {
	wantErr := errors.New("bad worker config")
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		Compiler: RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			return CompiledRuntime{}, wantErr
		}),
	})

	_, _, err := factory.Worker(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Worker() error = %v, want %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "compile active runtime") {
		t.Fatalf("Worker() error = %v, want compile context", err)
	}
}

func TestFactoryWorkerFailsStartupWithoutRequiredDependencies(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{
			name: "postgres dsn",
			cfg:  config.Config{ClickHouseAddr: "localhost:9000"},
		},
		{
			name: "clickhouse address",
			cfg:  config.Config{PostgresDSN: "postgres://example"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory(tt.cfg)

			_, _, err := factory.Worker(context.Background())
			if err == nil {
				t.Fatal("Worker() error = nil, want missing dependency error")
			}
		})
	}
}

func TestFactoryProductionHandlersReturnMigrationError(t *testing.T) {
	cfg := requiredConfig()
	cfg.PostgresDSN = "://bad"

	tests := []struct {
		name string
		run  func(*Factory) error
	}{
		{
			name: "gateway",
			run: func(factory *Factory) error {
				_, _, err := factory.GatewayHandler(context.Background())
				return err
			},
		},
		{
			name: "control",
			run: func(factory *Factory) error {
				_, _, err := factory.ControlHandler(context.Background(), "test")
				return err
			},
		},
		{
			name: "worker",
			run: func(factory *Factory) error {
				_, _, err := factory.Worker(context.Background())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(NewFactory(cfg))
			if err == nil {
				t.Fatal("handler error = nil, want migration error")
			}
		})
	}
}

func TestFactoryProductionHandlersUseProductionDepsOpener(t *testing.T) {
	tests := []struct {
		name              string
		run               func(*Factory) (any, Cleanup, error)
		wantEventPipeline bool
	}{
		{
			name: "gateway",
			run: func(factory *Factory) (any, Cleanup, error) {
				return factory.GatewayHandler(context.Background())
			},
			wantEventPipeline: true,
		},
		{
			name: "control",
			run: func(factory *Factory) (any, Cleanup, error) {
				return factory.ControlHandler(context.Background(), "test")
			},
			wantEventPipeline: false,
		},
		{
			name: "worker",
			run: func(factory *Factory) (any, Cleanup, error) {
				return factory.Worker(context.Background())
			},
			wantEventPipeline: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opener := &recordingProductionDepsOpener{}
			factory := NewFactoryWithOptions(FactoryOptions{
				Config:         requiredConfig(),
				productionDeps: opener.open,
			})

			child, cleanup, err := tt.run(factory)
			if err != nil {
				t.Fatalf("production handler error = %v", err)
			}
			if child == nil || cleanup == nil {
				t.Fatalf("child=%v cleanup=%v, want non-nil", child, cleanup)
			}
			if opener.calls != 1 {
				t.Fatalf("production deps calls = %d, want 1", opener.calls)
			}
			if opener.withEventPipeline != tt.wantEventPipeline {
				t.Fatalf("withEventPipeline = %v, want %v", opener.withEventPipeline, tt.wantEventPipeline)
			}
			if err := cleanup(context.Background()); err != nil {
				t.Fatalf("cleanup() error = %v", err)
			}
			if opener.cleanupCalls != 1 {
				t.Fatalf("cleanup calls = %d, want 1", opener.cleanupCalls)
			}
		})
	}
}

func TestFactoryProductionDepsOpenerErrorPropagates(t *testing.T) {
	wantErr := errors.New("production deps unavailable")
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: requiredConfig(),
		productionDeps: func(context.Context, bool) (*productionDeps, Cleanup, error) {
			return nil, nil, wantErr
		},
	})

	_, _, err := factory.GatewayHandler(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("GatewayHandler() error = %v, want %v", err, wantErr)
	}
}

func TestOpenProductionDepsPropagatesResourceOpenerError(t *testing.T) {
	wantErr := errors.New("open resources failed")
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: requiredConfig(),
		productionResources: func(context.Context) (productionResources, error) {
			return productionResources{}, wantErr
		},
	})

	_, _, err := factory.openProductionDeps(context.Background(), false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("openProductionDeps() error = %v, want %v", err, wantErr)
	}
}

func TestOpenProductionDepsClosesResourcesWhenSpoolCreationFails(t *testing.T) {
	conn := &closeRecordingConn{}
	cfg := requiredConfig()
	cfg.EventSpoolDir = t.TempDir() + "/spool-file"
	if err := os.WriteFile(cfg.EventSpoolDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: cfg,
		productionResources: func(context.Context) (productionResources, error) {
			return productionResources{
				clickhouseConn: conn,
				runtimeRepo:    &missingRuntimeRepository{},
			}, nil
		},
	})

	_, _, err := factory.openProductionDeps(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "create event spool") {
		t.Fatalf("openProductionDeps() error = %v, want spool creation error", err)
	}
	if !conn.closed {
		t.Fatal("clickhouse connection was not closed after spool creation failure")
	}
}

func TestOpenProductionDepsCleansUpWhenEventPipelineFails(t *testing.T) {
	conn := &closeRecordingConn{}
	cfg := requiredConfig()
	cfg.EventSpoolDir = t.TempDir()
	cfg.EventSpoolMaxBytes = 1 << 20
	cfg.EventSpoolReplayInterval = time.Millisecond
	cfg.EventSpoolBatchSize = 1
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: cfg,
		productionResources: func(context.Context) (productionResources, error) {
			return productionResources{
				clickhouseConn: conn,
				runtimeRepo:    &missingRuntimeRepository{},
			}, nil
		},
	})

	_, _, err := factory.openProductionDeps(context.Background(), true)
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("openProductionDeps() error = %v, want missing active runtime config", err)
	}
	if !conn.closed {
		t.Fatal("clickhouse connection was not closed after event pipeline failure")
	}
}

func TestFactoryLoadCompiledRuntimeRequiresRepository(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{Config: requiredConfig()})

	_, err := factory.loadCompiledRuntime(context.Background(), nil, nil)
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("loadCompiledRuntime() error = %v, want %v", err, ErrActiveRuntimeConfigNotFound)
	}

	_, err = factory.loadCompiledRuntimeFrom(context.Background(), nil, nil, nil)
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("loadCompiledRuntimeFrom() error = %v, want %v", err, ErrActiveRuntimeConfigNotFound)
	}
}

func TestFactoryLoadCompiledRuntimeWrapsCompilerError(t *testing.T) {
	wantErr := errors.New("bad config")
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		Compiler: RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			return CompiledRuntime{}, wantErr
		}),
	})

	_, err := factory.loadCompiledRuntime(context.Background(), nil, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("loadCompiledRuntime() error = %v, want %v", err, wantErr)
	}
}

func TestFactoryLoadCompiledRuntimeWithCompilerFallsBackToDefaultCompiler(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{Config: requiredConfig()})
	repo := &staticRuntimeRepository{cfg: RuntimeConfig{
		Revision: RuntimeRevision{Number: 3},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        http.MethodPost,
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}}

	compiled, err := factory.loadCompiledRuntimeWithCompiler(context.Background(), repo, nil)
	if err != nil {
		t.Fatalf("loadCompiledRuntimeWithCompiler() error = %v", err)
	}
	if compiled.RevisionNumber != 3 || compiled.Gateway == nil || compiled.Gateway.RouteCount() != 1 {
		t.Fatalf("compiled runtime = %+v, want revision 3 with one gateway route", compiled)
	}
}

func TestFactoryCompilerWithUsesConfiguredRuntimeOptions(t *testing.T) {
	cfg := requiredConfig()
	cfg.MaxBodyBytes = 512
	cfg.UpstreamTimeout = 3 * time.Second
	cfg.VerdictTimeout = 2 * time.Second
	writer := events.NewMemoryWriter(events.MemoryWriterOptions{Capacity: 1})
	gate := events.NewHealthGate()
	factory := NewFactoryWithOptions(FactoryOptions{Config: cfg})

	compiler, ok := factory.compilerWith(writer, gate).(runtimecompiler.Compiler)
	if !ok {
		t.Fatalf("compilerWith() type = %T, want gateway runtime compiler", factory.compilerWith(writer, gate))
	}
	if compiler.Options.MaxBodyBytes != cfg.MaxBodyBytes {
		t.Fatalf("MaxBodyBytes = %d, want %d", compiler.Options.MaxBodyBytes, cfg.MaxBodyBytes)
	}
	if compiler.Options.UpstreamTimeout != cfg.UpstreamTimeout {
		t.Fatalf("UpstreamTimeout = %s, want %s", compiler.Options.UpstreamTimeout, cfg.UpstreamTimeout)
	}
	if compiler.Options.VerdictTimeout != cfg.VerdictTimeout {
		t.Fatalf("VerdictTimeout = %s, want %s", compiler.Options.VerdictTimeout, cfg.VerdictTimeout)
	}
	if compiler.Options.EventWriter != writer {
		t.Fatal("EventWriter was not wired into compiler options")
	}
	if compiler.Options.AuditGate != gate {
		t.Fatal("AuditGate was not wired into compiler options")
	}
}

func TestFactoryCompilerWithUsesInjectedCompiler(t *testing.T) {
	injected := RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
		return CompiledRuntime{RevisionNumber: 7}, nil
	})
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:   requiredConfig(),
		Compiler: injected,
	})

	got := factory.compilerWith(events.NewMemoryWriter(events.MemoryWriterOptions{Capacity: 1}), events.NewHealthGate())
	compiled, err := got.CompileRuntime(context.Background(), RuntimeConfig{})
	if err != nil {
		t.Fatalf("CompileRuntime() error = %v", err)
	}
	if compiled.RevisionNumber != 7 {
		t.Fatalf("RevisionNumber = %d, want 7", compiled.RevisionNumber)
	}
}

func TestStartRuntimeWatcherStartsAndStops(t *testing.T) {
	state, err := NewActiveRuntimeState(CompiledRuntime{
		RevisionNumber: 1,
		Gateway:        fakeGatewayRuntime{routeKey: "chat"},
	})
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}
	watcher := NewFactoryWithOptions(FactoryOptions{Config: requiredConfig()}).newRuntimeWatcher(
		&staticRuntimeRepository{cfg: RuntimeConfig{Revision: RuntimeRevision{Number: 1}}},
		RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
			t.Fatal("CompileRuntime() was called for unchanged revision")
			return CompiledRuntime{}, nil
		}),
		state,
		nil,
	)

	cancel, done := startRuntimeWatcher(watcher)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runtime watcher did not stop after cancellation")
	}
}

func TestProductionDepsCleanupStopsBackgroundWorkAndClosesResources(t *testing.T) {
	pipelineDone := make(chan struct{})
	replayDone := make(chan struct{})
	monitorDone := make(chan struct{})
	pipelineCanceled := make(chan struct{})
	replayCanceled := make(chan struct{})
	monitorCanceled := make(chan struct{})
	pool := &closeRecordingPool{}
	conn := &closeRecordingConn{}
	deps := &productionDeps{
		pool:           pool,
		clickhouseConn: conn,
		cancelPipeline: func() { close(pipelineCanceled) },
		pipelineDone:   pipelineDone,
		cancelReplay:   func() { close(replayCanceled) },
		replayDone:     replayDone,
		cancelMonitor:  func() { close(monitorCanceled) },
		monitorDone:    monitorDone,
	}

	done := make(chan error, 1)
	go func() {
		done <- deps.cleanup(context.Background())
	}()

	<-pipelineCanceled
	<-replayCanceled
	<-monitorCanceled
	close(pipelineDone)
	close(replayDone)
	close(monitorDone)

	if err := <-done; err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if !conn.closed {
		t.Fatal("clickhouse connection was not closed")
	}
	if !pool.closed {
		t.Fatal("postgres pool was not closed")
	}
}

func TestProductionDepsCleanupReturnsClickHouseCloseError(t *testing.T) {
	wantErr := errors.New("close failed")
	deps := &productionDeps{clickhouseConn: &closeRecordingConn{err: wantErr}}

	err := deps.cleanup(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("cleanup() error = %v, want %v", err, wantErr)
	}
}

func TestCloseProductionResourcesAllowsEmptyResources(t *testing.T) {
	if err := closeProductionResources(productionResources{}); err != nil {
		t.Fatalf("closeProductionResources() error = %v, want nil", err)
	}
}

func TestCloseProductionResourcesReturnsClickHouseCloseError(t *testing.T) {
	wantErr := errors.New("clickhouse close failed")
	resources := productionResources{clickhouseConn: &closeRecordingConn{err: wantErr}}

	err := closeProductionResources(resources)
	if !errors.Is(err, wantErr) {
		t.Fatalf("closeProductionResources() error = %v, want %v", err, wantErr)
	}
}

func TestProductionDepsStartClickHouseHealthMonitorStartsAndStops(t *testing.T) {
	deps := &productionDeps{eventGate: events.NewHealthGate()}
	probe := &flappingClickHouseProbe{err: errors.New("clickhouse down")}

	deps.startClickHouseHealthMonitor(probe, time.Millisecond)
	if deps.cancelMonitor == nil || deps.monitorDone == nil {
		t.Fatalf("monitor cancel/done were not wired: %+v", deps)
	}
	waitForGate(t, deps.eventGate, false)

	probe.setErr(nil)
	waitForGate(t, deps.eventGate, true)
	deps.cancelMonitor()

	select {
	case <-deps.monitorDone:
	case <-time.After(time.Second):
		t.Fatal("monitor did not stop after cancellation")
	}
}

func TestClickHouseHealthMonitorMarksGateUnhealthyAndRecovers(t *testing.T) {
	gate := events.NewHealthGate()
	probe := &flappingClickHouseProbe{err: errors.New("clickhouse down")}
	cancel, done := observabilityassembly.StartClickHouseHealthMonitor(probe, gate, time.Millisecond)
	defer func() {
		cancel()
		<-done
	}()
	waitForGate(t, gate, false)

	probe.setErr(nil)
	waitForGate(t, gate, true)
}

func TestBuildDurableEventSinkSpoolsFailedPrimaryWrites(t *testing.T) {
	cfg := requiredConfig()
	cfg.EventSpoolDir = t.TempDir()
	cfg.EventSpoolMaxBytes = 1 << 20
	cfg.EventSpoolReplayInterval = time.Millisecond
	cfg.EventSpoolBatchSize = 1

	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: cfg.EventSpoolDir, MaxBytes: cfg.EventSpoolMaxBytes})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink, worker, err := eventassembly.BuildDurableSink(cfg, RuntimeConfig{}, spool, &recordingRuntimeSink{err: errors.New("clickhouse down")}, nil)
	if err != nil {
		t.Fatalf("buildDurableEventSink() error = %v", err)
	}
	if worker == nil {
		t.Fatal("worker is nil")
	}
	if err := sink.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	if stats := spool.Stats(); stats.Depth != 1 {
		t.Fatalf("Depth = %d, want 1", stats.Depth)
	}
}

func TestBuildDurableEventSinkUsesReplayPrimaryWhenProvided(t *testing.T) {
	cfg := requiredConfig()
	cfg.EventSpoolDir = t.TempDir()
	cfg.EventSpoolMaxBytes = 1 << 20
	cfg.EventSpoolReplayInterval = time.Millisecond
	cfg.EventSpoolBatchSize = 1
	primary := &recordingRuntimeSink{err: errors.New("clickhouse down")}
	replayPrimary := &recordingRuntimeSink{}

	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: cfg.EventSpoolDir, MaxBytes: cfg.EventSpoolMaxBytes})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink, worker, err := eventassembly.BuildDurableSink(cfg, RuntimeConfig{}, spool, primary, replayPrimary)
	if err != nil {
		t.Fatalf("buildDurableEventSink() error = %v", err)
	}
	if err := sink.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1", RequestID: "one"}}); err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}
	if err := worker.ReplayOnce(context.Background()); err != nil {
		t.Fatalf("ReplayOnce() error = %v", err)
	}

	if len(primary.batches) != 1 {
		t.Fatalf("primary batches = %d, want only initial failed write", len(primary.batches))
	}
	if len(replayPrimary.batches) != 1 {
		t.Fatalf("replay primary batches = %d, want replayed batch", len(replayPrimary.batches))
	}
	replayed := replayPrimary.batches[0][0]
	if replayed.EventID != "evt-1" || replayed.RetryCount != 1 || replayed.SpoolStatus != "replayed" {
		t.Fatalf("replayed event = %+v, want event replayed through replay primary", replayed)
	}
}

func TestBuildDurableEventSinkReturnsSpoolCreationError(t *testing.T) {
	filePath := t.TempDir() + "/spool-file"
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := requiredConfig()
	cfg.EventSpoolDir = filePath

	_, _, err := eventassembly.BuildDurableSink(cfg, RuntimeConfig{}, nil, &recordingRuntimeSink{}, nil)
	if err == nil {
		t.Fatal("buildDurableEventSink() error = nil, want spool creation error")
	}
	if !strings.Contains(err.Error(), "create event spool") {
		t.Fatalf("buildDurableEventSink() error = %v, want spool creation context", err)
	}
}

func TestBuildDurableEventSinkRejectsUnsupportedRuntimeSink(t *testing.T) {
	cfg := requiredConfig()
	cfg.EventSpoolDir = t.TempDir()
	runtimeCfg := RuntimeConfig{
		Sinks: []SinkConfig{{
			Key:  "events-webhook",
			Kind: "webhook",
		}},
	}

	_, _, err := eventassembly.BuildDurableSink(cfg, runtimeCfg, nil, &recordingRuntimeSink{}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported event sink") {
		t.Fatalf("buildDurableEventSink() error = %v, want unsupported event sink error", err)
	}
}

func TestValidateRuntimeEventSinksAllowsDefaultsAndDisabledUnsupportedSinks(t *testing.T) {
	cfg := RuntimeConfig{
		Sinks: []SinkConfig{
			{Key: "default"},
			{Key: "events", Kind: "clickhouse"},
			{Key: "disabled-webhook", Kind: "webhook", Disabled: true},
		},
	}

	if err := eventassembly.ValidateRuntimeSinks(cfg); err != nil {
		t.Fatalf("validateRuntimeEventSinks() error = %v", err)
	}
}

func TestValidateRuntimeEventSinksRejectsEnabledUnsupportedSinkAfterSupportedSinks(t *testing.T) {
	cfg := RuntimeConfig{
		Sinks: []SinkConfig{
			{Key: "default"},
			{Key: "events", Kind: "clickhouse"},
			{Key: "events-webhook", Kind: "webhook"},
		},
	}

	err := eventassembly.ValidateRuntimeSinks(cfg)
	if err == nil {
		t.Fatal("validateRuntimeEventSinks() error = nil, want unsupported event sink error")
	}
	if !strings.Contains(err.Error(), `unsupported event sink "events-webhook" kind "webhook"`) {
		t.Fatalf("validateRuntimeEventSinks() error = %v, want sink key and kind", err)
	}
}

func TestFactoryNewProductionDepsWiresAdaptersAroundSharedSpool(t *testing.T) {
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	factory := NewFactoryWithOptions(FactoryOptions{Config: requiredConfig()})

	deps := factory.newProductionDeps(productionResources{
		runtimeRepo: &staticRuntimeRepository{cfg: validRuntimeConfig()},
	}, spool)

	if deps == nil || deps.runtimeRepo == nil || deps.notifier == nil || deps.subscriber == nil || deps.eventGate == nil {
		t.Fatalf("production deps were not wired: %+v", deps)
	}
	if deps.eventSpool != spool {
		t.Fatal("event spool was not shared")
	}
	if deps.spoolStatus == nil || deps.retentionMaintainer == nil || deps.auditMaintainer == nil {
		t.Fatalf("production maintainers/status were not wired: %+v", deps)
	}
}

func TestFactoryGatewayHandlerUsesInjectedRepository(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{
		Config: requiredConfig(),
		Repository: &staticRuntimeRepository{cfg: RuntimeConfig{
			Revision: RuntimeRevision{Number: 1},
			Providers: []ProviderConfig{{
				Key:     "openai",
				BaseURL: "https://upstream.example",
			}},
			Routes: []RouteConfig{{
				Key:           "chat",
				Method:        http.MethodPost,
				Path:          "/v1/chat/completions",
				ProviderKey:   "openai",
				ExecutionMode: "inline",
			}},
			PolicyBundles: []policy.Bundle{{
				Key:           "empty",
				Version:       "1",
				Source:        policy.SourceBuiltIn,
				DefaultAction: policy.ActionAllow,
			}},
		}},
	})

	handler, cleanup, err := factory.GatewayHandler(context.Background())
	if err != nil {
		t.Fatalf("GatewayHandler() error = %v", err)
	}
	if handler == nil {
		t.Fatal("GatewayHandler() handler is nil")
	}
	if cleanup == nil {
		t.Fatal("GatewayHandler() cleanup is nil")
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/not-openai", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "kiwiguard_http_requests_total") {
		t.Fatalf("metrics body does not include KiwiGuard HTTP metrics:\n%s", rec.Body.String())
	}
}

func TestFactoryWorkerUsesInjectedRepository(t *testing.T) {
	repo := &staticRuntimeRepository{cfg: RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        http.MethodPost,
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}}
	factory := NewFactoryWithOptions(FactoryOptions{
		Config:     requiredConfig(),
		Repository: repo,
	})

	worker, cleanup, err := factory.Worker(context.Background())
	if err != nil {
		t.Fatalf("Worker() error = %v", err)
	}
	if worker == nil || cleanup == nil {
		t.Fatalf("worker=%v cleanup=%v, want non-nil", worker, cleanup)
	}
}

func TestRuntimeCompilerFuncAndReadinessState(t *testing.T) {
	want := CompiledRuntime{
		RevisionNumber: 9,
		Gateway:        fakeGatewayRuntime{routeKey: "chat"},
	}
	compiler := RuntimeCompilerFunc(func(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
		return want, nil
	})
	got, err := compiler.CompileRuntime(context.Background(), RuntimeConfig{})
	if err != nil {
		t.Fatalf("CompileRuntime() error = %v", err)
	}
	if got.RevisionNumber != want.RevisionNumber {
		t.Fatalf("RevisionNumber = %d, want %d", got.RevisionNumber, want.RevisionNumber)
	}

	state := NewReadinessState()
	state.MarkConfigDegraded("bad_config")
	if state.ConfigReady() {
		t.Fatal("ConfigReady() = true, want false")
	}
	if state.Reason() != "bad_config" {
		t.Fatalf("Reason() = %q, want bad_config", state.Reason())
	}
	state.MarkConfigReady()
	if !state.ConfigReady() || state.Reason() != "" {
		t.Fatalf("ready=%v reason=%q, want ready empty reason", state.ConfigReady(), state.Reason())
	}
}

func requiredConfig() config.Config {
	return config.Config{
		PostgresDSN:        "postgres://example",
		ClickHouseAddr:     "localhost:9000",
		EventQueueCapacity: 8,
		EventBatchSize:     2,
	}
}

type missingRuntimeRepository struct{}

func (r *missingRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return 0, ErrActiveRuntimeConfigNotFound
}

func (r *missingRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return RuntimeConfig{}, ErrActiveRuntimeConfigNotFound
}

type staticRuntimeRepository struct {
	cfg RuntimeConfig
}

func (r *staticRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.cfg.Revision.Number, nil
}

func (r *staticRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	return r.cfg, nil
}

type recordingProductionDepsOpener struct {
	calls             int
	cleanupCalls      int
	withEventPipeline bool
}

func (o *recordingProductionDepsOpener) open(ctx context.Context, withEventPipeline bool) (*productionDeps, Cleanup, error) {
	o.calls++
	o.withEventPipeline = withEventPipeline
	deps := &productionDeps{
		runtimeRepo:    &staticRuntimeRepository{cfg: validRuntimeConfig()},
		subscriber:     nil,
		eventPipeline:  events.NewPipeline(events.PipelineOptions{Capacity: 1, BatchSize: 1}),
		eventGate:      events.NewHealthGate(),
		spoolStatus:    nil,
		clickhouseConn: &closeRecordingConn{},
	}
	return deps, func(context.Context) error {
		o.cleanupCalls++
		return nil
	}, nil
}

func validRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        http.MethodPost,
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}
}

type flappingClickHouseProbe struct {
	mu  sync.RWMutex
	err error
}

func (p *flappingClickHouseProbe) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *flappingClickHouseProbe) currentErr() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.err
}

func (p *flappingClickHouseProbe) Ping(ctx context.Context) error {
	return p.currentErr()
}

func (p *flappingClickHouseProbe) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	if err := p.currentErr(); err != nil {
		return fakeClickHouseRow{err: err}
	}
	if strings.Contains(query, "system.tables") {
		return fakeClickHouseRow{values: []any{uint64(1)}}
	}
	if len(args) >= 2 {
		if columns, ok := args[1].([]string); ok {
			return fakeClickHouseRow{values: []any{uint64(len(columns))}}
		}
	}
	return fakeClickHouseRow{err: errors.New("unexpected clickhouse column query")}
}

type closeRecordingPool struct {
	closed bool
}

func (p *closeRecordingPool) Close() {
	p.closed = true
}

func (p *closeRecordingPool) Ping(ctx context.Context) error {
	return nil
}

type closeRecordingConn struct {
	closed bool
	err    error
}

func (c *closeRecordingConn) Close() error {
	c.closed = true
	return c.err
}

func (c *closeRecordingConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, errors.New("query is not supported")
}

func (c *closeRecordingConn) Contributors() []string {
	return nil
}

func (c *closeRecordingConn) ServerVersion() (*driver.ServerVersion, error) {
	return nil, errors.New("server version is not supported")
}

func (c *closeRecordingConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return errors.New("select is not supported")
}

func (c *closeRecordingConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return fakeClickHouseRow{err: errors.New("query row is not supported")}
}

func (c *closeRecordingConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, errors.New("prepare batch is not supported")
}

func (c *closeRecordingConn) Exec(ctx context.Context, query string, args ...any) error {
	return errors.New("exec is not supported")
}

func (c *closeRecordingConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return errors.New("async insert is not supported")
}

func (c *closeRecordingConn) Ping(ctx context.Context) error {
	return nil
}

func (c *closeRecordingConn) Stats() driver.Stats {
	return driver.Stats{}
}

type recordingRuntimeSink struct {
	batches [][]events.Event
	err     error
}

func (s *recordingRuntimeSink) WriteBatch(ctx context.Context, batch []events.Event) error {
	s.batches = append(s.batches, append([]events.Event(nil), batch...))
	return s.err
}

type fakeClickHouseRow struct {
	values []any
	err    error
}

func (r fakeClickHouseRow) Err() error {
	return r.err
}

func (r fakeClickHouseRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		target := dest[i].(*uint64)
		*target = r.values[i].(uint64)
	}
	return nil
}

func (r fakeClickHouseRow) ScanStruct(dest any) error {
	return errors.New("scan struct is not supported")
}

func waitForGate(t *testing.T, gate *events.HealthGate, healthy bool) {
	t.Helper()
	for range 50 {
		if gate.Healthy() == healthy {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("gate healthy = %v, want %v", gate.Healthy(), healthy)
}
