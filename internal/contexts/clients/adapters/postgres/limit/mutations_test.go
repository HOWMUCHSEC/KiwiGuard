package limit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpsertRoutePolicyWritesPolicyValues(t *testing.T) {
	q := &recordingQueryer{}
	err := UpsertRoutePolicy(context.Background(), q, "revision-id", RoutePolicy{
		RouteID:               "route-id",
		RequestsPerWindow:     100,
		WindowSeconds:         60,
		MaxConcurrentRequests: 8,
		MaxBodyBytes:          4096,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("UpsertRoutePolicy() error = %v", err)
	}

	assertArg(t, q.args, 0, "revision-id")
	assertArg(t, q.args, 1, "route-id")
	assertArg(t, q.args, 2, 100)
	assertArg(t, q.args, 3, 60)
	assertArg(t, q.args, 4, 8)
	assertArg(t, q.args, 5, int64(4096))
	assertArg(t, q.args, 6, true)
}

func TestUpsertClientRouteOverrideWritesOverrideValues(t *testing.T) {
	q := &recordingQueryer{}
	err := UpsertClientRouteOverride(context.Background(), q, "revision-id", ClientRouteOverride{
		ClientID:              "client-id",
		RouteID:               "route-id",
		RequestsPerWindow:     50,
		WindowSeconds:         30,
		MaxConcurrentRequests: 4,
		MaxBodyBytes:          2048,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("UpsertClientRouteOverride() error = %v", err)
	}

	assertArg(t, q.args, 0, "revision-id")
	assertArg(t, q.args, 1, "client-id")
	assertArg(t, q.args, 2, "route-id")
	assertArg(t, q.args, 3, 50)
	assertArg(t, q.args, 4, 30)
	assertArg(t, q.args, 5, 4)
	assertArg(t, q.args, 6, int64(2048))
	assertArg(t, q.args, 7, true)
}

func TestLimitMutationsReturnExecErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(*recordingQueryer) error
	}{
		{
			name: "route policy",
			run: func(q *recordingQueryer) error {
				return UpsertRoutePolicy(context.Background(), q, "revision-id", RoutePolicy{RouteID: "route-id"})
			},
		},
		{
			name: "client override",
			run: func(q *recordingQueryer) error {
				return UpsertClientRouteOverride(context.Background(), q, "revision-id", ClientRouteOverride{
					ClientID: "client-id",
					RouteID:  "route-id",
				})
			},
		},
		{
			name: "delete client override",
			run: func(q *recordingQueryer) error {
				return DeleteClientRouteOverride(context.Background(), q, "revision-id", "client-id", "route-id")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &recordingQueryer{execErr: errors.New("exec failed")}
			if err := tt.run(q); err == nil {
				t.Fatal("mutation error = nil, want exec error")
			}
		})
	}
}

func TestLimitRemapRequiredID(t *testing.T) {
	ids := map[string]string{"old-id": "new-id"}

	got, err := remapRequiredID("old-id", ids, "route")
	if err != nil {
		t.Fatalf("remapRequiredID() error = %v", err)
	}
	if got != "new-id" {
		t.Fatalf("remapRequiredID() = %q, want new-id", got)
	}

	_, err = remapRequiredID("missing-id", ids, "route")
	if err == nil {
		t.Fatal("remapRequiredID(missing) error = nil, want error")
	}
}

