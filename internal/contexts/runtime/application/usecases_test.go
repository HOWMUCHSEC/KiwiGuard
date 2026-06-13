package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestLoaderLoadCompiledRuntimeCompilesRepositoryConfig(t *testing.T) {
	ctx := t.Context()
	repo := &fakeConfigRepository{
		config: RuntimeConfig{Revision: RuntimeRevision{Number: 7}},
	}
	compiler := &fakeCompiler{
		compiled: CompiledRuntime{RevisionNumber: 7, SnapshotHash: "hash-7"},
	}

	compiled, err := Loader{
		Repository: repo,
		Compiler:   compiler,
	}.LoadCompiledRuntime(ctx)
	if err != nil {
		t.Fatalf("LoadCompiledRuntime() error = %v", err)
	}
	if compiled.RevisionNumber != 7 || compiled.SnapshotHash != "hash-7" {
		t.Fatalf("LoadCompiledRuntime() = %+v, want revision 7 hash hash-7", compiled)
	}
	if repo.loadCalls != 1 {
		t.Fatalf("LoadRuntimeConfig calls = %d, want 1", repo.loadCalls)
	}
	if compiler.lastConfig.Revision.Number != 7 {
		t.Fatalf("compiler config revision = %d, want 7", compiler.lastConfig.Revision.Number)
	}
}

func TestCompilerFuncAdaptsFunction(t *testing.T) {
	compiler := CompilerFunc(func(_ context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
		return CompiledRuntime{RevisionNumber: cfg.Revision.Number}, nil
	})

	compiled, err := compiler.CompileRuntime(t.Context(), RuntimeConfig{
		Revision: RuntimeRevision{Number: 9},
	})
	if err != nil {
		t.Fatalf("CompileRuntime() error = %v", err)
	}
	if compiled.RevisionNumber != 9 {
		t.Fatalf("CompileRuntime() revision = %d, want 9", compiled.RevisionNumber)
	}
}

func TestLoaderLoadCompiledRuntimeRequiresDependencies(t *testing.T) {
	ctx := t.Context()
	_, err := Loader{}.LoadCompiledRuntime(ctx)
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("LoadCompiledRuntime() error = %v, want ErrActiveRuntimeConfigNotFound", err)
	}

	_, err = Loader{Repository: &fakeConfigRepository{}}.LoadCompiledRuntime(ctx)
	if err == nil {
		t.Fatal("LoadCompiledRuntime() error = nil, want compiler dependency error")
	}
}

func TestLoaderLoadCompiledRuntimeWrapsCompileError(t *testing.T) {
	wantErr := errors.New("compile failed")
	_, err := Loader{
		Repository: &fakeConfigRepository{},
		Compiler:   &fakeCompiler{err: wantErr},
	}.LoadCompiledRuntime(t.Context())
	if !errors.Is(err, wantErr) {
		t.Fatalf("LoadCompiledRuntime() error = %v, want %v", err, wantErr)
	}
}

func TestReloaderReloadIfChangedSkipsUnchangedRevision(t *testing.T) {
	repo := &fakeConfigRepository{revision: 3}
	compiler := &fakeCompiler{}
	state := &fakeState{snapshot: CompiledRuntime{RevisionNumber: 3}}

	reloader := Reloader{
		Repository: repo,
		Compiler:   compiler,
		State:      state,
	}
	if err := reloader.ReloadIfChanged(t.Context()); err != nil {
		t.Fatalf("ReloadIfChanged() error = %v", err)
	}
	if repo.loadCalls != 0 {
		t.Fatalf("LoadRuntimeConfig calls = %d, want 0", repo.loadCalls)
	}
	if compiler.calls != 0 {
		t.Fatalf("CompileRuntime calls = %d, want 0", compiler.calls)
	}
	if state.swapCalls != 0 {
		t.Fatalf("Swap calls = %d, want 0", state.swapCalls)
	}
}

