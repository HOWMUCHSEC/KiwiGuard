package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestNewOpenTelemetryUsesGlobalProviderWhenNil(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
	})

	telemetry := NewOpenTelemetry(nil)
	writer := telemetry.WrapWriter(nil)
	if err := writer.Enqueue(context.Background(), events.Event{RouteID: "chat"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "event.enqueue" {
		t.Fatalf("span name = %q, want event.enqueue", spans[0].Name)
	}
}

func TestTelemetryHTTPMiddlewareCreatesRequestSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	handler := telemetry.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "gateway POST /v1/chat/completions" {
		t.Fatalf("span name = %q", spans[0].Name)
	}
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.service", "gateway")
	assertSpanAttribute(t, spans[0].Attributes, "http.request.method", "POST")
	assertSpanAttribute(t, spans[0].Attributes, "url.path", "/v1/chat/completions")
	assertSpanAttribute(t, spans[0].Attributes, "http.route", "/v1/chat/completions")
	assertSpanAttribute(t, spans[0].Attributes, "http.response.status_code", int64(http.StatusAccepted))
}

func TestTelemetryHTTPMiddlewareUsesMatchedRoutePattern(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return telemetry.HTTPMiddleware("control", next)
	})
	router.Put("/api/verdict-providers/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/api/verdict-providers/vertical-security", nil))

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "control PUT /api/verdict-providers/{id}" {
		t.Fatalf("span name = %q", spans[0].Name)
	}
	assertSpanAttribute(t, spans[0].Attributes, "http.route", "/api/verdict-providers/{id}")
	assertSpanAttribute(t, spans[0].Attributes, "url.path", "/api/verdict-providers/vertical-security")
}

func TestTelemetryHTTPMiddlewareExtractsTraceParent(t *testing.T) {
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTextMapPropagator(previousPropagator)
	})

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	handler := telemetry.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	wantTraceID := trace.TraceID{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36}
	wantParentID := trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}
	if spans[0].SpanContext.TraceID() != wantTraceID {
		t.Fatalf("trace ID = %s, want %s", spans[0].SpanContext.TraceID(), wantTraceID)
	}
	if spans[0].Parent.SpanID() != wantParentID {
		t.Fatalf("parent span ID = %s, want %s", spans[0].Parent.SpanID(), wantParentID)
	}
	if spans[0].SpanKind != trace.SpanKindServer {
		t.Fatalf("span kind = %s, want server", spans[0].SpanKind)
	}
}

func TestTelemetryHTTPMiddlewareRecordsPanicSpanBeforeReraising(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	handler := telemetry.HTTPMiddleware("control", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	assertSpanAttribute(t, spans[0].Attributes, "http.response.status_code", int64(http.StatusInternalServerError))
	if spans[0].Status.Code.String() != "Error" {
		t.Fatalf("span status = %s, want Error", spans[0].Status.Code.String())
	}
}

func TestTelemetryHTTPMiddlewarePreservesStreamingInterfaces(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	stream := &streamingResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	handler := telemetry.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		if err := controller.Flush(); err != nil {
			t.Fatalf("Flush() error = %v", err)
		}
	}))

	handler.ServeHTTP(stream, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if !stream.flushed {
		t.Fatal("underlying streaming writer was not flushed")
	}
}

func TestNilTelemetryReturnsPassThroughWrappers(t *testing.T) {
	var telemetry *OpenTelemetry
	called := false
	handler := telemetry.HTTPMiddleware("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if got := telemetry.WrapWriter(recordingWriter{}); got == nil {
		t.Fatal("WrapWriter(nil telemetry) returned nil writer")
	}
	if got := telemetry.WrapBatchSink(recordingBatchSink{}); got == nil {
		t.Fatal("WrapBatchSink(nil telemetry) returned nil sink")
	}
}

func TestTelemetryBatchSinkCreatesWriteSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	sink := telemetry.WrapBatchSink(recordingBatchSink{err: errors.New("write failed")})

	err := sink.WriteBatch(context.Background(), []events.Event{
		{RouteID: "chat", ProviderID: "mock"},
		{RouteID: "chat", ProviderID: "mock"},
	})
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want failure")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "event_sink.write_batch" {
		t.Fatalf("span name = %q", spans[0].Name)
	}
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.event.rows", int64(2))
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.route", "chat")
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.provider", "mock")
	if spans[0].Status.Code.String() != "Error" {
		t.Fatalf("span status = %s, want Error", spans[0].Status.Code.String())
	}
}

func TestTelemetryBatchSinkReportsMissingNextSink(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	sink := telemetry.WrapBatchSink(nil)

	err := sink.WriteBatch(context.Background(), nil)
	if err == nil {
		t.Fatal("WriteBatch() error = nil, want missing sink error")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.event.rows", int64(0))
	if spans[0].Status.Code.String() != "Error" {
		t.Fatalf("span status = %s, want Error", spans[0].Status.Code.String())
	}
}

func TestTelemetryWriterCreatesEventEnqueueSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	writer := telemetry.WrapWriter(recordingWriter{})

	err := writer.Enqueue(context.Background(), events.Event{
		RouteID:    "chat",
		ProviderID: "mock",
		Direction:  events.Direction("input"),
		Action:     events.Action("allow"),
		RiskLevel:  "low",
	})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Name != "event.enqueue" {
		t.Fatalf("span name = %q", spans[0].Name)
	}
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.route", "chat")
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.provider", "mock")
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.direction", "input")
	assertSpanAttribute(t, spans[0].Attributes, "kiwiguard.action", "allow")
}

func TestTelemetryWriterRecordsNextWriterErrors(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	writer := telemetry.WrapWriter(recordingWriter{err: errors.New("enqueue failed")})

	err := writer.Enqueue(context.Background(), events.Event{RouteID: "chat"})
	if err == nil {
		t.Fatal("Enqueue() error = nil, want writer error")
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Status.Code.String() != "Error" {
		t.Fatalf("span status = %s, want Error", spans[0].Status.Code.String())
	}
}

func TestTelemetryWriterAllowsNilNextWriter(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry := NewOpenTelemetry(provider)
	writer := telemetry.WrapWriter(nil)

	if err := writer.Enqueue(context.Background(), events.Event{RouteID: "chat"}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}
	if spans[0].Status.Code.String() == "Error" {
		t.Fatal("span status = Error, want unset success status")
	}
}

func assertSpanAttribute(t *testing.T, attrs []attribute.KeyValue, key string, want any) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) != key {
			continue
		}
		switch want := want.(type) {
		case string:
			if attr.Value.AsString() != want {
				t.Fatalf("attribute %s = %q, want %q", key, attr.Value.AsString(), want)
			}
		case int64:
			if attr.Value.AsInt64() != want {
				t.Fatalf("attribute %s = %d, want %d", key, attr.Value.AsInt64(), want)
			}
		default:
			t.Fatalf("unsupported expected attribute type %T", want)
		}
		return
	}
	t.Fatalf("missing span attribute %s", key)
}