func TestLoadRoutePoliciesScansRows(t *testing.T) {
	q := &recordingQueryer{
		rows: newRows([][]any{{
			"policy-id",
			"route-id",
			100,
			60,
			8,
			int64(4096),
			true,
		}}),
	}

	got, err := LoadRoutePolicies(context.Background(), q, "revision-id")
	if err != nil {
		t.Fatalf("LoadRoutePolicies() error = %v", err)
	}

	want := []RoutePolicy{{
		ID:                    "policy-id",
		RouteID:               "route-id",
		RequestsPerWindow:     100,
		WindowSeconds:         60,
		MaxConcurrentRequests: 8,
		MaxBodyBytes:          4096,
		Enabled:               true,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadRoutePolicies() = %#v, want %#v", got, want)
	}
	assertArg(t, q.queryArgs, 0, "revision-id")
}

func TestLoadClientRouteOverridesScansRows(t *testing.T) {
	q := &recordingQueryer{
		rows: newRows([][]any{{
			"override-id",
			"client-id",
			"route-id",
			50,
			30,
			4,
			int64(2048),
			true,
		}}),
	}

	got, err := LoadClientRouteOverrides(context.Background(), q, "revision-id", "client-id")
	if err != nil {
		t.Fatalf("LoadClientRouteOverrides() error = %v", err)
	}

	want := []ClientRouteOverride{{
		ID:                    "override-id",
		ClientID:              "client-id",
		RouteID:               "route-id",
		RequestsPerWindow:     50,
		WindowSeconds:         30,
		MaxConcurrentRequests: 4,
		MaxBodyBytes:          2048,
		Enabled:               true,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadClientRouteOverrides() = %#v, want %#v", got, want)
	}
	assertArg(t, q.queryArgs, 0, "revision-id")
	assertArg(t, q.queryArgs, 1, "client-id")
}

func TestLoadRoutePoliciesReturnsQueryScanAndIterationErrors(t *testing.T) {
	queryErr := errors.New("query failed")
	scanErr := errors.New("scan failed")
	iterateErr := errors.New("iterate failed")

	tests := []struct {
		name string
		q    *recordingQueryer
	}{
		{
			name: "query",
			q:    &recordingQueryer{queryErr: queryErr},
		},
		{
			name: "scan",
			q: &recordingQueryer{
				rows: &fakeRows{
					values:  [][]any{{"policy-id"}},
					index:   -1,
					scanErr: scanErr,
				},
			},
		},
		{
			name: "iterate",
			q: &recordingQueryer{
				rows: &fakeRows{err: iterateErr},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadRoutePolicies(context.Background(), tt.q, "revision-id")
			if err == nil {
				t.Fatal("LoadRoutePolicies() error = nil, want error")
			}
		})
	}
}

func TestLoadClientRouteOverridesReturnsQueryScanAndIterationErrors(t *testing.T) {
	tests := []struct {
		name string
		q    *recordingQueryer
	}{
		{
			name: "query",
			q:    &recordingQueryer{queryErr: errors.New("query failed")},
		},
		{
			name: "scan",
			q: &recordingQueryer{
				rows: &fakeRows{
					values:  [][]any{{"override-id"}},
					index:   -1,
					scanErr: errors.New("scan failed"),
				},
			},
		},
		{
			name: "iterate",
			q: &recordingQueryer{
				rows: &fakeRows{err: errors.New("iterate failed")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadClientRouteOverrides(context.Background(), tt.q, "revision-id", "client-id")
			if err == nil {
				t.Fatal("LoadClientRouteOverrides() error = nil, want error")
			}
		})
	}
}

func TestCloneGatewayRecordsClonesRouteAndClientLimits(t *testing.T) {
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"old-route-id", 100, 60, 8, int64(4096), true}}),
			newRows([][]any{{"client-id", "old-route-id", 50, 30, 4, int64(2048), true}}),
		},
	}

	err := CloneGatewayRecords(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{
		"old-route-id": "new-route-id",
	})
	if err != nil {
		t.Fatalf("CloneGatewayRecords() error = %v", err)
	}

	if len(q.execArgs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(q.execArgs))
	}
	assertArg(t, q.execArgs[0], 0, "draft-revision-id")
	assertArg(t, q.execArgs[0], 1, "new-route-id")
	assertArg(t, q.execArgs[1], 0, "draft-revision-id")
	assertArg(t, q.execArgs[1], 1, "client-id")
	assertArg(t, q.execArgs[1], 2, "new-route-id")
}

