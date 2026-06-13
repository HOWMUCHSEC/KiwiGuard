package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpsertModelMappingDefaultsParameters(t *testing.T) {
	q := &recordingQueryer{}
	err := UpsertModelMapping(context.Background(), q, "revision-id", ModelMapping{
		Name:             "chat",
		SourceModel:      "gpt-4.1",
		TargetProviderID: "provider-id",
		TargetModel:      "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("UpsertModelMapping() error = %v", err)
	}

	if got := string(q.args[5].(json.RawMessage)); got != "{}" {
		t.Fatalf("parameters = %q, want {}", got)
	}
}

func TestUpsertVerdictProviderDefaultsOptionalSettings(t *testing.T) {
	q := &recordingQueryer{}
	err := UpsertVerdictProvider(context.Background(), q, "revision-id", VerdictProvider{
		Name:     "guard",
		Endpoint: "http://guard.local",
	})
	if err != nil {
		t.Fatalf("UpsertVerdictProvider() error = %v", err)
	}

	if got := q.args[2]; got != "http" {
		t.Fatalf("adapter = %v, want http", got)
	}
	if got := q.args[4]; got != 5000 {
		t.Fatalf("timeout millis = %v, want 5000", got)
	}
	if got := string(q.args[6].(json.RawMessage)); got != "{}" {
		t.Fatalf("adapter config = %q, want {}", got)
	}
	if got := string(q.args[8].(json.RawMessage)); got != "{}" {
		t.Fatalf("retry config = %q, want {}", got)
	}
	if got := string(q.args[9].(json.RawMessage)); got != "{}" {
		t.Fatalf("circuit breaker config = %q, want {}", got)
	}
	if got := q.args[10]; got != 16 {
		t.Fatalf("max concurrency = %v, want 16", got)
	}
}

func TestRoutingHelpersPreserveExplicitValues(t *testing.T) {
	raw := json.RawMessage(`{"a":1}`)
	if got := string(defaultJSONObject(raw)); got != `{"a":1}` {
		t.Fatalf("defaultJSONObject(raw) = %q, want original JSON", got)
	}
	if got := durationMillis(1500 * time.Millisecond); got != 1500 {
		t.Fatalf("durationMillis(1500ms) = %d, want 1500", got)
	}
}

func TestRoutingRemapHelpers(t *testing.T) {
	ids := map[string]string{"old-id": "new-id"}

	got, err := remapRequiredID("old-id", ids, "provider")
	if err != nil {
		t.Fatalf("remapRequiredID() error = %v", err)
	}
	if got != "new-id" {
		t.Fatalf("remapRequiredID() = %q, want new-id", got)
	}

	got, err = remapOptionalID("", ids, "optional provider")
	if err != nil {
		t.Fatalf("remapOptionalID(empty) error = %v", err)
	}
	if got != "" {
		t.Fatalf("remapOptionalID(empty) = %q, want empty", got)
	}

	_, err = remapRequiredID("missing-id", ids, "route")
	if err == nil {
		t.Fatal("remapRequiredID(missing) error = nil, want error")
	}
}

func TestLoadRoutingRecordsScansRows(t *testing.T) {
	raw := json.RawMessage(`{"a":1}`)

	tests := []struct {
		name string
		load func(*recordingQueryer) (any, error)
		rows pgx.Rows
		want any
	}{
		{
			name: "routes",
			load: func(q *recordingQueryer) (any, error) {
				return LoadRoutes(context.Background(), q, "revision-id")
			},
			rows: newRows([][]any{{"route-id", "chat", true, 10, "POST", "/v1/chat", "/v1", "openai", "gpt-4.1-mini", "mapping-id", "inline", "allow"}}),
			want: []Route{{
				ID:             "route-id",
				Name:           "chat",
				Enabled:        true,
				Priority:       10,
				Method:         "POST",
				Path:           "/v1/chat",
				PathPrefix:     "/v1",
				Provider:       "openai",
				UpstreamModel:  "gpt-4.1-mini",
				ModelMappingID: "mapping-id",
				ExecutionMode:  "inline",
				FallbackAction: "allow",
			}},
		},
		{
			name: "providers",
			load: func(q *recordingQueryer) (any, error) {
				return LoadProviders(context.Background(), q, "revision-id")
			},
			rows: newRows([][]any{{"provider-id", "openai", "https://api.openai.test", "credential-ref", 2500, "openai", raw, raw, raw, raw}}),
			want: []Provider{{
				ID:                   "provider-id",
				Name:                 "openai",
				BaseURL:              "https://api.openai.test",
				CredentialRef:        "credential-ref",
				Timeout:              2500 * time.Millisecond,
				ProviderType:         "openai",
				Headers:              raw,
				RetryConfig:          raw,
				CircuitBreakerConfig: raw,
				Capabilities:         raw,
			}},
		},
		{
			name: "model mappings",
			load: func(q *recordingQueryer) (any, error) {
				return LoadModelMappings(context.Background(), q, "revision-id")
			},
			rows: newRows([][]any{{"mapping-id", "chat", "gpt-4.1", "provider-id", "gpt-4.1-mini", raw}}),
			want: []ModelMapping{{
				ID:               "mapping-id",
				Name:             "chat",
				SourceModel:      "gpt-4.1",
				TargetProviderID: "provider-id",
				TargetModel:      "gpt-4.1-mini",
				Parameters:       raw,
			}},
		},
		{
			name: "verdict providers",
			load: func(q *recordingQueryer) (any, error) {
				return LoadVerdictProviders(context.Background(), q, "revision-id")
			},
			rows: newRows([][]any{{"verdict-id", "guard", "http", "http://guard.test", 1500, "verdict-ref", raw, "guard-model", raw, raw, 4, true}}),
			want: []VerdictProvider{{
				ID:                   "verdict-id",
				Name:                 "guard",
				Adapter:              "http",
				Endpoint:             "http://guard.test",
				CredentialRef:        "verdict-ref",
				ModelName:            "guard-model",
				Timeout:              1500 * time.Millisecond,
				AdapterConfig:        raw,
				RetryConfig:          raw,
				CircuitBreakerConfig: raw,
				MaxConcurrency:       4,
				Enabled:              true,
			}},
		},
		{
			name: "route verdict bindings",
			load: func(q *recordingQueryer) (any, error) {
				return LoadRouteVerdictProviderBindings(context.Background(), q, "revision-id")
			},
			rows: newRows([][]any{{"binding-id", "route-id", "verdict-id", true, "async-shadow", 20}}),
			want: []RouteVerdictProviderBinding{{
				ID:                "binding-id",
				RouteID:           "route-id",
				VerdictProviderID: "verdict-id",
				Enabled:           true,
				ExecutionMode:     "async-shadow",
				Priority:          20,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &recordingQueryer{rows: tt.rows}
			got, err := tt.load(q)
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("load() = %#v, want %#v", got, tt.want)
			}
			assertArg(t, q.queryArgs, 0, "revision-id")
		})
	}
}

func TestCloneRoutingRecordsClonesRows(t *testing.T) {
	raw := json.RawMessage(`{"a":1}`)
	q := &recordingQueryer{
		queryRows: []pgx.Rows{
			newRows([][]any{{"old-provider-id", "openai", "https://api.openai.test", "credential-ref", 2500, "openai", raw, raw, raw, raw}}),
			newRows([][]any{{"old-mapping-id", "chat", "gpt-4.1", "old-provider-id", "gpt-4.1-mini", raw}}),
			newRows([][]any{{"old-route-id", "chat", "/v1", "openai", "gpt-4.1-mini", "inline", "allow", true, 10, "POST", "/v1/chat", "old-mapping-id"}}),
			newRows([][]any{{"old-verdict-id", "guard", "http", "http://guard.test", 1500, "verdict-ref", raw, "guard-model", raw, raw, 4, true}}),
			newRows([][]any{{"old-route-id", "old-verdict-id", true, "async-shadow", 20}}),
		},
		rowsForQueryRow: []pgx.Row{
			rowWithValues("new-provider-id"),
			rowWithValues("new-mapping-id"),
			rowWithValues("new-route-id"),
			rowWithValues("new-verdict-id"),
		},
	}

	providerIDs, err := CloneProviders(context.Background(), q, "source-revision-id", "draft-revision-id")
	if err != nil {
		t.Fatalf("CloneProviders() error = %v", err)
	}
	mappingIDs, err := CloneModelMappings(context.Background(), q, "source-revision-id", "draft-revision-id", providerIDs)
	if err != nil {
		t.Fatalf("CloneModelMappings() error = %v", err)
	}
	routeIDs, err := CloneRoutes(context.Background(), q, "source-revision-id", "draft-revision-id", mappingIDs)
	if err != nil {
		t.Fatalf("CloneRoutes() error = %v", err)
	}
	verdictProviderIDs, err := CloneVerdictProviders(context.Background(), q, "source-revision-id", "draft-revision-id")
	if err != nil {
		t.Fatalf("CloneVerdictProviders() error = %v", err)
	}
	err = CloneRouteVerdictProviderBindings(context.Background(), q, "source-revision-id", routeIDs, verdictProviderIDs)
	if err != nil {
		t.Fatalf("CloneRouteVerdictProviderBindings() error = %v", err)
	}

	if providerIDs["old-provider-id"] != "new-provider-id" {
		t.Fatalf("provider ID map = %#v", providerIDs)
	}
	if mappingIDs["old-mapping-id"] != "new-mapping-id" {
		t.Fatalf("mapping ID map = %#v", mappingIDs)
	}
	if routeIDs["old-route-id"] != "new-route-id" {
		t.Fatalf("route ID map = %#v", routeIDs)
	}
	if verdictProviderIDs["old-verdict-id"] != "new-verdict-id" {
		t.Fatalf("verdict provider ID map = %#v", verdictProviderIDs)
	}
	if len(q.execArgs) != 1 {
		t.Fatalf("exec count = %d, want 1", len(q.execArgs))
	}
	assertArg(t, q.execArgs[0], 0, "new-route-id")
	assertArg(t, q.execArgs[0], 1, "new-verdict-id")
}

func TestCloneModelMappingsRequiresMappedProviderID(t *testing.T) {
	q := &recordingQueryer{
		rows: newRows([][]any{{"old-mapping-id", "chat", "gpt-4.1", "missing-provider-id", "gpt-4.1-mini", json.RawMessage(`{}`)}}),
	}

	_, err := CloneModelMappings(context.Background(), q, "source-revision-id", "draft-revision-id", map[string]string{})
	if err == nil {
		t.Fatal("CloneModelMappings() error = nil, want missing provider error")
	}
}

func TestLoadRoutesReturnsQueryScanAndIterationErrors(t *testing.T) {
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
					values:  [][]any{{"route-id"}},
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
			_, err := LoadRoutes(context.Background(), tt.q, "revision-id")
			if err == nil {
				t.Fatal("LoadRoutes() error = nil, want error")
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