func TestReloaderReloadIfChangedRejectsTypedNilState(t *testing.T) {
	var state *fakeState
	err := Reloader{
		Repository: &fakeConfigRepository{},
		Compiler:   &fakeCompiler{},
		State:      state,
	}.ReloadIfChanged(t.Context())
	if err == nil {
		t.Fatal("ReloadIfChanged() error = nil, want state dependency error")
	}
}

func TestReloaderReloadIfChangedMarksDegradedInsteadOfReturningTransientErrors(t *testing.T) {
	tests := []struct {
		name       string
		repo       *fakeConfigRepository
		compiler   *fakeCompiler
		state      *fakeState
		wantReason string
	}{
		{
			name:       "revision error",
			repo:       &fakeConfigRepository{revisionErr: errors.New("revision unavailable")},
			compiler:   &fakeCompiler{},
			state:      &fakeState{},
			wantReason: "active_revision_unavailable",
		},
		{
			name:       "load error",
			repo:       &fakeConfigRepository{revision: 2, loadErr: errors.New("load failed")},
			compiler:   &fakeCompiler{},
			state:      &fakeState{snapshot: CompiledRuntime{RevisionNumber: 1}},
			wantReason: "runtime_config_load_failed",
		},
		{
			name:       "compile error",
			repo:       &fakeConfigRepository{revision: 2},
			compiler:   &fakeCompiler{err: errors.New("compile failed")},
			state:      &fakeState{snapshot: CompiledRuntime{RevisionNumber: 1}},
			wantReason: "runtime_config_compile_failed",
		},
		{
			name:       "swap error",
			repo:       &fakeConfigRepository{revision: 2},
			compiler:   &fakeCompiler{compiled: CompiledRuntime{RevisionNumber: 2}},
			state:      &fakeState{snapshot: CompiledRuntime{RevisionNumber: 1}, swapErr: errors.New("swap failed")},
			wantReason: "runtime_config_swap_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readiness := &fakeReadiness{}
			err := Reloader{
				Repository: tt.repo,
				Compiler:   tt.compiler,
				State:      tt.state,
				Readiness:  readiness,
			}.ReloadIfChanged(t.Context())
			if err != nil {
				t.Fatalf("ReloadIfChanged() error = %v, want nil", err)
			}
			if readiness.reason != tt.wantReason {
				t.Fatalf("readiness reason = %q, want %q", readiness.reason, tt.wantReason)
			}
		})
	}
}

func TestReloaderReloadIfChangedSwapsCompiledRuntime(t *testing.T) {
	repo := &fakeConfigRepository{
		revision: 9,
		config:   RuntimeConfig{Revision: RuntimeRevision{Number: 9}},
	}
	compiler := &fakeCompiler{compiled: CompiledRuntime{RevisionNumber: 9, SnapshotHash: "hash-9"}}
	state := &fakeState{snapshot: CompiledRuntime{RevisionNumber: 8}}
	readiness := &fakeReadiness{reason: "old"}

	reloader := Reloader{
		Repository: repo,
		Compiler:   compiler,
		State:      state,
		Readiness:  readiness,
	}
	if err := reloader.ReloadIfChanged(t.Context()); err != nil {
		t.Fatalf("ReloadIfChanged() error = %v", err)
	}
	if state.snapshot.RevisionNumber != 9 {
		t.Fatalf("state revision = %d, want 9", state.snapshot.RevisionNumber)
	}
	if !readiness.ready || readiness.reason != "" {
		t.Fatalf("readiness = ready %v reason %q, want ready true empty reason", readiness.ready, readiness.reason)
	}
}

