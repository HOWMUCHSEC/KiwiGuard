package runtime

import "testing"

func TestActiveRuntimeStateReturnsLatestSnapshot(t *testing.T) {
	initial := compiledRuntimeForTest(1, "one")
	state, err := NewActiveRuntimeState(initial)
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}

	next := compiledRuntimeForTest(2, "two")
	if err := state.Swap(next); err != nil {
		t.Fatalf("Swap() error = %v", err)
	}

	if got := state.Snapshot().RevisionNumber; got != 2 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want 2", got)
	}
	if got := state.Snapshot().Gateway.(fakeGatewayRuntime).routeKey; got != "two" {
		t.Fatalf("Snapshot().Gateway route key = %q, want two", got)
	}
}

func TestActiveRuntimeStateRejectsEmptyRuntime(t *testing.T) {
	if _, err := NewActiveRuntimeState(CompiledRuntime{}); err == nil {
		t.Fatal("NewActiveRuntimeState() error = nil, want error")
	}
	state := &ActiveRuntimeState{}
	if got := state.Snapshot().RevisionNumber; got != 0 {
		t.Fatalf("empty Snapshot().RevisionNumber = %d, want 0", got)
	}
}

func TestActiveRuntimeStateRejectsSwapWithoutRoutes(t *testing.T) {
	state, err := NewActiveRuntimeState(compiledRuntimeForTest(1, "one"))
	if err != nil {
		t.Fatalf("NewActiveRuntimeState() error = %v", err)
	}

	err = state.Swap(CompiledRuntime{RevisionNumber: 2})
	if err == nil {
		t.Fatal("Swap() error = nil, want missing routes error")
	}
	if got := state.Snapshot().RevisionNumber; got != 1 {
		t.Fatalf("Snapshot().RevisionNumber = %d, want previous revision 1", got)
	}
}

func TestNilReadinessStateIsReadyAndIgnoresMarks(t *testing.T) {
	var state *ReadinessState

	if !state.ConfigReady() {
		t.Fatal("nil ConfigReady() = false, want true")
	}
	if got := state.Reason(); got != "" {
		t.Fatalf("nil Reason() = %q, want empty", got)
	}

	state.MarkConfigDegraded("ignored")
	state.MarkConfigReady()

	if !state.ConfigReady() {
		t.Fatal("nil ConfigReady() after marks = false, want true")
	}
	if got := state.Reason(); got != "" {
		t.Fatalf("nil Reason() after marks = %q, want empty", got)
	}
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

func compiledRuntimeForTest(revision int64, routeKey string) CompiledRuntime {
	return CompiledRuntime{
		RevisionNumber: revision,
		SnapshotHash:   "hash",
		Gateway:        fakeGatewayRuntime{routeKey: routeKey},
	}
}
