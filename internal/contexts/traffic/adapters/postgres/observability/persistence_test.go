package observability

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLoadObservabilityRecords(t *testing.T) {
	raw := json.RawMessage(`{"kind":"clickhouse"}`)
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"sink-id", "clickhouse", "clickhouse", true, raw}}),
			newRows([][]any{{"retention-id", "events-30d", "sink-id", "*", 30}}),
			newRows([][]any{{"raw-id", "redacted-sample", "route-id", "both", true, 0.25, "redacted"}}),
		},
	}

	sinks, err := LoadSinks(context.Background(), q, "revision-id")
	if err != nil {
		t.Fatalf("LoadSinks() error = %v", err)
	}
	retention, err := LoadRetentionPolicies(context.Background(), q, "revision-id")
	if err != nil {
		t.Fatalf("LoadRetentionPolicies() error = %v", err)
	}
	rawCapture, err := LoadRawCapturePolicies(context.Background(), q, "revision-id")
	if err != nil {
		t.Fatalf("LoadRawCapturePolicies() error = %v", err)
	}

	if len(sinks) != 1 || sinks[0].Name != "clickhouse" {
		t.Fatalf("sinks = %#v", sinks)
	}
	if len(retention) != 1 || retention[0].SinkID != "sink-id" {
		t.Fatalf("retention = %#v", retention)
	}
	if len(rawCapture) != 1 || rawCapture[0].RouteID != "route-id" {
		t.Fatalf("raw capture = %#v", rawCapture)
	}
}

func TestCloneObservabilityRecordsClonesRows(t *testing.T) {
	raw := json.RawMessage(`{"kind":"clickhouse"}`)
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"old-sink-id", "clickhouse", "clickhouse", true, raw}}),
			newRows([][]any{{"events-30d", "old-sink-id", "*", 30}}),
			newRows([][]any{{"redacted-sample", "old-route-id", "both", true, 0.25, "redacted"}}),
		},
		rowsForQueryRow: []pgx.Row{rowWithValues("new-sink-id")},
	}

	sinkIDs, err := CloneSinks(context.Background(), q, "source-revision-id", "draft-revision-id")
	if err != nil {
		t.Fatalf("CloneSinks() error = %v", err)
	}
	if sinkIDs["old-sink-id"] != "new-sink-id" {
		t.Fatalf("sink ID map = %#v", sinkIDs)
	}
	if err := CloneRetentionPolicies(context.Background(), q, "source-revision-id", "draft-revision-id", sinkIDs); err != nil {
		t.Fatalf("CloneRetentionPolicies() error = %v", err)
	}
	if err := CloneRawCapturePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{"old-route-id": "new-route-id"}); err != nil {
		t.Fatalf("CloneRawCapturePolicies() error = %v", err)
	}

	if len(q.execArgs) != 2 {
		t.Fatalf("exec count = %d, want 2", len(q.execArgs))
	}
	assertArg(t, q.execArgs[0], 0, "draft-revision-id")
	assertArg(t, q.execArgs[0], 2, "new-sink-id")
	assertArg(t, q.execArgs[1], 0, "draft-revision-id")
	assertArg(t, q.execArgs[1], 2, "new-route-id")
}

func TestCloneRawCapturePoliciesRequiresMappedRouteID(t *testing.T) {
	q := &recordingQueryer{
		rows: newRows([][]any{{"redacted-sample", "missing-route-id", "both", true, 0.25, "redacted"}}),
	}

	err := CloneRawCapturePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
	if err == nil {
		t.Fatal("CloneRawCapturePolicies() error = nil, want missing route error")
	}
}

