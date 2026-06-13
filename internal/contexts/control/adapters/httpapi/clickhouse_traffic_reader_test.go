package httpapi

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func TestTrafficEventWhereClauseMapsRiskLevelFilter(t *testing.T) {
	where, args := trafficEventWhereClause(trafficEventFilter{
		RouteID:    "openai",
		ProviderID: "mock",
		Direction:  "output",
		Status:     403,
		RiskLevel:  "high",
	})

	wantWhere := "WHERE 1 = 1 AND route_id = ? AND provider_id = ? AND direction = ? AND gateway_status = ? AND risk_level = ?"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	wantArgs := []any{"openai", "mock", "output", uint16(403), "high"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestClickHouseTrafficReaderQuerySummaryScansCounts(t *testing.T) {
	querier := &fakeTrafficEventQuerier{
		rows: &fakeTrafficEventRows{
			values: [][]any{{
				uint64(7),
				uint64(2),
				uint64(1),
				uint64(3),
			}},
		},
	}
	reader := clickHouseTrafficReader{querier: querier}

	summary, err := reader.querySummary(context.Background(), "WHERE route_id = ?", []any{"openai"})
	if err != nil {
		t.Fatalf("querySummary returned error: %v", err)
	}

	if summary.Total != 7 || summary.Blocked != 2 || summary.UpstreamErrors != 1 || summary.Fallbacks != 3 {
		t.Fatalf("summary = %#v, want counts 7/2/1/3", summary)
	}
	if !strings.Contains(querier.queries[0], "countIf(action = 'block')") {
		t.Fatalf("summary query missing blocked count: %s", querier.queries[0])
	}
	if !reflect.DeepEqual(querier.args[0], []any{"openai"}) {
		t.Fatalf("summary args = %#v, want route arg", querier.args[0])
	}
	if !querier.rows.closed {
		t.Fatal("summary rows were not closed")
	}
}

func TestClickHouseTrafficReaderQueryItemsScansEventRows(t *testing.T) {
	eventTime := time.Unix(1715000000, 0).UTC()
	querier := &fakeTrafficEventQuerier{
		rows: &fakeTrafficEventRows{
			values: [][]any{{
				eventTime,
				"req-1",
				"corr-1",
				"openai",
				"mock",
				"output",
				"block",
				uint16(403),
				uint16(200),
				"blocked_output",
				"blocked_output",
				"high",
				"gpt-test",
				"gpt-mapped",
				uint32(42),
				uint32(12),
				uint32(8),
				uint32(2),
				true,
				"request-hash",
				"response-hash",
				`{"input":"secret"}`,
				`{"output":"blocked"}`,
				"replayed",
			}},
		},
	}
	reader := clickHouseTrafficReader{querier: querier}

	items, err := reader.queryItems(context.Background(), "WHERE risk_level = ?", []any{"high"}, 25)
	if err != nil {
		t.Fatalf("queryItems returned error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.EventTime != eventTime || item.RequestID != "req-1" || item.Action != "block" {
		t.Fatalf("item identity fields = %#v", item)
	}
	if item.GatewayStatus != 403 || item.UpstreamStatus != 200 {
		t.Fatalf("status fields = %d/%d, want 403/200", item.GatewayStatus, item.UpstreamStatus)
	}
	if item.RiskLevel != "high" {
		t.Fatalf("RiskLevel = %q, want high", item.RiskLevel)
	}
	if item.RequestPayload != `{"input":"secret"}` || item.ResponsePayload != `{"output":"blocked"}` {
		t.Fatalf("payloads = %q/%q", item.RequestPayload, item.ResponsePayload)
	}
	if item.SpoolStatus != "replayed" {
		t.Fatalf("SpoolStatus = %q, want replayed", item.SpoolStatus)
	}
	if !strings.Contains(querier.queries[0], "risk_level") {
		t.Fatalf("items query missing risk_level projection: %s", querier.queries[0])
	}
	if !strings.Contains(querier.queries[0], "spool_status") {
		t.Fatalf("items query missing spool_status projection: %s", querier.queries[0])
	}
	if !strings.Contains(querier.queries[0], "LIMIT 25") {
		t.Fatalf("items query missing limit: %s", querier.queries[0])
	}
	if !reflect.DeepEqual(querier.args[0], []any{"high"}) {
		t.Fatalf("items args = %#v, want risk arg", querier.args[0])
	}
	if !querier.rows.closed {
		t.Fatal("items rows were not closed")
	}
}

func TestClickHouseTrafficReaderQueryItemsWrapsRowErrors(t *testing.T) {
	reader := clickHouseTrafficReader{querier: &fakeTrafficEventQuerier{
		rows: &fakeTrafficEventRows{err: errors.New("boom")},
	}}

	_, err := reader.queryItems(context.Background(), "WHERE 1 = 1", nil, 10)
	if err == nil {
		t.Fatal("queryItems returned nil error, want row iteration error")
	}
	if !strings.Contains(err.Error(), "iterate traffic events: boom") {
		t.Fatalf("error = %v, want wrapped iteration error", err)
	}
}

func TestClickHouseTrafficReaderQuerySummaryWrapsRowErrors(t *testing.T) {
	reader := clickHouseTrafficReader{querier: &fakeTrafficEventQuerier{
		rows: &fakeTrafficEventRows{err: errors.New("summary boom")},
	}}

	_, err := reader.querySummary(context.Background(), "WHERE 1 = 1", nil)
	if err == nil {
		t.Fatal("querySummary returned nil error, want row iteration error")
	}
	if !strings.Contains(err.Error(), "iterate traffic event summary: summary boom") {
		t.Fatalf("error = %v, want wrapped summary iteration error", err)
	}
}

func TestClickHouseTrafficReaderWrapsQueryAndScanErrors(t *testing.T) {
	t.Run("summary query error", func(t *testing.T) {
		reader := clickHouseTrafficReader{querier: &fakeTrafficEventQuerier{err: errors.New("clickhouse down")}}
		_, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{})
		if err == nil || !strings.Contains(err.Error(), "query traffic event summary") {
			t.Fatalf("ListTrafficEvents error = %v, want wrapped summary query error", err)
		}
	})

	t.Run("summary scan error", func(t *testing.T) {
		reader := clickHouseTrafficReader{querier: &fakeTrafficEventQuerier{
			rows: &fakeTrafficEventRows{values: [][]any{{uint64(1)}}},
		}}
		_, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{})
		if err == nil || !strings.Contains(err.Error(), "scan traffic event summary") {
			t.Fatalf("ListTrafficEvents error = %v, want wrapped summary scan error", err)
		}
	})

	t.Run("item scan error", func(t *testing.T) {
		reader := clickHouseTrafficReader{querier: &fakeTrafficEventQuerier{
			rowsQueue: []*fakeTrafficEventRows{
				{values: [][]any{{uint64(0), uint64(0), uint64(0), uint64(0)}}},
				{values: [][]any{{time.Unix(1, 0).UTC()}}},
			},
		}}
		_, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{})
		if err == nil || !strings.Contains(err.Error(), "scan traffic event") {
			t.Fatalf("ListTrafficEvents error = %v, want wrapped item scan error", err)
		}
	})
}

