package bootstrap

import kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"

var ErrActiveRuntimeConfigNotFound = kgruntime.ErrActiveRuntimeConfigNotFound

type RuntimeConfig = kgruntime.RuntimeConfig
type RuntimeRevision = kgruntime.RuntimeRevision
type RouteConfig = kgruntime.RouteConfig
type ProviderConfig = kgruntime.ProviderConfig
type SinkConfig = kgruntime.SinkConfig
type RetentionPolicyConfig = kgruntime.RetentionPolicyConfig
type CompiledRuntime = kgruntime.CompiledRuntime
type RuntimeCompilerFunc = kgruntime.RuntimeCompilerFunc

func NewReadinessState() *kgruntime.ReadinessState {
	return kgruntime.NewReadinessState()
}

func NewActiveRuntimeState(initial CompiledRuntime) (*kgruntime.ActiveRuntimeState, error) {
	return kgruntime.NewActiveRuntimeState(initial)
}

type fakeGatewayRuntime struct {
	routeKey string
}

func (r fakeGatewayRuntime) RouteCount() int {
	if r.routeKey == "" {
		return 0
	}
	return 1
}