func TestLoadObservabilityRecordsReturnQueryScanAndIterationErrors(t *testing.T) {
	tests := []struct {
		name string
		load func(context.Context, Queryer, string) error
		q    *recordingQueryer
	}{
		{
			name: "sinks query",
			load: func(ctx context.Context, q Queryer, revisionID string) error {
				_, err := LoadSinks(ctx, q, revisionID)
				return err
			},
			q: &recordingQueryer{queryErr: errors.New("query failed")},
		},
		{
			name: "sinks scan",
			load: func(ctx context.Context, q Queryer, revisionID string) error {
				_, err := LoadSinks(ctx, q, revisionID)
				return err
			},
			q: &recordingQueryer{rows: newRows([][]any{{"sink-id"}})},
		},
		{
			name: "retention iterate",
			load: func(ctx context.Context, q Queryer, revisionID string) error {
				_, err := LoadRetentionPolicies(ctx, q, revisionID)
				return err
			},
			q: &recordingQueryer{rows: &fakeRows{index: -1, err: errors.New("iterate failed")}},
		},
		{
			name: "raw capture scan",
			load: func(ctx context.Context, q Queryer, revisionID string) error {
				_, err := LoadRawCapturePolicies(ctx, q, revisionID)
				return err
			},
			q: &recordingQueryer{rows: &fakeRows{
				values:  [][]any{{"raw-id", "redacted-sample", "route-id", "both", true, 0.25, "redacted"}},
				index:   -1,
				scanErr: errors.New("scan failed"),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.load(context.Background(), tt.q, "revision-id"); err == nil {
				t.Fatal("load error = nil, want error")
			}
		})
	}
}

func TestCloneSinksReturnsLoadScanIterateAndInsertErrors(t *testing.T) {
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
			q:    &recordingQueryer{rows: newRows([][]any{{"sink-id"}})},
		},
		{
			name: "iterate",
			q:    &recordingQueryer{rows: &fakeRows{index: -1, err: errors.New("iterate failed")}},
		},
		{
			name: "insert",
			q: &recordingQueryer{
				rows:            newRows([][]any{{"old-sink-id", "clickhouse", "clickhouse", true, json.RawMessage(`{}`)}}),
				rowsForQueryRow: []pgx.Row{rowWithErr(errors.New("insert failed"))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CloneSinks(context.Background(), tt.q, "source-revision-id", "draft-revision-id"); err == nil {
				t.Fatal("CloneSinks() error = nil, want error")
			}
		})
	}
}

func TestCloneRetentionPoliciesHandlesOptionalSinkAndErrors(t *testing.T) {
	t.Run("empty optional sink inserts empty ID", func(t *testing.T) {
		q := &recordingQueryer{
			rows: newRows([][]any{{"events-30d", "", "*", 30}}),
		}

		err := CloneRetentionPolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err != nil {
			t.Fatalf("CloneRetentionPolicies() error = %v", err)
		}
		assertArg(t, q.execArgs[0], 2, "")
	})

	t.Run("missing mapped sink", func(t *testing.T) {
		q := &recordingQueryer{
			rows: newRows([][]any{{"events-30d", "missing-sink-id", "*", 30}}),
		}

		err := CloneRetentionPolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err == nil {
			t.Fatal("CloneRetentionPolicies() error = nil, want missing sink error")
		}
	})

	t.Run("exec", func(t *testing.T) {
		q := &recordingQueryer{
			rows:    newRows([][]any{{"events-30d", "", "*", 30}}),
			execErr: errors.New("exec failed"),
		}

		err := CloneRetentionPolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err == nil {
			t.Fatal("CloneRetentionPolicies() error = nil, want exec error")
		}
	})
}

func TestCloneRawCapturePoliciesHandlesOptionalRouteAndErrors(t *testing.T) {
	t.Run("empty optional route inserts empty ID", func(t *testing.T) {
		q := &recordingQueryer{
			rows: newRows([][]any{{"redacted-sample", "", "both", true, 0.25, "redacted"}}),
		}

		err := CloneRawCapturePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err != nil {
			t.Fatalf("CloneRawCapturePolicies() error = %v", err)
		}
		assertArg(t, q.execArgs[0], 2, "")
	})

	t.Run("scan", func(t *testing.T) {
		q := &recordingQueryer{rows: newRows([][]any{{"redacted-sample"}})}

		err := CloneRawCapturePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err == nil {
			t.Fatal("CloneRawCapturePolicies() error = nil, want scan error")
		}
	})

	t.Run("iterate", func(t *testing.T) {
		q := &recordingQueryer{rows: &fakeRows{index: -1, err: errors.New("iterate failed")}}

		err := CloneRawCapturePolicies(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
		if err == nil {
			t.Fatal("CloneRawCapturePolicies() error = nil, want iterate error")
		}
	})
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
	execArgs        [][]any
	rows            pgx.Rows
	queryRows       []pgx.Rows
	queryErr        error
	execErr         error
	rowsForQueryRow []pgx.Row
}

func (q *recordingQueryer) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
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

func (q *recordingQueryer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	q.execArgs = append(q.execArgs, args)
	if q.execErr != nil {
		return pgconn.CommandTag{}, q.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

type fakeRows struct {
	values  [][]any
	index   int
	err     error
	scanErr error
}

func newRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func (r *fakeRows) Close() {}

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
	return r.index < len(r.values)
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
