package clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/domain"
)

const insertTrafficEventsQuery = `
INSERT INTO kiwiguard_traffic_events (
	event_id,
	event_schema_version,
	event_time,
	request_id,
	correlation_id,
	config_revision_number,
	snapshot_hash,
	route_id,
	provider_id,
	verdict_provider_id,
	policy_bundle_ids,
	http_method,
	api_path,
	endpoint_kind,
	requested_model,
	mapped_model,
	upstream_model,
	gateway_status,
	upstream_status,
	error_type,
	block_reason,
	fallback_triggered,
	risk_level,
	verdict_categories,
	confidence,
	detector_categories,
	matched_span_count,
	streaming_chunk_count,
	redaction_count,
	termination_reason,
	gateway_latency_ms,
	detector_latency_ms,
	verdict_latency_ms,
	upstream_latency_ms,
	queue_delay_ms,
	total_latency_ms,
	sink_status,
	retry_count,
	spool_status,
	dropped,
	drop_reason,
	raw_capture_policy_id,
	retention_policy_id,
	capture_reference,
	route,
	provider,
	direction,
	verdict,
	action,
	detector_hits,
	policy_rule_ids,
	latency_ms,
	streaming_terminated,
	raw_capture_enabled,
	request_hash,
	response_hash,
	request_payload,
	response_payload,
	metadata_json
)`

// Appender appends mapped traffic rows to a storage backend.
type Appender interface {
	Append(context.Context, []TrafficEventRow) error
}

// TrafficEventRow is the ClickHouse row shape for a gateway event.
type TrafficEventRow struct {
	EventID              string
	EventSchemaVersion   string
	EventTime            time.Time
	RequestID            string
	CorrelationID        string
	ConfigRevisionNumber int64
	SnapshotHash         string
	RouteID              string
	ProviderID           string
	VerdictProviderID    string
	PolicyBundleIDs      []string
	HTTPMethod           string
	APIPath              string
	EndpointKind         string
	RequestedModel       string
	MappedModel          string
	UpstreamModel        string
	GatewayStatus        uint16
	UpstreamStatus       uint16
	ErrorType            string
	BlockReason          string
	FallbackTriggered    bool
	RiskLevel            string
	VerdictCategories    []string
	Confidence           float64
	DetectorCategories   []string
	MatchedSpanCount     uint32
	StreamingChunkCount  uint32
	RedactionCount       uint32
	TerminationReason    string
	GatewayLatencyMS     uint32
	DetectorLatencyMS    uint32
	VerdictLatencyMS     uint32
	UpstreamLatencyMS    uint32
	QueueDelayMS         uint32
	TotalLatencyMS       uint32
	SinkStatus           string
	RetryCount           uint16
	SpoolStatus          string
	Dropped              bool
	DropReason           string
	RawCapturePolicyID   string
	RetentionPolicyID    string
	CaptureReference     string
	Route                string
	Provider             string
	Direction            string
	Verdict              string
	Action               string
	DetectorHits         []string
	PolicyRuleIDs        []string
	LatencyMS            uint32
	StreamingTerminated  bool
	RawCaptureEnabled    bool
	RequestHash          string
	ResponseHash         string
	RequestPayload       string
	ResponsePayload      string
	MetadataJSON         string
}

type eventMetadata struct {
	ClientID string `json:"client_id,omitempty"`
}

// Writer writes gateway events to ClickHouse.
type Writer struct {
	appender Appender
}

// MapEvent converts an event into a ClickHouse traffic row.
func MapEvent(event traffic.Event) TrafficEventRow {
	return TrafficEventRow{
		EventID:              event.EventID,
		EventSchemaVersion:   event.SchemaVersion,
		EventTime:            event.EventTime,
		RequestID:            event.RequestID,
		CorrelationID:        event.CorrelationID,
		ConfigRevisionNumber: event.ConfigRevisionNumber,
		SnapshotHash:         event.SnapshotHash,
		RouteID:              event.RouteID,
		ProviderID:           event.ProviderID,
		VerdictProviderID:    event.VerdictProviderID,
		PolicyBundleIDs:      append([]string(nil), event.PolicyBundleIDs...),
		HTTPMethod:           event.HTTPMethod,
		APIPath:              event.APIPath,
		EndpointKind:         event.EndpointKind,
		RequestedModel:       event.RequestedModel,
		MappedModel:          event.MappedModel,
		UpstreamModel:        event.UpstreamModel,
		GatewayStatus:        event.GatewayStatus,
		UpstreamStatus:       event.UpstreamStatus,
		ErrorType:            event.ErrorType,
		BlockReason:          event.BlockReason,
		FallbackTriggered:    event.FallbackTriggered,
		RiskLevel:            event.RiskLevel,
		VerdictCategories:    append([]string(nil), event.Categories...),
		Confidence:           event.Confidence,
		DetectorCategories:   append([]string(nil), event.DetectorCategories...),
		MatchedSpanCount:     event.MatchedSpanCount,
		StreamingChunkCount:  saturatingInt(event.StreamingChunkCount),
		RedactionCount:       saturatingInt(event.RedactionCount),
		TerminationReason:    event.TerminationReason,
		GatewayLatencyMS:     durationMillis(event.GatewayLatency),
		DetectorLatencyMS:    durationMillis(event.DetectorLatency),
		VerdictLatencyMS:     durationMillis(event.VerdictLatency),
		UpstreamLatencyMS:    durationMillis(event.UpstreamLatency),
		QueueDelayMS:         durationMillis(event.QueueDelay),
		TotalLatencyMS:       durationMillis(event.TotalLatency),
		SinkStatus:           event.SinkStatus,
		RetryCount:           event.RetryCount,
		SpoolStatus:          event.SpoolStatus,
		Dropped:              event.Dropped,
		DropReason:           event.DropReason,
		RawCapturePolicyID:   event.RawCapturePolicyID,
		RetentionPolicyID:    event.RetentionPolicyID,
		CaptureReference:     event.CaptureReference,
		Route:                event.RouteID,
		Provider:             event.ProviderID,
		Direction:            string(event.Direction),
		Verdict:              event.Verdict,
		Action:               string(event.Action),
		DetectorHits:         append([]string(nil), event.DetectorCategories...),
		PolicyRuleIDs:        append([]string(nil), event.PolicyRuleIDs...),
		LatencyMS:            durationMillis(event.TotalLatency),
		StreamingTerminated:  event.TerminationReason != "" && event.StreamingChunkCount > 0,
		RawCaptureEnabled:    event.RawCapturePolicyID != "" || event.CaptureReference != "",
		RequestHash:          event.RequestHash,
		ResponseHash:         event.ResponseHash,
		RequestPayload:       event.RequestPayload,
		ResponsePayload:      event.ResponsePayload,
		MetadataJSON:         marshalEventMetadata(event),
	}
}

