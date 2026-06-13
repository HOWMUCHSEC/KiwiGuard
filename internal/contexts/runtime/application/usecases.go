package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Loader compiles the active runtime configuration.
type Loader struct {
	Repository RuntimeConfigLoader
	Compiler   Compiler
}

// LoadCompiledRuntime hydrates the active runtime aggregate and compiles it into a publishable snapshot.
func (l Loader) LoadCompiledRuntime(ctx context.Context) (CompiledRuntime, error) {
	if isNil(l.Repository) {
		return CompiledRuntime{}, ErrActiveRuntimeConfigNotFound
	}
	if isNil(l.Compiler) {
		return CompiledRuntime{}, errors.New("runtime compiler is required")
	}
	cfg, err := l.Repository.LoadRuntimeConfig(ctx)
	if err != nil {
		return CompiledRuntime{}, err
	}
	compiled, err := l.Compiler.CompileRuntime(ctx, cfg)
	if err != nil {
		return CompiledRuntime{}, fmt.Errorf("compile active runtime: %w", err)
	}
	return compiled, nil
}

// Reloader refreshes active runtime state when the active revision changes.
type Reloader struct {
	Repository ConfigRepository
	Compiler   Compiler
	State      State
	Readiness  Readiness
}

// ReloadIfChanged reloads runtime state when storage reports a newer revision.
func (r Reloader) ReloadIfChanged(ctx context.Context) error {
	if isNil(r.Repository) {
		return errors.New("reload runtime config: repository is required")
	}
	if isNil(r.Compiler) {
		return errors.New("reload runtime config: compiler is required")
	}
	if isNil(r.State) {
		return errors.New("reload runtime config: state is required")
	}

	revision, err := r.Repository.ActiveRevisionNumber(ctx)
	if err != nil {
		r.markDegraded("active_revision_unavailable")
		return nil
	}
	if revision == r.State.Snapshot().RevisionNumber {
		return nil
	}

	cfg, err := r.Repository.LoadRuntimeConfig(ctx)
	if err != nil {
		r.markDegraded("runtime_config_load_failed")
		return nil
	}
	compiled, err := r.Compiler.CompileRuntime(ctx, cfg)
	if err != nil {
		r.markDegraded("runtime_config_compile_failed")
		return nil
	}
	if err := r.State.Swap(compiled); err != nil {
		r.markDegraded("runtime_config_swap_failed")
		return nil
	}
	if !isNil(r.Readiness) {
		r.Readiness.MarkConfigReady()
	}
	return nil
}

func (r Reloader) markDegraded(reason string) {
	if !isNil(r.Readiness) {
		r.Readiness.MarkConfigDegraded(reason)
	}
}

// RetentionMaintainer rehydrates active runtime configuration before retention work runs.
type RetentionMaintainer struct {
	Repository RuntimeConfigLoader
	Applier    RetentionApplier
}

// Apply applies retention policies for the current runtime config.
func (m RetentionMaintainer) Apply(ctx context.Context) error {
	if isNil(m.Repository) {
		return errors.New("apply retention policies: repository is required")
	}
	if isNil(m.Applier) {
		return errors.New("apply retention policies: applier is required")
	}
	cfg, err := m.Repository.LoadRuntimeConfig(ctx)
	if err != nil {
		return err
	}
	return m.Applier.ApplyRetentionPolicies(ctx, cfg)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflectValue.IsNil()
	default:
		return false
	}
}
