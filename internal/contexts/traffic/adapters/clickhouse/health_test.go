package clickhouse

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func TestCheckSchemaRequiresTrafficEventsTable(t *testing.T) {
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(0)}},
		},
	}

	err := CheckSchema(context.Background(), probe)
	if err == nil {
		t.Fatal("CheckSchema() error = nil, want missing table error")
	}
	if !strings.Contains(err.Error(), "kiwiguard_traffic_events table is required") {
		t.Fatalf("CheckSchema() error = %v, want missing table context", err)
	}
}

func TestCheckSchemaRequiresProbe(t *testing.T) {
	err := CheckSchema(context.Background(), nil)
	if err == nil {
		t.Fatal("CheckSchema() error = nil, want missing probe error")
	}
	if !strings.Contains(err.Error(), "clickhouse probe is required") {
		t.Fatalf("CheckSchema() error = %v, want missing probe context", err)
	}
}

func TestCheckSchemaWrapsTableQueryFailure(t *testing.T) {
	wantErr := errors.New("system.tables unavailable")
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{err: wantErr},
		},
	}

	err := CheckSchema(context.Background(), probe)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CheckSchema() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "query clickhouse traffic event table") {
		t.Fatalf("CheckSchema() error = %v, want table query context", err)
	}
}

func TestCheckSchemaRequiresInsertColumns(t *testing.T) {
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(1)}},
			fakeRow{values: []any{uint64(len(requiredTrafficEventColumns) - 1)}},
		},
	}

	err := CheckSchema(context.Background(), probe)
	if err == nil {
		t.Fatal("CheckSchema() error = nil, want missing columns error")
	}
	if !strings.Contains(err.Error(), "kiwiguard_traffic_events columns are invalid") {
		t.Fatalf("CheckSchema() error = %v, want invalid columns context", err)
	}
}

func TestCheckSchemaWrapsColumnQueryFailure(t *testing.T) {
	wantErr := errors.New("system.columns unavailable")
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(1)}},
			fakeRow{err: wantErr},
		},
	}

	err := CheckSchema(context.Background(), probe)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CheckSchema() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "query clickhouse traffic event columns") {
		t.Fatalf("CheckSchema() error = %v, want column query context", err)
	}
}

func TestRequiredTrafficEventColumnsCoverObservabilityFields(t *testing.T) {
	want := map[string]string{
		"provider":             "String",
		"direction":            "LowCardinality(String)",
		"detector_hits":        "Array(String)",
		"policy_rule_ids":      "Array(String)",
		"latency_ms":           "UInt32",
		"streaming_terminated": "Bool",
		"raw_capture_enabled":  "Bool",
		"request_payload":      "String",
		"response_payload":     "String",
	}

	for name, columnType := range want {
		got, ok := requiredTrafficEventColumns[name]
		if !ok {
			t.Fatalf("requiredTrafficEventColumns[%q] is missing", name)
		}
		if got != columnType {
			t.Fatalf("requiredTrafficEventColumns[%q] = %q, want %q", name, got, columnType)
		}
	}
}

func TestRequiredColumnKeysIncludeNamesAndTypes(t *testing.T) {
	keys := requiredColumnKeys()
	if len(keys) != len(requiredTrafficEventColumns) {
		t.Fatalf("len(requiredColumnKeys()) = %d, want %d", len(keys), len(requiredTrafficEventColumns))
	}

	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		seen[key] = struct{}{}
	}
	for name, columnType := range requiredTrafficEventColumns {
		key := name + ":" + columnType
		if _, ok := seen[key]; !ok {
			t.Fatalf("requiredColumnKeys() missing %q", key)
		}
	}
}

func TestCheckSchemaAcceptsRequiredTableAndColumns(t *testing.T) {
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(1)}},
			fakeRow{values: []any{uint64(len(requiredTrafficEventColumns))}},
		},
	}

	if err := CheckSchema(context.Background(), probe); err != nil {
		t.Fatalf("CheckSchema() error = %v", err)
	}
}

func TestProbeHealthRequiresProbe(t *testing.T) {
	err := ProbeHealth(context.Background(), nil)
	if err == nil {
		t.Fatal("ProbeHealth() error = nil, want missing probe error")
	}
	if !strings.Contains(err.Error(), "clickhouse probe is required") {
		t.Fatalf("ProbeHealth() error = %v, want missing probe context", err)
	}
}

func TestProbeHealthReportsPingFailure(t *testing.T) {
	wantErr := errors.New("dial tcp refused")
	probe := &fakeProbe{pingErr: wantErr}

	err := ProbeHealth(context.Background(), probe)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ProbeHealth() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "ping clickhouse") {
		t.Fatalf("ProbeHealth() error = %v, want ping context", err)
	}
}

