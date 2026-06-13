package clickhouse

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

func TestWriterMapsEventToTrafficEventRow(t *testing.T) {
	row := MapEvent(events.Event{
		EventID:              "evt-1",
		SchemaVersion:        "1",
		EventTime:            time.Unix(10, 0).UTC(),
		RequestID:            "req-1",
		CorrelationID:        "corr-1",
		ConfigRevisionNumber: 7,
		SnapshotHash:         "snapshot",
		RouteID:              "route-openai",
		ProviderID:           "provider-openai",
		VerdictProviderID:    "verdict-sec",
		PolicyBundleIDs:      []string{"built-in"},
		HTTPMethod:           "POST",
		APIPath:              "/v1/chat/completions",
		EndpointKind:         "chat_completions",
		RequestedModel:       "gpt-test",
		MappedModel:          "gpt-mapped",
		UpstreamModel:        "gpt-upstream",
		Direction:            events.Direction("output"),
		GatewayLatency:       12 * time.Millisecond,
		DetectorLatency:      13 * time.Millisecond,
		VerdictLatency:       14 * time.Millisecond,
		UpstreamLatency:      15 * time.Millisecond,
		QueueDelay:           16 * time.Millisecond,
		TotalLatency:         17 * time.Millisecond,
		DetectorCategories:   []string{"email"},
		PolicyRuleIDs:        []string{"block-email"},
		RequestPayload:       `{"messages":[{"content":"hello"}]}`,
		ResponsePayload:      `{"choices":[{"message":{"content":"safe"}}]}`,
	})

	if row.EventID != "evt-1" {
		t.Fatalf("EventID = %q, want evt-1", row.EventID)
	}
	if row.EventSchemaVersion != "1" {
		t.Fatalf("EventSchemaVersion = %q, want 1", row.EventSchemaVersion)
	}
	if row.GatewayLatencyMS != 12 {
		t.Fatalf("GatewayLatencyMS = %d, want 12", row.GatewayLatencyMS)
	}
	if row.PolicyBundleIDs[0] != "built-in" {
		t.Fatalf("PolicyBundleIDs = %+v", row.PolicyBundleIDs)
	}
	if row.TotalLatencyMS != 17 {
		t.Fatalf("TotalLatencyMS = %d, want 17", row.TotalLatencyMS)
	}
	if row.Provider != "provider-openai" {
		t.Fatalf("Provider = %q, want provider-openai", row.Provider)
	}
	if row.Direction != "output" {
		t.Fatalf("Direction = %q, want output", row.Direction)
	}
	if strings.Join(row.DetectorHits, ",") != "email" {
		t.Fatalf("DetectorHits = %v, want [email]", row.DetectorHits)
	}
	if strings.Join(row.PolicyRuleIDs, ",") != "block-email" {
		t.Fatalf("PolicyRuleIDs = %v, want [block-email]", row.PolicyRuleIDs)
	}
	if row.RequestPayload != `{"messages":[{"content":"hello"}]}` {
		t.Fatalf("RequestPayload = %q, want raw request payload", row.RequestPayload)
	}
	if row.ResponsePayload != `{"choices":[{"message":{"content":"safe"}}]}` {
		t.Fatalf("ResponsePayload = %q, want raw response payload", row.ResponsePayload)
	}
}

