package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestTrafficEventsEndpointUsesReaderFilters(t *testing.T) {
	reader := &fakeTrafficReader{
		response: trafficEventsResponse{
			Items: []trafficEventDTO{{
				EventTime:        time.Unix(10, 0).UTC(),
				RequestID:        "req-1",
				RouteID:          "openai",
				ProviderID:       "mock",
				Direction:        "output",
				Action:           "allow",
				GatewayStatus:    200,
				UpstreamStatus:   200,
				RequestedModel:   "gpt-test",
				MappedModel:      "gpt-mapped",
				LatencyMS:        17,
				MatchedSpanCount: 2,
				RequestHash:      "request-hash",
				ResponseHash:     "response-hash",
				RequestPayload:   `{"model":"gpt-test"}`,
				ResponsePayload:  `{"output_text":"safe"}`,
				SpoolStatus:      "replayed",
			}},
			Summary: trafficEventsSummaryDTO{
				Total:          1,
				Blocked:        0,
				UpstreamErrors: 0,
				Fallbacks:      0,
			},
		},
	}
	server := NewServer(ServerOptions{Version: "test", TrafficReader: reader})

	response := get(t, server, "/api/traffic/events?route_id=openai&provider_id=mock&direction=output&status=200&risk_level=high&limit=25")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}

	if reader.filter.RouteID != "openai" {
		t.Fatalf("RouteID = %q, want openai", reader.filter.RouteID)
	}
	if reader.filter.ProviderID != "mock" {
		t.Fatalf("ProviderID = %q, want mock", reader.filter.ProviderID)
	}
	if reader.filter.Direction != "output" {
		t.Fatalf("Direction = %q, want output", reader.filter.Direction)
	}
	if reader.filter.Status != 200 {
		t.Fatalf("Status = %d, want 200", reader.filter.Status)
	}
	if reader.filter.RiskLevel != "high" {
		t.Fatalf("RiskLevel = %q, want high", reader.filter.RiskLevel)
	}
	if reader.filter.Limit != 25 {
		t.Fatalf("Limit = %d, want 25", reader.filter.Limit)
	}

	var body trafficEventsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].Direction != "output" {
		t.Fatalf("Direction = %q, want output", body.Items[0].Direction)
	}
	if body.Items[0].RequestPayload != `{"model":"gpt-test"}` {
		t.Fatalf("RequestPayload = %q, want payload", body.Items[0].RequestPayload)
	}
	if body.Items[0].ResponsePayload != `{"output_text":"safe"}` {
		t.Fatalf("ResponsePayload = %q, want payload", body.Items[0].ResponsePayload)
	}
	if body.Items[0].SpoolStatus != "replayed" {
		t.Fatalf("SpoolStatus = %q, want replayed", body.Items[0].SpoolStatus)
	}
	if body.Summary.Total != 1 {
		t.Fatalf("Summary.Total = %d, want 1", body.Summary.Total)
	}
}

func TestTrafficEventsEndpointRejectsInvalidFilters(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test", TrafficReader: &fakeTrafficReader{}})

	response := get(t, server, "/api/traffic/events?direction=sideways")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}

	response = get(t, server, "/api/traffic/events?status=abc")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}

	response = get(t, server, "/api/traffic/events?limit=0")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestTrafficEventsEndpointReturnsUnavailableWithoutReader(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	response := get(t, server, "/api/traffic/events")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", response.Code, response.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "traffic_reader_unavailable" {
		t.Fatalf("Code = %q, want traffic_reader_unavailable", body.Code)
	}
}

func TestTrafficEventsEndpointCapsDefaultLimitAndNormalizesNilItems(t *testing.T) {
	reader := &fakeTrafficReader{response: trafficEventsResponse{}}
	server := NewServer(ServerOptions{Version: "test", TrafficReader: reader})

	response := get(t, server, "/api/traffic/events?limit=999")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if reader.filter.Limit != maxTrafficEventLimit {
		t.Fatalf("Limit = %d, want %d", reader.filter.Limit, maxTrafficEventLimit)
	}
	var body trafficEventsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items == nil {
		t.Fatal("Items is nil, want empty slice")
	}

	response = get(t, server, "/api/traffic/events")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if reader.filter.Limit != defaultTrafficEventLimit {
		t.Fatalf("Limit = %d, want default %d", reader.filter.Limit, defaultTrafficEventLimit)
	}
}

func TestTrafficEventsEndpointReturnsQueryError(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test", TrafficReader: &fakeTrafficReader{err: errors.New("query failed")}})

	response := get(t, server, "/api/traffic/events")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", response.Code, response.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "traffic_query_failed" {
		t.Fatalf("Code = %q, want traffic_query_failed", body.Code)
	}
}

type fakeTrafficReader struct {
	filter        trafficEventFilter
	summaryFilter trafficEventFilter
	response      trafficEventsResponse
	err           error
}

func (r *fakeTrafficReader) ListTrafficEvents(ctx context.Context, filter trafficEventFilter) (trafficEventsResponse, error) {
	r.filter = filter
	return r.response, r.err
}

func (r *fakeTrafficReader) SummarizeTrafficEvents(ctx context.Context, filter trafficEventFilter) (trafficEventsSummaryDTO, error) {
	r.summaryFilter = filter
	return r.response.Summary, r.err
}