func TestClickHouseTrafficReaderListTrafficEventsClampsLimitAndQueriesSummaryBeforeItems(t *testing.T) {
	querier := &fakeTrafficEventQuerier{
		rowsQueue: []*fakeTrafficEventRows{
			{values: [][]any{{uint64(4), uint64(1), uint64(2), uint64(3)}}},
			{},
		},
	}
	reader := clickHouseTrafficReader{querier: querier}

	response, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{
		RouteID:   "openai",
		RiskLevel: "critical",
		Limit:     maxTrafficEventLimit + 1,
	})
	if err != nil {
		t.Fatalf("ListTrafficEvents returned error: %v", err)
	}

	if response.Summary.Total != 4 || response.Summary.Blocked != 1 {
		t.Fatalf("summary = %#v, want total/block counts", response.Summary)
	}
	if len(response.Items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(response.Items))
	}
	if len(querier.queries) != 2 {
		t.Fatalf("query count = %d, want 2", len(querier.queries))
	}
	if !strings.Contains(querier.queries[0], "count()") {
		t.Fatalf("first query = %s, want summary query", querier.queries[0])
	}
	if !strings.Contains(querier.queries[1], "ORDER BY event_time DESC") {
		t.Fatalf("second query = %s, want item query", querier.queries[1])
	}
	if !strings.Contains(querier.queries[1], "LIMIT 200") {
		t.Fatalf("item query = %s, want clamped limit", querier.queries[1])
	}
	wantArgs := []any{"openai", "critical"}
	if !reflect.DeepEqual(querier.args[0], wantArgs) || !reflect.DeepEqual(querier.args[1], wantArgs) {
		t.Fatalf("query args = %#v, want repeated filter args %#v", querier.args, wantArgs)
	}
}

func TestClickHouseTrafficReaderListTrafficEventsUsesDefaultLimit(t *testing.T) {
	querier := &fakeTrafficEventQuerier{
		rowsQueue: []*fakeTrafficEventRows{
			{values: [][]any{{uint64(0), uint64(0), uint64(0), uint64(0)}}},
			{},
		},
	}
	reader := clickHouseTrafficReader{querier: querier}

	_, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{Limit: 0})
	if err != nil {
		t.Fatalf("ListTrafficEvents returned error: %v", err)
	}
	if len(querier.queries) != 2 {
		t.Fatalf("query count = %d, want 2", len(querier.queries))
	}
	if !strings.Contains(querier.queries[1], "LIMIT 50") {
		t.Fatalf("item query = %s, want default limit", querier.queries[1])
	}
}

