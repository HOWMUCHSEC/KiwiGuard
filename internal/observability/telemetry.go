package observability

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry owns KiwiGuard's OpenTelemetry tracing boundaries.
type OpenTelemetry struct {
	tracer trace.Tracer
}

// NewOpenTelemetry creates tracing instrumentation from a tracer provider.
func NewOpenTelemetry(provider trace.TracerProvider) *OpenTelemetry {
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	return &OpenTelemetry{tracer: provider.Tracer("github.com/howmuchsec/kiwiguard")}
}

// HTTPMiddleware wraps one HTTP handler with server-span extraction, naming, and status recording.
func (t *OpenTelemetry) HTTPMiddleware(service string, next http.Handler) http.Handler {
	if t == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanName := service + " " + r.Method
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String("kiwiguard.service", service),
			attribute.String("http.request.method", r.Method),
			attribute.String("url.path", r.URL.Path),
		))
		defer span.End()

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				if !recorder.wroteHeader {
					recorder.status = http.StatusInternalServerError
				}
				recordHTTPSpan(service, r, span, recorder.status)
				err := fmt.Errorf("panic: %v", recovered)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				panic(recovered)
			}
			recordHTTPSpan(service, r, span, recorder.status)
		}()

		next.ServeHTTP(recorder, r.WithContext(ctx))
	})
}

func recordHTTPSpan(service string, r *http.Request, span trace.Span, statusCode int) {
	route := routeLabel(r)
	span.SetName(service + " " + r.Method + " " + route)
	span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
	span.SetAttributes(attribute.String("http.route", route))
	if statusCode >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, strconv.Itoa(statusCode))
	}
}

// WrapBatchSink creates spans around event sink batch writes.
func (t *OpenTelemetry) WrapBatchSink(next events.BatchSink) events.BatchSink {
	if t == nil {
		return next
	}
	return telemetryBatchSink{telemetry: t, next: next}
}

// WrapWriter creates spans around gateway event enqueue calls.
func (t *OpenTelemetry) WrapWriter(next events.Writer) events.Writer {
	if t == nil {
		return next
	}
	return telemetryEventWriter{telemetry: t, next: next}
}

type telemetryEventWriter struct {
	telemetry *OpenTelemetry
	next      events.Writer
}

type telemetryBatchSink struct {
	telemetry *OpenTelemetry
	next      events.BatchSink
}

func (w telemetryEventWriter) Enqueue(ctx context.Context, event events.Event) error {
	ctx, span := w.telemetry.tracer.Start(ctx, "event.enqueue", trace.WithAttributes(
		attribute.String("kiwiguard.route", event.RouteID),
		attribute.String("kiwiguard.provider", event.ProviderID),
		attribute.String("kiwiguard.direction", string(event.Direction)),
		attribute.String("kiwiguard.action", string(event.Action)),
		attribute.String("kiwiguard.risk_level", event.RiskLevel),
	))
	defer span.End()

	if w.next == nil {
		return nil
	}
	err := w.next.Enqueue(ctx, event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (s telemetryBatchSink) WriteBatch(ctx context.Context, batch []events.Event) error {
	ctx, span := s.telemetry.tracer.Start(ctx, "event_sink.write_batch", trace.WithAttributes(
		attribute.Int("kiwiguard.event.rows", len(batch)),
	))
	defer span.End()

	if len(batch) > 0 {
		span.SetAttributes(
			attribute.String("kiwiguard.route", batch[0].RouteID),
			attribute.String("kiwiguard.provider", batch[0].ProviderID),
		)
	}

	err := errorForMissingBatchSink
	if s.next != nil {
		err = s.next.WriteBatch(ctx, batch)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
