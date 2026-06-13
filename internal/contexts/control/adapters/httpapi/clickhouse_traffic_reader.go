package httpapi

import (
	"context"
	"fmt"
	"strings"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type clickHouseConn interface {
	Query(context.Context, string, ...any) (chdriver.Rows, error)
}

type trafficEventQuerier interface {
	queryTrafficEvents(context.Context, string, ...any) (trafficEventRows, error)
}

type trafficEventRows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}

type clickHouseTrafficReader struct {
	querier trafficEventQuerier
}

type clickHouseTrafficQuerier struct {
	conn clickHouseConn
}

type clickHouseRows struct {
	rows chdriver.Rows
}

// NewClickHouseTrafficReader builds the control-plane traffic reader backed by ClickHouse queries.
func NewClickHouseTrafficReader(conn clickHouseConn) TrafficReader {
	return clickHouseTrafficReader{querier: clickHouseTrafficQuerier{conn: conn}}
}

func (r clickHouseTrafficReader) ListTrafficEvents(ctx context.Context, filter trafficEventFilter) (trafficEventsResponse, error) {
	if filter.Limit <= 0 {
		filter.Limit = defaultTrafficEventLimit
	}
	if filter.Limit > maxTrafficEventLimit {
		filter.Limit = maxTrafficEventLimit
	}

	where, args := trafficEventWhereClause(filter)
	summary, err := r.querySummary(ctx, where, args)
	if err != nil {
		return trafficEventsResponse{}, err
	}
	items, err := r.queryItems(ctx, where, args, filter.Limit)
	if err != nil {
		return trafficEventsResponse{}, err
	}
	return trafficEventsResponse{Items: items, Summary: summary}, nil
}

func (r clickHouseTrafficReader) SummarizeTrafficEvents(ctx context.Context, filter trafficEventFilter) (trafficEventsSummaryDTO, error) {
	where, args := trafficEventWhereClause(filter)
	return r.querySummary(ctx, where, args)
}

func (r clickHouseTrafficReader) querySummary(ctx context.Context, where string, args []any) (trafficEventsSummaryDTO, error) {
	rows, err := r.querier.queryTrafficEvents(ctx, `
		SELECT
			count(),
			countIf(action = 'block'),
			countIf(error_type = 'upstream_error' OR upstream_status >= 500),
			countIf(fallback_triggered)
		FROM kiwiguard_traffic_events
		`+where, args...)
	if err != nil {
		return trafficEventsSummaryDTO{}, fmt.Errorf("query traffic event summary: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var summary trafficEventsSummaryDTO
	if rows.Next() {
		if err := rows.Scan(&summary.Total, &summary.Blocked, &summary.UpstreamErrors, &summary.Fallbacks); err != nil {
			return trafficEventsSummaryDTO{}, fmt.Errorf("scan traffic event summary: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return trafficEventsSummaryDTO{}, fmt.Errorf("iterate traffic event summary: %w", err)
	}
	return summary, nil
}

func (r clickHouseTrafficReader) queryItems(ctx context.Context, where string, args []any, limit int) ([]trafficEventDTO, error) {
	queryArgs := append([]any(nil), args...)
	rows, err := r.querier.queryTrafficEvents(ctx, fmt.Sprintf(`
		SELECT
			event_time,
			request_id,
			correlation_id,
			route_id,
			provider_id,
			direction,
			action,
			gateway_status,
			upstream_status,
			error_type,
			block_reason,
			risk_level,
			requested_model,
			mapped_model,
			total_latency_ms,
			verdict_latency_ms,
			detector_latency_ms,
			matched_span_count,
			fallback_triggered,
			request_hash,
			response_hash,
			request_payload,
			response_payload,
			spool_status
		FROM kiwiguard_traffic_events
		%s
		ORDER BY event_time DESC
		LIMIT %d`, where, limit), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query traffic events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	items := []trafficEventDTO{}
	for rows.Next() {
		var item trafficEventDTO
		if err := rows.Scan(
			&item.EventTime,
			&item.RequestID,
			&item.CorrelationID,
			&item.RouteID,
			&item.ProviderID,
			&item.Direction,
			&item.Action,
			&item.GatewayStatus,
			&item.UpstreamStatus,
			&item.ErrorType,
			&item.BlockReason,
			&item.RiskLevel,
			&item.RequestedModel,
			&item.MappedModel,
			&item.LatencyMS,
			&item.VerdictLatencyMS,
			&item.DetectorLatencyMS,
			&item.MatchedSpanCount,
			&item.FallbackTriggered,
			&item.RequestHash,
			&item.ResponseHash,
			&item.RequestPayload,
			&item.ResponsePayload,
			&item.SpoolStatus,
		); err != nil {
			return nil, fmt.Errorf("scan traffic event: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate traffic events: %w", err)
	}
	return items, nil
}

func trafficEventWhereClause(filter trafficEventFilter) (string, []any) {
	conditions := []string{"1 = 1"}
	args := []any{}
	if filter.RouteID != "" {
		conditions = append(conditions, "route_id = ?")
		args = append(args, filter.RouteID)
	}
	if filter.ProviderID != "" {
		conditions = append(conditions, "provider_id = ?")
		args = append(args, filter.ProviderID)
	}
	if filter.Direction != "" {
		conditions = append(conditions, "direction = ?")
		args = append(args, filter.Direction)
	}
	if filter.Status != 0 {
		conditions = append(conditions, "gateway_status = ?")
		args = append(args, filter.Status)
	}
	if filter.RiskLevel != "" {
		conditions = append(conditions, "risk_level = ?")
		args = append(args, filter.RiskLevel)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (q clickHouseTrafficQuerier) queryTrafficEvents(ctx context.Context, query string, args ...any) (trafficEventRows, error) {
	if q.conn == nil {
		return nil, fmt.Errorf("clickhouse connection is required")
	}
	rows, err := q.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return clickHouseRows{rows: rows}, nil
}

func (r clickHouseRows) Next() bool {
	return r.rows.Next()
}

func (r clickHouseRows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r clickHouseRows) Close() error {
	return r.rows.Close()
}

func (r clickHouseRows) Err() error {
	return r.rows.Err()
}
