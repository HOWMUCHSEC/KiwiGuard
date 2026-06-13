package openai

import (
	"testing"

	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

func TestCompiledGatewayRuntimeExposesConfig(t *testing.T) {
	config := Config{
		Routes: []Route{{Key: "chat"}},
	}

	runtime := NewCompiledGatewayRuntime(config)
	if runtime.RouteCount() != 1 {
		t.Fatalf("RouteCount() = %d, want 1", runtime.RouteCount())
	}

	compiled := appruntime.CompiledRuntime{Gateway: runtime}
	got, err := GatewayConfigFromRuntime(compiled)
	if err != nil {
		t.Fatalf("GatewayConfigFromRuntime() error = %v", err)
	}
	if len(got.Routes) != 1 || got.Routes[0].Key != "chat" {
		t.Fatalf("GatewayConfigFromRuntime() = %#v, want chat route", got.Routes)
	}
}

func TestGatewayConfigFromRuntimeRejectsUnknownRuntime(t *testing.T) {
	_, err := GatewayConfigFromRuntime(appruntime.CompiledRuntime{Gateway: routeCounter(1)})
	if err == nil {
		t.Fatal("GatewayConfigFromRuntime() error = nil, want error")
	}
}

func TestNewCompiledServerBuildsHandler(t *testing.T) {
	handler, err := NewCompiledServer(appruntime.CompiledRuntime{
		Gateway: NewCompiledGatewayRuntime(Config{}),
	})
	if err != nil {
		t.Fatalf("NewCompiledServer() error = %v", err)
	}
	if handler == nil {
		t.Fatal("NewCompiledServer() handler = nil")
	}
}

func TestRuntimeConfigProviderHandlesNilAndUnknownState(t *testing.T) {
	if got := (runtimeConfigProvider{}).CurrentConfig(); len(got.Routes) != 0 {
		t.Fatalf("CurrentConfig(nil) routes = %d, want 0", len(got.Routes))
	}

	provider := runtimeConfigProvider{state: staticRuntimeState{
		compiled: appruntime.CompiledRuntime{Gateway: routeCounter(1)},
	}}
	if got := provider.CurrentConfig(); len(got.Routes) != 0 {
		t.Fatalf("CurrentConfig(unknown runtime) routes = %d, want 0", len(got.Routes))
	}
}

func TestNewServerWithRuntimeStateBuildsHandler(t *testing.T) {
	handler := NewServerWithRuntimeState(staticRuntimeState{
		compiled: appruntime.CompiledRuntime{Gateway: NewCompiledGatewayRuntime(Config{})},
	})
	if handler == nil {
		t.Fatal("NewServerWithRuntimeState() handler = nil")
	}
}

type routeCounter int

func (r routeCounter) RouteCount() int {
	return int(r)
}

type staticRuntimeState struct {
	compiled appruntime.CompiledRuntime
}

func (s staticRuntimeState) Snapshot() appruntime.CompiledRuntime {
	return s.compiled
}
