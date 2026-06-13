package openai

import (
	"errors"
	"net/http"

	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
)

type compiledGatewayRuntime struct {
	config Config
}

// NewCompiledGatewayRuntime adapts gateway HTTP configuration to the application runtime contract.
func NewCompiledGatewayRuntime(config Config) appruntime.GatewayRuntime {
	return compiledGatewayRuntime{config: config}
}

func (r compiledGatewayRuntime) RouteCount() int {
	return len(r.config.Routes)
}

func (r compiledGatewayRuntime) Config() Config {
	return r.config
}

// GatewayConfigFromRuntime extracts gateway HTTP configuration from a compiled runtime.
func GatewayConfigFromRuntime(compiled appruntime.CompiledRuntime) (Config, error) {
	runtime, ok := compiled.Gateway.(interface {
		Config() Config
	})
	if !ok {
		return Config{}, errors.New("compiled runtime gateway config is unavailable")
	}
	return runtime.Config(), nil
}

// NewCompiledServer builds a gateway HTTP handler from an immutable compiled runtime snapshot.
func NewCompiledServer(compiled appruntime.CompiledRuntime) (http.Handler, error) {
	config, err := GatewayConfigFromRuntime(compiled)
	if err != nil {
		return nil, err
	}
	return NewServer(config), nil
}

// RuntimeState exposes the active compiled runtime snapshot.
type RuntimeState interface {
	Snapshot() appruntime.CompiledRuntime
}

// NewServerWithRuntimeState builds a gateway HTTP handler that reads from hot-swappable runtime state.
func NewServerWithRuntimeState(state RuntimeState) http.Handler {
	return NewServerWithProvider(runtimeConfigProvider{state: state})
}

type runtimeConfigProvider struct {
	state RuntimeState
}

func (p runtimeConfigProvider) CurrentConfig() Config {
	if p.state == nil {
		return Config{}
	}
	config, _ := GatewayConfigFromRuntime(p.state.Snapshot())
	return config
}