func TestRetentionMaintainerApplyLoadsConfigBeforeApplying(t *testing.T) {
	repo := &fakeConfigRepository{config: RuntimeConfig{
		Revision: RuntimeRevision{Number: 4},
		Retention: []RetentionPolicyConfig{
			{Key: "traffic", RetentionDays: 30},
		},
	}}
	applier := &fakeRetentionApplier{}

	maintainer := RetentionMaintainer{
		Repository: repo,
		Applier:    applier,
	}
	if err := maintainer.Apply(t.Context()); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if repo.loadCalls != 1 {
		t.Fatalf("LoadRuntimeConfig calls = %d, want 1", repo.loadCalls)
	}
	if applier.config.Revision.Number != 4 || len(applier.config.Retention) != 1 {
		t.Fatalf("applied config = %+v, want revision 4 with one retention policy", applier.config)
	}
}

func TestRetentionMaintainerApplyRequiresDependencies(t *testing.T) {
	tests := []struct {
		name       string
		maintainer RetentionMaintainer
	}{
		{
			name:       "repository",
			maintainer: RetentionMaintainer{Applier: &fakeRetentionApplier{}},
		},
		{
			name:       "applier",
			maintainer: RetentionMaintainer{Repository: &fakeConfigRepository{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.maintainer.Apply(t.Context()); err == nil {
				t.Fatalf("Apply() error = nil, want dependency error")
			}
		})
	}
}

func TestRetentionMaintainerApplyPropagatesErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	applyErr := errors.New("apply failed")
	tests := []struct {
		name       string
		repo       *fakeConfigRepository
		applier    *fakeRetentionApplier
		wantErr    error
		wantLoaded bool
	}{
		{
			name:       "load",
			repo:       &fakeConfigRepository{loadErr: loadErr},
			applier:    &fakeRetentionApplier{},
			wantErr:    loadErr,
			wantLoaded: true,
		},
		{
			name:       "apply",
			repo:       &fakeConfigRepository{config: RuntimeConfig{Revision: RuntimeRevision{Number: 12}}},
			applier:    &fakeRetentionApplier{err: applyErr},
			wantErr:    applyErr,
			wantLoaded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RetentionMaintainer{
				Repository: tt.repo,
				Applier:    tt.applier,
			}.Apply(t.Context())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Apply() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantLoaded && tt.repo.loadCalls != 1 {
				t.Fatalf("LoadRuntimeConfig calls = %d, want 1", tt.repo.loadCalls)
			}
		})
	}
}

type fakeConfigRepository struct {
	revision    int64
	revisionErr error
	config      RuntimeConfig
	loadErr     error
	loadCalls   int
}

func (r *fakeConfigRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	if r.revisionErr != nil {
		return 0, r.revisionErr
	}
	return r.revision, nil
}

func (r *fakeConfigRepository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	r.loadCalls++
	if r.loadErr != nil {
		return RuntimeConfig{}, r.loadErr
	}
	return r.config, nil
}

type fakeCompiler struct {
	compiled   CompiledRuntime
	err        error
	calls      int
	lastConfig RuntimeConfig
}

func (c *fakeCompiler) CompileRuntime(ctx context.Context, cfg RuntimeConfig) (CompiledRuntime, error) {
	c.calls++
	c.lastConfig = cfg
	if c.err != nil {
		return CompiledRuntime{}, c.err
	}
	return c.compiled, nil
}

type fakeState struct {
	snapshot  CompiledRuntime
	swapErr   error
	swapCalls int
}

func (s *fakeState) Snapshot() CompiledRuntime {
	return s.snapshot
}

func (s *fakeState) Swap(next CompiledRuntime) error {
	s.swapCalls++
	if s.swapErr != nil {
		return s.swapErr
	}
	s.snapshot = next
	return nil
}

type fakeReadiness struct {
	ready  bool
	reason string
}

func (r *fakeReadiness) MarkConfigReady() {
	r.ready = true
	r.reason = ""
}

func (r *fakeReadiness) MarkConfigDegraded(reason string) {
	r.ready = false
	r.reason = reason
}

type fakeRetentionApplier struct {
	config RuntimeConfig
	err    error
}

func (a *fakeRetentionApplier) ApplyRetentionPolicies(ctx context.Context, cfg RuntimeConfig) error {
	a.config = cfg
	if a.err != nil {
		return a.err
	}
	return nil
}