func TestProbeHealthWrapsSchemaFailure(t *testing.T) {
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(0)}},
		},
	}

	err := ProbeHealth(context.Background(), probe)
	if err == nil {
		t.Fatal("ProbeHealth() error = nil, want schema error")
	}
	if !strings.Contains(err.Error(), "check clickhouse schema") {
		t.Fatalf("ProbeHealth() error = %v, want schema context", err)
	}
	if !strings.Contains(err.Error(), "kiwiguard_traffic_events table is required") {
		t.Fatalf("ProbeHealth() error = %v, want missing table context", err)
	}
	if probe.pings != 1 {
		t.Fatalf("pings = %d, want 1", probe.pings)
	}
}

func TestProbeHealthChecksSchemaAfterPing(t *testing.T) {
	probe := &fakeProbe{
		rows: []driver.Row{
			fakeRow{values: []any{uint64(1)}},
			fakeRow{values: []any{uint64(len(requiredTrafficEventColumns))}},
		},
	}

	if err := ProbeHealth(context.Background(), probe); err != nil {
		t.Fatalf("ProbeHealth() error = %v", err)
	}
	if probe.pings != 1 {
		t.Fatalf("pings = %d, want 1", probe.pings)
	}
	if probe.queries != 2 {
		t.Fatalf("queries = %d, want 2", probe.queries)
	}
}

func TestOpenAppliesConnectionOptions(t *testing.T) {
	tests := []struct {
		name         string
		options      Options
		wantDatabase string
		wantUsername string
		wantPassword string
	}{
		{
			name:         "defaults database and username",
			options:      Options{Addr: "127.0.0.1:9000"},
			wantDatabase: "kiwiguard",
			wantUsername: "default",
		},
		{
			name: "preserves explicit auth",
			options: Options{
				Addr:     "127.0.0.1:9000",
				Database: "analytics",
				Username: "gateway",
				Password: "secret",
			},
			wantDatabase: "analytics",
			wantUsername: "gateway",
			wantPassword: "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := Open(tt.options)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			t.Cleanup(func() {
				if err := conn.Close(); err != nil {
					t.Fatalf("Close() error = %v", err)
				}
			})

			options := clickHouseOptionsValue(t, conn)
			if got := options.FieldByName("Addr").Index(0).String(); got != tt.options.Addr {
				t.Fatalf("Addr[0] = %q, want %q", got, tt.options.Addr)
			}
			auth := options.FieldByName("Auth")
			if got := auth.FieldByName("Database").String(); got != tt.wantDatabase {
				t.Fatalf("Auth.Database = %q, want %q", got, tt.wantDatabase)
			}
			if got := auth.FieldByName("Username").String(); got != tt.wantUsername {
				t.Fatalf("Auth.Username = %q, want %q", got, tt.wantUsername)
			}
			if got := auth.FieldByName("Password").String(); got != tt.wantPassword {
				t.Fatalf("Auth.Password = %q, want %q", got, tt.wantPassword)
			}
			if got := time.Duration(options.FieldByName("DialTimeout").Int()); got != 5*time.Second {
				t.Fatalf("DialTimeout = %v, want 5s", got)
			}
		})
	}
}

func TestOpenRequiresAddress(t *testing.T) {
	_, err := Open(Options{})
	if err == nil {
		t.Fatal("Open() error = nil, want missing address error")
	}
}

func clickHouseOptionsValue(t *testing.T, conn any) reflect.Value {
	t.Helper()

	value := reflect.ValueOf(conn)
	if value.Kind() != reflect.Pointer {
		t.Fatalf("Open() returned %T, want pointer-backed connection", conn)
	}
	options := value.Elem().FieldByName("opt")
	if !options.IsValid() || options.IsNil() {
		t.Fatalf("Open() returned %T without opt field", conn)
	}
	return options.Elem()
}

type fakeProbe struct {
	pingErr error
	pings   int
	queries int
	rows    []driver.Row
}

func (p *fakeProbe) Ping(ctx context.Context) error {
	p.pings++
	return p.pingErr
}

func (p *fakeProbe) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	p.queries++
	if len(p.rows) == 0 {
		return fakeRow{err: errors.New("unexpected query")}
	}
	row := p.rows[0]
	p.rows = p.rows[1:]
	return row
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Err() error {
	return r.err
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *uint64:
			value, ok := r.values[i].(uint64)
			if !ok {
				return errors.New("fake row value is not uint64")
			}
			*target = value
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

func (r fakeRow) ScanStruct(dest any) error {
	return errors.New("ScanStruct is not supported")
}