func TestMapEventCopiesSlices(t *testing.T) {
	event := events.Event{
		PolicyBundleIDs:    []string{"policy-1"},
		Categories:         []string{"category-1"},
		DetectorCategories: []string{"detector-1"},
		PolicyRuleIDs:      []string{"rule-1"},
	}

	row := MapEvent(event)
	event.PolicyBundleIDs[0] = "policy-2"
	event.Categories[0] = "category-2"
	event.DetectorCategories[0] = "detector-2"

	if row.PolicyBundleIDs[0] != "policy-1" {
		t.Fatalf("PolicyBundleIDs = %v, want copied policy-1", row.PolicyBundleIDs)
	}
	if row.VerdictCategories[0] != "category-1" {
		t.Fatalf("VerdictCategories = %v, want copied category-1", row.VerdictCategories)
	}
	if row.DetectorCategories[0] != "detector-1" {
		t.Fatalf("DetectorCategories = %v, want copied detector-1", row.DetectorCategories)
	}
	if row.DetectorHits[0] != "detector-1" {
		t.Fatalf("DetectorHits = %v, want copied detector-1", row.DetectorHits)
	}
	if row.PolicyRuleIDs[0] != "rule-1" {
		t.Fatalf("PolicyRuleIDs = %v, want copied rule-1", row.PolicyRuleIDs)
	}
}

func TestWriterSaturatesDurationMilliseconds(t *testing.T) {
	row := MapEvent(events.Event{GatewayLatency: 1<<63 - 1})
	if row.GatewayLatencyMS != 1<<32-1 {
		t.Fatalf("GatewayLatencyMS = %d, want MaxUint32", row.GatewayLatencyMS)
	}
}

func TestDurationMillis(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     uint32
	}{
		{name: "negative", duration: -time.Millisecond, want: 0},
		{name: "zero", duration: 0, want: 0},
		{name: "sub millisecond", duration: time.Microsecond, want: 0},
		{name: "milliseconds", duration: 42 * time.Millisecond, want: 42},
		{name: "saturates", duration: time.Duration(math.MaxUint32+1) * time.Millisecond, want: math.MaxUint32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationMillis(tt.duration)
			if got != tt.want {
				t.Fatalf("durationMillis(%v) = %d, want %d", tt.duration, got, tt.want)
			}
		})
	}
}

func TestMapEventSetsDerivedFields(t *testing.T) {
	row := MapEvent(events.Event{
		RouteID:             "route-openai",
		ProviderID:          "provider-openai",
		TerminationReason:   "policy_block",
		StreamingChunkCount: 3,
		RawCapturePolicyID:  "raw-policy",
		TotalLatency:        25 * time.Millisecond,
	})

	if row.Route != "route-openai" {
		t.Fatalf("Route = %q, want route-openai", row.Route)
	}
	if row.Provider != "provider-openai" {
		t.Fatalf("Provider = %q, want provider-openai", row.Provider)
	}
	if !row.StreamingTerminated {
		t.Fatal("StreamingTerminated = false, want true")
	}
	if !row.RawCaptureEnabled {
		t.Fatal("RawCaptureEnabled = false, want true")
	}
	if row.LatencyMS != 25 {
		t.Fatalf("LatencyMS = %d, want 25", row.LatencyMS)
	}
	if row.MetadataJSON != "{}" {
		t.Fatalf("MetadataJSON = %q, want {}", row.MetadataJSON)
	}
}

func TestMapEventStoresGatewayClientMetadata(t *testing.T) {
	row := MapEvent(events.Event{ClientID: "client-a", BlockReason: "rate_limit_exceeded"})
	if !strings.Contains(row.MetadataJSON, `"client_id":"client-a"`) {
		t.Fatalf("MetadataJSON = %s, want client_id", row.MetadataJSON)
	}
	if strings.Contains(row.MetadataJSON, "kgk_") {
		t.Fatalf("MetadataJSON leaked raw key: %s", row.MetadataJSON)
	}
}

func TestMapEventLeavesDerivedFlagsFalseWithoutInputs(t *testing.T) {
	row := MapEvent(events.Event{
		TerminationReason:   "client_cancel",
		StreamingChunkCount: 0,
	})

	if row.StreamingTerminated {
		t.Fatal("StreamingTerminated = true, want false without streamed chunks")
	}
	if row.RawCaptureEnabled {
		t.Fatal("RawCaptureEnabled = true, want false without raw capture identifiers")
	}
}