func TestNewClickHouseTrafficReaderRequiresConnection(t *testing.T) {
	reader := NewClickHouseTrafficReader(nil)
	if _, err := reader.ListTrafficEvents(context.Background(), trafficEventFilter{}); err == nil {
		t.Fatal("ListTrafficEvents error = nil, want clickhouse connection required")
	}
}

func TestClickHouseTrafficQuerierWrapsDriverRows(t *testing.T) {
	driverRows := &fakeClickHouseDriverRows{
		values: [][]any{{uint64(1), "openai"}},
	}
	conn := &fakeClickHouseTrafficConn{rows: driverRows}
	querier := clickHouseTrafficQuerier{conn: conn}

	rows, err := querier.queryTrafficEvents(context.Background(), "SELECT count(), route_id FROM events WHERE route_id = ?", "openai")
	if err != nil {
		t.Fatalf("queryTrafficEvents() error = %v", err)
	}
	if !rows.Next() {
		t.Fatal("Next() = false, want driver row")
	}
	var total uint64
	var routeID string
	if err := rows.Scan(&total, &routeID); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if total != 1 || routeID != "openai" {
		t.Fatalf("row = %d/%q, want 1/openai", total, routeID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if conn.query != "SELECT count(), route_id FROM events WHERE route_id = ?" {
		t.Fatalf("query = %q, want original query", conn.query)
	}
	if !reflect.DeepEqual(conn.args, []any{"openai"}) {
		t.Fatalf("args = %#v, want openai arg", conn.args)
	}
	if !driverRows.closed {
		t.Fatal("driver rows were not closed")
	}
}

func TestClickHouseTrafficQuerierWrapsDriverQueryError(t *testing.T) {
	wantErr := errors.New("clickhouse query failed")
	querier := clickHouseTrafficQuerier{conn: &fakeClickHouseTrafficConn{err: wantErr}}

	_, err := querier.queryTrafficEvents(context.Background(), "SELECT 1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("queryTrafficEvents() error = %v, want %v", err, wantErr)
	}
}

type fakeTrafficEventQuerier struct {
	rows      *fakeTrafficEventRows
	rowsQueue []*fakeTrafficEventRows
	err       error
	queries   []string
	args      [][]any
}

func (q *fakeTrafficEventQuerier) queryTrafficEvents(ctx context.Context, query string, args ...any) (trafficEventRows, error) {
	q.queries = append(q.queries, query)
	q.args = append(q.args, append([]any(nil), args...))
	if q.err != nil {
		return nil, q.err
	}
	if len(q.rowsQueue) > 0 {
		rows := q.rowsQueue[0]
		q.rowsQueue = q.rowsQueue[1:]
		return rows, nil
	}
	return q.rows, nil
}

type fakeTrafficEventRows struct {
	values [][]any
	err    error
	index  int
	closed bool
}

func (r *fakeTrafficEventRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeTrafficEventRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return errors.New("scan past end")
	}
	row := r.values[r.index]
	r.index++
	if len(dest) != len(row) {
		return errors.New("destination count mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *time.Time:
			*d = row[i].(time.Time)
		case *string:
			*d = row[i].(string)
		case *uint16:
			*d = row[i].(uint16)
		case *uint32:
			*d = row[i].(uint32)
		case *uint64:
			*d = row[i].(uint64)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported destination")
		}
	}
	return nil
}

func (r *fakeTrafficEventRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeTrafficEventRows) Err() error {
	return r.err
}

type fakeClickHouseTrafficConn struct {
	rows  chdriver.Rows
	err   error
	query string
	args  []any
}

func (c *fakeClickHouseTrafficConn) Query(ctx context.Context, query string, args ...any) (chdriver.Rows, error) {
	c.query = query
	c.args = append([]any(nil), args...)
	if c.err != nil {
		return nil, c.err
	}
	return c.rows, nil
}

type fakeClickHouseDriverRows struct {
	values [][]any
	err    error
	index  int
	closed bool
}

func (r *fakeClickHouseDriverRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeClickHouseDriverRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return errors.New("driver scan past end")
	}
	row := r.values[r.index]
	r.index++
	if len(dest) != len(row) {
		return errors.New("driver destination count mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *uint64:
			*d = row[i].(uint64)
		case *string:
			*d = row[i].(string)
		default:
			return errors.New("driver unsupported destination")
		}
	}
	return nil
}

func (r *fakeClickHouseDriverRows) ScanStruct(dest any) error {
	return errors.New("scan struct is not supported")
}

func (r *fakeClickHouseDriverRows) ColumnTypes() []chdriver.ColumnType {
	return nil
}

func (r *fakeClickHouseDriverRows) Totals(dest ...any) error {
	return errors.New("totals is not supported")
}

func (r *fakeClickHouseDriverRows) Columns() []string {
	return nil
}

func (r *fakeClickHouseDriverRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeClickHouseDriverRows) Err() error {
	return r.err
}

func (r *fakeClickHouseDriverRows) HasData() bool {
	return len(r.values) > 0
}