func TestCloneRoutePoliciesRequiresMappedRouteID(t *testing.T) {
	q := &recordingQueryer{
		rows: newRows([][]any{{"missing-route-id", 100, 60, 8, int64(4096), true}}),
	}

	err := CloneRoutePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
	if err == nil {
		t.Fatal("CloneRoutePolicies() error = nil, want missing route error")
	}
}

func TestCloneRoutePoliciesReturnsLoadScanIterateAndExecErrors(t *testing.T) {
	tests := []struct {
		name string
		q    *recordingQueryer
	}{
		{
			name: "load",
			q:    &recordingQueryer{queryErr: errors.New("query failed")},
		},
		{
			name: "scan",
			q:    &recordingQueryer{rows: newRows([][]any{{"route-id"}})},
		},
		{
			name: "iterate",
			q:    &recordingQueryer{rows: &fakeRows{index: -1, err: errors.New("iterate failed")}},
		},
		{
			name: "exec",
			q: &recordingQueryer{
				rows:    newRows([][]any{{"old-route-id", 100, 60, 8, int64(4096), true}}),
				execErr: errors.New("exec failed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CloneRoutePolicies(context.Background(), tt.q, "source-revision-id", "draft-revision-id", map[string]string{
				"old-route-id": "new-route-id",
			})
			if err == nil {
				t.Fatal("CloneRoutePolicies() error = nil, want error")
			}
		})
	}
}

func TestCloneClientRouteOverridesReturnsRemapAndExecErrors(t *testing.T) {
	t.Run("missing mapped route", func(t *testing.T) {
		q := &recordingQueryer{
			rows: newRows([][]any{{"client-id", "missing-route-id", 50, 30, 4, int64(2048), true}}),
		}

		err := CloneClientRouteOverrides(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err == nil {
			t.Fatal("CloneClientRouteOverrides() error = nil, want missing route error")
		}
	})

	t.Run("exec", func(t *testing.T) {
		q := &recordingQueryer{
			rows:    newRows([][]any{{"client-id", "old-route-id", 50, 30, 4, int64(2048), true}}),
			execErr: errors.New("exec failed"),
		}

		err := CloneClientRouteOverrides(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{
			"old-route-id": "new-route-id",
		})
		if err == nil {
			t.Fatal("CloneClientRouteOverrides() error = nil, want exec error")
		}
	})
}

func TestDeleteClientRouteOverrideWritesIdentity(t *testing.T) {
	q := &recordingQueryer{}
	err := DeleteClientRouteOverride(context.Background(), q, "revision-id", "client-id", "route-id")
	if err != nil {
		t.Fatalf("DeleteClientRouteOverride() error = %v", err)
	}

	assertArg(t, q.args, 0, "revision-id")
	assertArg(t, q.args, 1, "client-id")
	assertArg(t, q.args, 2, "route-id")
}

func TestRouteIDForRevisionResolvesCurrentRoute(t *testing.T) {
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithValues("route-id"),
		},
	}

	got, err := RouteIDForRevision(context.Background(), q, "revision-id", "route-id")
	if err != nil {
		t.Fatalf("RouteIDForRevision() error = %v", err)
	}
	if got != "route-id" {
		t.Fatalf("RouteIDForRevision() = %q, want route-id", got)
	}
}

func TestRouteIDForRevisionFallsBackToRouteName(t *testing.T) {
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithErr(pgx.ErrNoRows),
			rowWithValues("chat"),
			rowWithValues("cloned-route-id"),
		},
	}

	got, err := RouteIDForRevision(context.Background(), q, "draft-revision-id", "source-route-id")
	if err != nil {
		t.Fatalf("RouteIDForRevision() error = %v", err)
	}
	if got != "cloned-route-id" {
		t.Fatalf("RouteIDForRevision() = %q, want cloned-route-id", got)
	}
}

