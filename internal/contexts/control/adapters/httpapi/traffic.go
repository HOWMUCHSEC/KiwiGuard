package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const defaultTrafficEventLimit = 50
const maxTrafficEventLimit = 200

// TrafficReader supplies traffic event pages and summaries to the control-plane HTTP adapter.
type TrafficReader interface {
	ListTrafficEvents(context.Context, trafficEventFilter) (trafficEventsResponse, error)
	SummarizeTrafficEvents(context.Context, trafficEventFilter) (trafficEventsSummaryDTO, error)
}

type trafficEventFilter struct {
	RouteID    string
	ProviderID string
	Direction  string
	Status     uint16
	RiskLevel  string
	Limit      int
}

type trafficEventsResponse struct {
	Items   []trafficEventDTO       `json:"items"`
	Summary trafficEventsSummaryDTO `json:"summary"`
}

type trafficEventsSummaryDTO struct {
	Total          uint64 `json:"total"`
	Blocked        uint64 `json:"blocked"`
	UpstreamErrors uint64 `json:"upstream_errors"`
	Fallbacks      uint64 `json:"fallbacks"`
}

type trafficEventDTO struct {
	EventTime         time.Time `json:"event_time"`
	RequestID         string    `json:"request_id"`
	CorrelationID     string    `json:"correlation_id,omitempty"`
	RouteID           string    `json:"route_id"`
	ProviderID        string    `json:"provider_id"`
	Direction         string    `json:"direction"`
	Action            string    `json:"action"`
	GatewayStatus     uint16    `json:"gateway_status"`
	UpstreamStatus    uint16    `json:"upstream_status"`
	ErrorType         string    `json:"error_type,omitempty"`
	BlockReason       string    `json:"block_reason,omitempty"`
	RiskLevel         string    `json:"risk_level,omitempty"`
	RequestedModel    string    `json:"requested_model"`
	MappedModel       string    `json:"mapped_model"`
	LatencyMS         uint32    `json:"latency_ms"`
	VerdictLatencyMS  uint32    `json:"verdict_latency_ms"`
	DetectorLatencyMS uint32    `json:"detector_latency_ms"`
	MatchedSpanCount  uint32    `json:"matched_span_count"`
	FallbackTriggered bool      `json:"fallback_triggered"`
	RequestHash       string    `json:"request_hash"`
	ResponseHash      string    `json:"response_hash"`
	RequestPayload    string    `json:"request_payload"`
	ResponsePayload   string    `json:"response_payload"`
	SpoolStatus       string    `json:"spool_status,omitempty"`
}

func trafficEventsHandler(reader TrafficReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reader == nil {
			writeError(w, http.StatusServiceUnavailable, "traffic_reader_unavailable", "traffic reader is unavailable")
			return
		}

		filter, ok := parseTrafficEventFilter(w, r)
		if !ok {
			return
		}
		response, err := reader.ListTrafficEvents(r.Context(), filter)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "traffic_query_failed", "traffic events query failed")
			return
		}
		if response.Items == nil {
			response.Items = []trafficEventDTO{}
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func parseTrafficEventFilter(w http.ResponseWriter, r *http.Request) (trafficEventFilter, bool) {
	query := r.URL.Query()
	filter := trafficEventFilter{
		RouteID:    query.Get("route_id"),
		ProviderID: query.Get("provider_id"),
		Direction:  query.Get("direction"),
		RiskLevel:  query.Get("risk_level"),
		Limit:      defaultTrafficEventLimit,
	}
	if filter.Direction != "" && filter.Direction != "input" && filter.Direction != "output" {
		writeError(w, http.StatusBadRequest, "invalid_direction", "direction must be input or output")
		return trafficEventFilter{}, false
	}
	if statusValue := query.Get("status"); statusValue != "" {
		status, err := strconv.ParseUint(statusValue, 10, 16)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_status", "status must be an HTTP status code")
			return trafficEventFilter{}, false
		}
		filter.Status = uint16(status)
	}
	if limitValue := query.Get("limit"); limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return trafficEventFilter{}, false
		}
		filter.Limit = limit
	}
	if filter.Limit > maxTrafficEventLimit {
		filter.Limit = maxTrafficEventLimit
	}
	return filter, true
}
