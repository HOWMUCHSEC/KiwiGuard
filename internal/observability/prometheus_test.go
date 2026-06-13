package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
)

func TestHTTPMiddlewareRecordsRequests(t *testing.T) {
	metrics := NewPrometheusMetrics()
	handler := metrics.HTTPMiddleware("control", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/policy-bundles", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_http_requests_total{method="POST",route="/api/policy-bundles",service="control",status="201"} 1`)
	assertMetricContains(t, body, `kiwiguard_http_request_duration_seconds_bucket{method="POST",route="/api/policy-bundles",service="control",status="201"`)
}

func TestHTTPMiddlewareRecordsFirstWrittenStatus(t *testing.T) {
	metrics := NewPrometheusMetrics()
	handler := metrics.HTTPMiddleware("control", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/policy-bundles", nil))

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_http_requests_total{method="POST",route="/api/policy-bundles",service="control",status="202"} 1`)
}

func TestHTTPMiddlewareRecordsPanicsBeforeReraising(t *testing.T) {
	metrics := NewPrometheusMetrics()
	handler := metrics.HTTPMiddleware("control", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("handler failed")
	}))

	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("ServeHTTP() did not re-panic")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/panic", nil))
	}()

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_http_requests_total{method="GET",route="/api/panic",service="control",status="500"} 1`)
}

func TestHTTPMiddlewareUsesMatchedRoutePattern(t *testing.T) {
	metrics := NewPrometheusMetrics()
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return metrics.HTTPMiddleware("control", next)
	})
	router.Put("/api/model-mappings/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/model-mappings/123", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/model-mappings/456", nil))

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_http_requests_total{method="PUT",route="/api/model-mappings/{id}",service="control",status="204"} 2`)
	if strings.Contains(body, `route="/api/model-mappings/123"`) || strings.Contains(body, `route="/api/model-mappings/456"`) {
		t.Fatalf("metrics body contains raw dynamic route labels:\n%s", body)
	}
}

func TestHTTPMiddlewarePreservesStreamingInterfaces(t *testing.T) {
	metrics := NewPrometheusMetrics()
	stream := &streamingResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	handler := metrics.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("wrapped response writer does not implement http.Flusher")
		}
		controller := http.NewResponseController(w)
		if err := controller.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(stream, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	if !stream.flushed {
		t.Fatal("underlying streaming writer was not flushed")
	}
}

func TestNilPrometheusMetricsReturnsPassThroughHandlers(t *testing.T) {
	var metrics *PrometheusMetrics
	called := false
	handler := metrics.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if !called {
		t.Fatal("pass-through handler was not called")
	}
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.Code)
	}
	if got := metrics.WrapWriter(recordingWriter{}); got == nil {
		t.Fatal("WrapWriter(nil metrics) returned nil writer")
	}
	if got := metrics.WrapBatchSink(recordingBatchSink{}); got == nil {
		t.Fatal("WrapBatchSink(nil metrics) returned nil sink")
	}

	notFound := metrics.Handler()
	resp = httptest.NewRecorder()
	notFound.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("nil Handler status = %d, want 404", resp.Code)
	}
}

func TestWriterRecordsGatewayEvents(t *testing.T) {
	metrics := NewPrometheusMetrics()
	writer := metrics.WrapWriter(recordingWriter{})

	err := writer.Enqueue(context.Background(), events.Event{
		RouteID:             "chat",
		ProviderID:          "mock",
		VerdictProviderID:   "vertical-security",
		Direction:           events.Direction("output"),
		Action:              events.Action("block"),
		RiskLevel:           "high",
		GatewayStatus:       http.StatusForbidden,
		UpstreamStatus:      http.StatusOK,
		GatewayLatency:      150 * time.Millisecond,
		DetectorLatency:     12 * time.Millisecond,
		VerdictLatency:      40 * time.Millisecond,
		StreamingChunkCount: 3,
		TerminationReason:   "stream_blocked",
	})
	if err != nil {
		t.Fatalf("enqueue event: %v", err)
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_gateway_events_total{action="block",client_id="",direction="output",gateway_status="403",provider="mock",risk_level="high",route="chat",upstream_status="200"} 1`)
	assertMetricContains(t, body, `kiwiguard_gateway_latency_seconds_bucket{provider="mock",route="chat"`)
	assertMetricContains(t, body, `kiwiguard_detector_latency_seconds_bucket{direction="output",route="chat"`)
	assertMetricContains(t, body, `kiwiguard_verdict_latency_seconds_bucket{direction="output",provider="vertical-security",route="chat"`)
	assertMetricContains(t, body, `kiwiguard_streaming_chunks_bucket{route="chat",termination_reason="stream_blocked"`)
}