func TestRouteIDForRevisionKeepsMissingSourceRoute(t *testing.T) {
	q := &recordingQueryer{
		rowsForQueryRow: []pgx.Row{
			rowWithErr(pgx.ErrNoRows),
			rowWithErr(pgx.ErrNoRows),
		},
	}

	got, err := RouteIDForRevision(context.Background(), q, "draft-revision-id", "source-route-id")
	if err != nil {
		t.Fatalf("RouteIDForRevision() error = %v", err)
	}
	if got != "source-route-id" {
		t.Fatalf("RouteIDForRevision() = %q, want source-route-id", got)
	}
}

func TestRouteIDForRevisionReturnsResolutionErrors(t *testing.T) {
	tests := []struct {
		name string
		rows []pgx.Row
	}{
		{
			name: "current route lookup",
			rows: []pgx.Row{rowWithErr(errors.New("current lookup failed"))},
		},
		{
			name: "route name lookup",
			rows: []pgx.Row{
				rowWithErr(pgx.ErrNoRows),
				rowWithErr(errors.New("name lookup failed")),
			},
		},
		{
			name: "cloned route missing",
			rows: []pgx.Row{
				rowWithErr(pgx.ErrNoRows),
				rowWithValues("chat"),
				rowWithErr(pgx.ErrNoRows),
			},
		},
		{
			name: "cloned route lookup",
			rows: []pgx.Row{
				rowWithErr(pgx.ErrNoRows),
				rowWithValues("chat"),
				rowWithErr(errors.New("clone lookup failed")),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &recordingQueryer{rowsForQueryRow: tt.rows}

			_, err := RouteIDForRevision(context.Background(), q, "draft-revision-id", "source-route-id")
			if err == nil {
				t.Fatal("RouteIDForRevision() error = nil, want error")
			}
		})
	}
}

func assertArg[T comparable](t *testing.T, args []any, index int, want T) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("args length = %d, need index %d", len(args), index)
	}
	got, ok := args[index].(T)
	if !ok {
		t.Fatalf("arg[%d] type = %T, want %T", index, args[index], want)
	}
	if got != want {
		t.Fatalf("arg[%d] = %v, want %v", index, got, want)
	}
}

type recordingQueryer struct {
	sql             string
	args            []any
	queryArgs       []any
	execArgs        [][]any
	rows            pgx.Rows
	queryRows       []pgx.Rows
	queryErr        error
	execErr         error
	rowsForQueryRow []pgx.Row
}

func (q *recordingQueryer) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	q.queryArgs = args
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	if len(q.queryRows) > 0 {
		rows := q.queryRows[0]
		q.queryRows = q.queryRows[1:]
		return rows, nil
	}
	if q.rows != nil {
		return q.rows, nil
	}
	panic("unexpected Query")
}

func (q *recordingQueryer) QueryRow(context.Context, string, ...any) pgx.Row {
	if len(q.rowsForQueryRow) == 0 {
		panic("unexpected QueryRow")
	}
	row := q.rowsForQueryRow[0]
	q.rowsForQueryRow = q.rowsForQueryRow[1:]
	return row
}

func (q *recordingQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.sql = sql
	q.args = args
	q.execArgs = append(q.execArgs, args)
	if q.execErr != nil {
		return pgconn.CommandTag{}, q.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakeRows struct {
	values  [][]any
	index   int
	closed  bool
	err     error
	scanErr error
}

func newRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT 1")
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	r.index++
	if r.index >= len(r.values) {
		r.Close()
		return false
	}
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	return assignValues(dest, r.values[r.index])
}

func (r *fakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.values) {
		return nil, errors.New("values without current row")
	}
	return r.values[r.index], nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

type fakeRow struct {
	values []any
	err    error
}

func rowWithValues(values ...any) fakeRow {
	return fakeRow{values: values}
}

func rowWithErr(err error) fakeRow {
	return fakeRow{err: err}
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignValues(dest, r.values)
}

func assignValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan destination count does not match values")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("scan destination must be a non-nil pointer")
		}
		value := reflect.ValueOf(values[i])
		if !value.Type().AssignableTo(target.Elem().Type()) {
			return errors.New("scan value type is not assignable")
		}
		target.Elem().Set(value)
	}
	return nil
}
