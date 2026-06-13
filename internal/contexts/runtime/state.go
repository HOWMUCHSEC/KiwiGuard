package runtime

import (
	"errors"
	"sync/atomic"
)

// ActiveRuntimeState owns the immutable gateway runtime snapshot currently served in memory.
type ActiveRuntimeState struct {
	value atomic.Value
}

// NewActiveRuntimeState creates runtime state with an initial compiled runtime.
func NewActiveRuntimeState(initial CompiledRuntime) (*ActiveRuntimeState, error) {
	if err := validateCompiledRuntime(initial); err != nil {
		return nil, err
	}
	state := &ActiveRuntimeState{}
	state.value.Store(initial)
	return state, nil
}

// Snapshot exposes the compiled runtime snapshot currently published to serving code.
func (s *ActiveRuntimeState) Snapshot() CompiledRuntime {
	value := s.value.Load()
	if value == nil {
		return CompiledRuntime{}
	}
	return value.(CompiledRuntime)
}

// Swap publishes a new compiled runtime snapshot after validating its invariants.
func (s *ActiveRuntimeState) Swap(next CompiledRuntime) error {
	if err := validateCompiledRuntime(next); err != nil {
		return err
	}
	s.value.Store(next)
	return nil
}

func validateCompiledRuntime(runtime CompiledRuntime) error {
	if runtime.RevisionNumber <= 0 {
		return errors.New("compiled runtime revision number is required")
	}
	if runtime.Gateway == nil || runtime.Gateway.RouteCount() == 0 {
		return errors.New("compiled runtime gateway routes are required")
	}
	return nil
}

// ReadinessState tracks whether runtime configuration is currently safe to serve.
type ReadinessState struct {
	configReady atomic.Bool
	reason      atomic.Value
}

// NewReadinessState initializes readiness tracking in a healthy state.
func NewReadinessState() *ReadinessState {
	state := &ReadinessState{}
	state.configReady.Store(true)
	state.reason.Store("")
	return state
}

// ConfigReady reports whether runtime configuration is currently considered ready.
func (s *ReadinessState) ConfigReady() bool {
	if s == nil {
		return true
	}
	return s.configReady.Load()
}

// Reason exposes the latest degradation reason recorded for runtime configuration.
func (s *ReadinessState) Reason() string {
	if s == nil {
		return ""
	}
	reason, _ := s.reason.Load().(string)
	return reason
}

// MarkConfigReady clears any degradation reason and marks runtime configuration ready.
func (s *ReadinessState) MarkConfigReady() {
	if s == nil {
		return
	}
	s.configReady.Store(true)
	s.reason.Store("")
}

// MarkConfigDegraded records a degradation reason and marks runtime configuration unready.
func (s *ReadinessState) MarkConfigDegraded(reason string) {
	if s == nil {
		return
	}
	s.configReady.Store(false)
	s.reason.Store(reason)
}