func TestSaturatingInt(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  uint32
	}{
		{name: "negative", value: -1, want: 0},
		{name: "zero", value: 0, want: 0},
		{name: "positive", value: 42, want: 42},
	}
	if strconv.IntSize == 64 {
		maxUint32 := uint64(math.MaxUint32)
		tests = append(tests,
			struct {
				name  string
				value int
				want  uint32
			}{name: "max uint32", value: int(maxUint32), want: math.MaxUint32},
			struct {
				name  string
				value int
				want  uint32
			}{name: "above max uint32", value: int(maxUint32 + 1), want: math.MaxUint32},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := saturatingInt(tt.value)
			if got != tt.want {
				t.Fatalf("saturatingInt(%d) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestWriterWritesBatchThroughAppender(t *testing.T) {
	appender := &recordingAppender{}
	writer := NewWriterWithAppender(appender)

	err := writer.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1", EventTime: time.Now().UTC()}})
	if err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}
	if len(appender.rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(appender.rows))
	}
	if appender.rows[0].EventID != "evt-1" {
		t.Fatalf("EventID = %q, want evt-1", appender.rows[0].EventID)
	}
}

func TestWriterEmptyBatchIsNoOp(t *testing.T) {
	appender := &recordingAppender{}
	writer := NewWriterWithAppender(appender)

	err := writer.WriteBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("WriteBatch() error = %v, want nil", err)
	}
	if appender.calls != 0 {
		t.Fatalf("Append calls = %d, want 0", appender.calls)
	}
}

func TestWriterRequiresAppender(t *testing.T) {
	writer := NewWriterWithAppender(nil)

	err := writer.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1"}})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want appender required error")
	}
	if !strings.Contains(err.Error(), "appender is required") {
		t.Fatalf("WriteBatch() error = %v, want appender required error", err)
	}
}

func TestWriterWrapsAppenderError(t *testing.T) {
	wantErr := errors.New("append failed")
	writer := NewWriterWithAppender(&recordingAppender{err: wantErr})

	err := writer.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteBatch() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "write clickhouse event batch") {
		t.Fatalf("WriteBatch() error = %v, want batch context", err)
	}
}

func TestClickHouseAppenderRequiresConnection(t *testing.T) {
	err := clickhouseAppender{}.Append(context.Background(), []TrafficEventRow{{EventID: "evt-1"}})
	if err == nil {
		t.Fatal("Append() error = nil, want connection required error")
	}
	if !strings.Contains(err.Error(), "connection is required") {
		t.Fatalf("Append() error = %v, want connection required error", err)
	}
}

func TestClickHouseAppenderSendsPreparedBatch(t *testing.T) {
	batch := &fakeBatch{}
	conn := &fakeClickHouseConn{batch: batch}
	row := TrafficEventRow{
		EventID:              "evt-1",
		EventSchemaVersion:   "v1",
		EventTime:            time.Unix(10, 0).UTC(),
		RequestID:            "req-1",
		ConfigRevisionNumber: 7,
		PolicyBundleIDs:      []string{"policy"},
		DetectorHits:         []string{"email"},
		PolicyRuleIDs:        []string{"block-email"},
		MetadataJSON:         "{}",
	}

	err := clickhouseAppender{conn: conn}.Append(context.Background(), []TrafficEventRow{row})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if conn.prepareQuery != insertTrafficEventsQuery {
		t.Fatalf("prepareQuery = %q, want insertTrafficEventsQuery", conn.prepareQuery)
	}
	if len(batch.appended) != 1 {
		t.Fatalf("len(appended) = %d, want 1", len(batch.appended))
	}
	if got := batch.appended[0][0]; got != "evt-1" {
		t.Fatalf("appended event_id = %v, want evt-1", got)
	}
	if !batch.sent {
		t.Fatal("batch was not sent")
	}
	if !batch.closed {
		t.Fatal("batch was not closed")
	}
}