func marshalEventMetadata(event traffic.Event) string {
	encoded, err := json.Marshal(eventMetadata{ClientID: event.ClientID})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// NewWriter builds a ClickHouse batch writer backed by a native ClickHouse connection.
func NewWriter(conn ch.Conn) *Writer {
	return NewWriterWithAppender(clickhouseAppender{conn: conn})
}

// NewWriterWithAppender builds a writer around a caller-provided ClickHouse batch appender.
func NewWriterWithAppender(appender Appender) *Writer {
	return &Writer{appender: appender}
}

// WriteBatch maps and writes events as one ClickHouse batch.
func (w *Writer) WriteBatch(ctx context.Context, batch []traffic.Event) error {
	if len(batch) == 0 {
		return nil
	}
	if w.appender == nil {
		return fmt.Errorf("write clickhouse event batch: appender is required")
	}

	rows := make([]TrafficEventRow, 0, len(batch))
	for _, event := range batch {
		rows = append(rows, MapEvent(event))
	}
	if err := w.appender.Append(ctx, rows); err != nil {
		return fmt.Errorf("write clickhouse event batch: %w", err)
	}
	return nil
}

type clickhouseAppender struct {
	conn ch.Conn
}

func (a clickhouseAppender) Append(ctx context.Context, rows []TrafficEventRow) error {
	if a.conn == nil {
		return fmt.Errorf("append clickhouse rows: connection is required")
	}

	batch, err := a.conn.PrepareBatch(ctx, insertTrafficEventsQuery)
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}
	defer func() {
		_ = batch.Close()
	}()

	for _, row := range rows {
		if err := batch.Append(
			row.EventID,
			row.EventSchemaVersion,
			row.EventTime,
			row.RequestID,
			row.CorrelationID,
			row.ConfigRevisionNumber,
			row.SnapshotHash,
			row.RouteID,
			row.ProviderID,
			row.VerdictProviderID,
			row.PolicyBundleIDs,
			row.HTTPMethod,
			row.APIPath,
			row.EndpointKind,
			row.RequestedModel,
			row.MappedModel,
			row.UpstreamModel,
			row.GatewayStatus,
			row.UpstreamStatus,
			row.ErrorType,
			row.BlockReason,
			row.FallbackTriggered,
			row.RiskLevel,
			row.VerdictCategories,
			row.Confidence,
			row.DetectorCategories,
			row.MatchedSpanCount,
			row.StreamingChunkCount,
			row.RedactionCount,
			row.TerminationReason,
			row.GatewayLatencyMS,
			row.DetectorLatencyMS,
			row.VerdictLatencyMS,
			row.UpstreamLatencyMS,
			row.QueueDelayMS,
			row.TotalLatencyMS,
			row.SinkStatus,
			row.RetryCount,
			row.SpoolStatus,
			row.Dropped,
			row.DropReason,
			row.RawCapturePolicyID,
			row.RetentionPolicyID,
			row.CaptureReference,
			row.Route,
			row.Provider,
			row.Direction,
			row.Verdict,
			row.Action,
			row.DetectorHits,
			row.PolicyRuleIDs,
			row.LatencyMS,
			row.StreamingTerminated,
			row.RawCaptureEnabled,
			row.RequestHash,
			row.ResponseHash,
			row.RequestPayload,
			row.ResponsePayload,
			row.MetadataJSON,
		); err != nil {
			return fmt.Errorf("append clickhouse row: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

func durationMillis(duration time.Duration) uint32 {
	if duration <= 0 {
		return 0
	}
	millis := duration / time.Millisecond
	if millis > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(millis)
}

func saturatingInt(value int) uint32 {
	if value <= 0 {
		return 0
	}
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value)
}