func TestWriterAllowsNilNextWriter(t *testing.T) {
	metrics := NewPrometheusMetrics()
	writer := metrics.WrapWriter(nil)

	err := writer.Enqueue(context.Background(), events.Event{
		RouteID:       "chat",
		ProviderID:    "mock",
		Direction:     events.Direction("input"),
		Action:        events.Action("allow"),
		GatewayStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_gateway_events_total{action="allow",client_id="",direction="input",gateway_status="200",provider="mock",risk_level="",route="chat",upstream_status="0"} 1`)
}

func TestPrometheusMetricsLabelsGatewayEventsByClientID(t *testing.T) {
	metrics := NewPrometheusMetrics()
	metrics.ObserveEvent(events.Event{
		RouteID:       "chat",
		ProviderID:    "mock",
		ClientID:      "client-a",
		GatewayStatus: http.StatusTooManyRequests,
		Action:        events.Action("block"),
	})

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `client_id="client-a"`)
	if strings.Contains(body, "kgk_") {
		t.Fatalf("metrics leaked raw key:\n%s", body)
	}
}

func TestSinkRecordsBatchOutcomes(t *testing.T) {
	metrics := NewPrometheusMetrics()
	sink := metrics.WrapBatchSink(recordingBatchSink{})

	err := sink.WriteBatch(context.Background(), []events.Event{
		{SinkStatus: "delivered", SpoolStatus: "replayed"},
		{SinkStatus: "delivered", SpoolStatus: "replayed"},
	})
	if err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_batches_total{outcome="success",sink_status="delivered",spool_status="replayed"} 1`)
	assertMetricContains(t, body, `kiwiguard_event_rows_total{outcome="success",sink_status="delivered",spool_status="replayed"} 2`)
	assertMetricContains(t, body, `kiwiguard_event_batch_duration_seconds_bucket{outcome="success"`)
}

func TestSinkRecordsFailedBatchOutcomes(t *testing.T) {
	metrics := NewPrometheusMetrics()
	sink := metrics.WrapBatchSink(recordingBatchSink{err: context.DeadlineExceeded})

	err := sink.WriteBatch(context.Background(), []events.Event{{SinkStatus: "failed"}})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want failure")
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_batches_total{outcome="failure",sink_status="failed",spool_status=""} 1`)
	assertMetricContains(t, body, `kiwiguard_event_rows_total{outcome="failure",sink_status="failed",spool_status=""} 1`)
}

func TestSinkRecordsMissingBatchSinkAsFailure(t *testing.T) {
	metrics := NewPrometheusMetrics()
	sink := metrics.WrapBatchSink(nil)

	err := sink.WriteBatch(context.Background(), []events.Event{{SinkStatus: "queued", SpoolStatus: "memory"}})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want missing sink error")
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_batches_total{outcome="failure",sink_status="queued",spool_status="memory"} 1`)
	assertMetricContains(t, body, `kiwiguard_event_rows_total{outcome="failure",sink_status="queued",spool_status="memory"} 1`)
}

func TestObserveEventBatchRecordsEmptyBatch(t *testing.T) {
	metrics := NewPrometheusMetrics()

	metrics.ObserveEventBatch(nil, nil, 0)

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_batches_total{outcome="success",sink_status="",spool_status=""} 1`)
	assertMetricContains(t, body, `kiwiguard_event_rows_total{outcome="success",sink_status="",spool_status=""} 0`)
}

func TestSinkRecordsSuccessfulDurableSpoolFallback(t *testing.T) {
	metrics := NewPrometheusMetrics()
	spool, err := events.NewFileSpool(events.FileSpoolOptions{Dir: t.TempDir(), MaxBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewFileSpool() error = %v", err)
	}
	sink := metrics.WrapBatchSink(events.NewDurableSink(events.DurableSinkOptions{
		Primary: recordingBatchSink{err: context.DeadlineExceeded},
		Spool:   spool,
	}))

	err = sink.WriteBatch(context.Background(), []events.Event{{EventID: "evt-1", SinkStatus: "delivered"}})
	if err != nil {
		t.Fatalf("WriteBatch() error = %v", err)
	}

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_batches_total{outcome="success",sink_status="spooled",spool_status="spooled"} 1`)
	assertMetricContains(t, body, `kiwiguard_event_rows_total{outcome="success",sink_status="spooled",spool_status="spooled"} 1`)
}

func TestSpoolStatsRecordsDepthCapacityAndOverflow(t *testing.T) {
	metrics := NewPrometheusMetrics()

	metrics.ObserveSpoolStats(events.SpoolStats{
		Depth:         7,
		Bytes:         2048,
		MaxBytes:      4096,
		OldestAge:     3 * time.Second,
		OverflowCount: 2,
	})

	body := scrapeMetrics(t, metrics)
	assertMetricContains(t, body, `kiwiguard_event_spool_depth 7`)
	assertMetricContains(t, body, `kiwiguard_event_spool_bytes 2048`)
	assertMetricContains(t, body, `kiwiguard_event_spool_max_bytes 4096`)
	assertMetricContains(t, body, `kiwiguard_event_spool_oldest_age_seconds 3`)
	assertMetricContains(t, body, `kiwiguard_event_spool_overflow_count 2`)
}

func TestStatusRecorderWriteMarksDefaultStatusAndUnwraps(t *testing.T) {
	resp := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: resp, status: http.StatusAccepted}

	if _, err := recorder.Write([]byte("ok")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if recorder.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.status)
	}
	if recorder.Unwrap() != resp {
		t.Fatal("Unwrap() did not return underlying response writer")
	}
}

func TestRouteLabelFallsBackToRequestPatternAndUnknown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/raw", nil)
	req.Pattern = "/pattern"
	if got := routeLabel(req); got != "/pattern" {
		t.Fatalf("routeLabel() = %q, want request pattern", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL = nil
	if got := routeLabel(req); got != "unknown" {
		t.Fatalf("routeLabel(nil URL) = %q, want unknown", got)
	}
}

func scrapeMetrics(t *testing.T, metrics *PrometheusMetrics) string {
	t.Helper()

	resp := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", resp.Code)
	}
	return resp.Body.String()
}

func assertMetricContains(t *testing.T, body string, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Fatalf("metrics body does not contain %q\n%s", want, body)
	}
}

type recordingWriter struct {
	err error
}

func (w recordingWriter) Enqueue(context.Context, events.Event) error {
	return w.err
}

type recordingBatchSink struct {
	err error
}

func (s recordingBatchSink) WriteBatch(context.Context, []events.Event) error {
	return s.err
}

type streamingResponseWriter struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (w *streamingResponseWriter) Flush() {
	w.flushed = true
}