func TestClickHouseAppenderWrapsPrepareBatchError(t *testing.T) {
	wantErr := errors.New("prepare failed")
	conn := &fakeClickHouseConn{prepareErr: wantErr}

	err := clickhouseAppender{conn: conn}.Append(context.Background(), []TrafficEventRow{{EventID: "evt-1"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Append() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "prepare clickhouse batch") {
		t.Fatalf("Append() error = %v, want prepare context", err)
	}
}

func TestClickHouseAppenderClosesBatchAfterAppendError(t *testing.T) {
	wantErr := errors.New("append failed")
	batch := &fakeBatch{appendErr: wantErr}
	conn := &fakeClickHouseConn{batch: batch}

	err := clickhouseAppender{conn: conn}.Append(context.Background(), []TrafficEventRow{{EventID: "evt-1"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Append() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "append clickhouse row") {
		t.Fatalf("Append() error = %v, want append row context", err)
	}
	if batch.sent {
		t.Fatal("batch was sent after append error")
	}
	if !batch.closed {
		t.Fatal("batch was not closed after append error")
	}
}

func TestClickHouseAppenderClosesBatchAfterSendError(t *testing.T) {
	wantErr := errors.New("send failed")
	batch := &fakeBatch{sendErr: wantErr}
	conn := &fakeClickHouseConn{batch: batch}

	err := clickhouseAppender{conn: conn}.Append(context.Background(), []TrafficEventRow{{EventID: "evt-1"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Append() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "send clickhouse batch") {
		t.Fatalf("Append() error = %v, want send context", err)
	}
	if !batch.sent {
		t.Fatal("batch send was not attempted")
	}
	if !batch.closed {
		t.Fatal("batch was not closed after send error")
	}
}

type recordingAppender struct {
	calls int
	err   error
	rows  []TrafficEventRow
}

func (a *recordingAppender) Append(ctx context.Context, rows []TrafficEventRow) error {
	a.calls++
	if a.err != nil {
		return a.err
	}
	a.rows = append(a.rows, rows...)
	return nil
}

type fakeClickHouseConn struct {
	batch        driver.Batch
	prepareErr   error
	prepareQuery string
}

func (c *fakeClickHouseConn) Contributors() []string {
	return nil
}

func (c *fakeClickHouseConn) ServerVersion() (*driver.ServerVersion, error) {
	return nil, nil
}

func (c *fakeClickHouseConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return nil
}

func (c *fakeClickHouseConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}

func (c *fakeClickHouseConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return fakeRow{}
}

func (c *fakeClickHouseConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.prepareQuery = query
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}
	return c.batch, nil
}

func (c *fakeClickHouseConn) Exec(ctx context.Context, query string, args ...any) error {
	return nil
}

func (c *fakeClickHouseConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}

func (c *fakeClickHouseConn) Ping(ctx context.Context) error {
	return nil
}

func (c *fakeClickHouseConn) Stats() driver.Stats {
	return driver.Stats{}
}

func (c *fakeClickHouseConn) Close() error {
	return nil
}

type fakeBatch struct {
	appendErr error
	sendErr   error
	appended  [][]any
	sent      bool
	closed    bool
}

func (b *fakeBatch) Abort() error {
	b.closed = true
	return nil
}

func (b *fakeBatch) Append(v ...any) error {
	if b.appendErr != nil {
		return b.appendErr
	}
	b.appended = append(b.appended, append([]any(nil), v...))
	return nil
}

func (b *fakeBatch) AppendStruct(v any) error {
	return nil
}

func (b *fakeBatch) Column(i int) driver.BatchColumn {
	return nil
}

func (b *fakeBatch) Flush() error {
	return nil
}

func (b *fakeBatch) Send() error {
	b.sent = true
	return b.sendErr
}

func (b *fakeBatch) IsSent() bool {
	return b.sent || b.closed
}

func (b *fakeBatch) Rows() int {
	return len(b.appended)
}

func (b *fakeBatch) Columns() []column.Interface {
	return nil
}

func (b *fakeBatch) Close() error {
	b.closed = true
	return nil
}
