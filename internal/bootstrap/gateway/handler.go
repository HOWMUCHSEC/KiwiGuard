// Package gateway assembles gateway HTTP handlers from compiled runtime state.
package gateway

import (
	"fmt"
	"net/http"

	gatewayhttp "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/http/openai"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
)

// BuildCompiledHandler validates compiled runtime configuration and returns a gateway handler.
func BuildCompiledHandler(compiled kgruntime.CompiledRuntime) (http.Handler, error) {
	if _, err := kgruntime.NewActiveRuntimeState(compiled); err != nil {
		return nil, fmt.Errorf("create active runtime state: %w", err)
	}
	handler, err := gatewayhttp.NewCompiledServer(compiled)
	if err != nil {
		return nil, fmt.Errorf("create gateway handler: %w", err)
	}
	return handler, nil
}

// BuildStateHandler returns a gateway handler backed by a hot-swapped runtime state.
func BuildStateHandler(state *kgruntime.ActiveRuntimeState) http.Handler {
	return gatewayhttp.NewServerWithRuntimeState(state)
}
