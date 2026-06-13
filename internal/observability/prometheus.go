// Package observability provides metrics and tracing boundaries for KiwiGuard.
package observability

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var errorForMissingBatchSink = errors.New("event batch sink is required")

// PrometheusMetrics owns KiwiGuard's Prometheus collectors.
type PrometheusMetrics struct {
	registry *prometheus.Registry

	httpRequests        *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	gatewayEvents       *prometheus.CounterVec
	gatewayLatency      *prometheus.HistogramVec
	detectorLatency     *prometheus.HistogramVec
	verdictLatency      *prometheus.HistogramVec
	streamingChunks     *prometheus.HistogramVec
	eventBatches        *prometheus.CounterVec
	eventRows           *prometheus.CounterVec
	eventBatchDuration  *prometheus.HistogramVec
	spoolDepth          prometheus.Gauge
	spoolBytes          prometheus.Gauge
	spoolMaxBytes       prometheus.Gauge
	spoolOldestAge      prometheus.Gauge
	spoolOverflow       prometheus.Gauge
}

// NewPrometheusMetrics builds a Prometheus registry and metric set owned by one KiwiGuard process.
func NewPrometheusMetrics() *PrometheusMetrics {
	metrics := &PrometheusMetrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kiwiguard_http_requests_total",
			Help: "Total HTTP requests handled by KiwiGuard services.",
		}, []string{"service", "method", "route", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_http_request_duration_seconds",
			Help:    "HTTP request duration by KiwiGuard service.",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method", "route", "status"}),
		gatewayEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kiwiguard_gateway_events_total",
			Help: "Total gateway traffic events emitted by KiwiGuard.",
		}, []string{"route", "provider", "client_id", "direction", "action", "risk_level", "gateway_status", "upstream_status"}),
		gatewayLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_gateway_latency_seconds",
			Help:    "Gateway end-to-end traffic latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "provider"}),
		detectorLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_detector_latency_seconds",
			Help:    "Policy detector evaluation latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "direction"}),
		verdictLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_verdict_latency_seconds",
			Help:    "Specialized verdict model latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "provider", "direction"}),
		streamingChunks: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_streaming_chunks",
			Help:    "Streaming chunks observed for a gateway response.",
			Buckets: []float64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
		}, []string{"route", "termination_reason"}),
		eventBatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kiwiguard_event_batches_total",
			Help: "Total event batches written to the configured event sink.",
		}, []string{"outcome", "sink_status", "spool_status"}),
		eventRows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kiwiguard_event_rows_total",
			Help: "Total event rows written to the configured event sink.",
		}, []string{"outcome", "sink_status", "spool_status"}),
		eventBatchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kiwiguard_event_batch_duration_seconds",
			Help:    "Duration of event batch writes to the configured event sink.",
			Buckets: prometheus.DefBuckets,
		}, []string{"outcome"}),
		spoolDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kiwiguard_event_spool_depth",
			Help: "Current number of durable event spool records.",
		}),
		spoolBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kiwiguard_event_spool_bytes",
			Help: "Current durable event spool storage bytes.",
		}),
		spoolMaxBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kiwiguard_event_spool_max_bytes",
			Help: "Configured durable event spool byte capacity.",
		}),
		spoolOldestAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kiwiguard_event_spool_oldest_age_seconds",
			Help: "Age in seconds of the oldest durable event spool record.",
		}),
		spoolOverflow: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kiwiguard_event_spool_overflow_count",
			Help: "Persistent durable event spool overflow count.",
		}),
	}
	metrics.registry.MustRegister(
		metrics.httpRequests,
		metrics.httpRequestDuration,
		metrics.gatewayEvents,
		metrics.gatewayLatency,
		metrics.detectorLatency,
		metrics.verdictLatency,
		metrics.streamingChunks,
		metrics.eventBatches,
		metrics.eventRows,
		metrics.eventBatchDuration,
		metrics.spoolDepth,
		metrics.spoolBytes,
		metrics.spoolMaxBytes,
		metrics.spoolOldestAge,
		metrics.spoolOverflow,
	)
	return metrics
}

// Handler returns an HTTP handler that exposes Prometheus metrics.
func (m *PrometheusMetrics) Handler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// HTTPMiddleware records request counts and latency for a service handler.
func (m *PrometheusMetrics) HTTPMiddleware(service string, next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				if !recorder.wroteHeader {
					recorder.status = http.StatusInternalServerError
				}
				m.observeHTTPRequest(service, r, recorder.status, time.Since(start))
				panic(recovered)
			}
			m.observeHTTPRequest(service, r, recorder.status, time.Since(start))
		}()

		next.ServeHTTP(recorder, r)
	})
}

func (m *PrometheusMetrics) observeHTTPRequest(service string, r *http.Request, statusCode int, duration time.Duration) {
	status := strconv.Itoa(statusCode)
	route := routeLabel(r)
	m.httpRequests.WithLabelValues(service, r.Method, route, status).Inc()
	m.httpRequestDuration.WithLabelValues(service, r.Method, route, status).Observe(duration.Seconds())
}

// WrapWriter records gateway event metrics before forwarding to the next writer.
func (m *PrometheusMetrics) WrapWriter(next events.Writer) events.Writer {
	if m == nil {
		return next
	}
	return eventWriter{metrics: m, next: next}
}

// WrapBatchSink records event sink batch metrics before returning sink results.
func (m *PrometheusMetrics) WrapBatchSink(next events.BatchSink) events.BatchSink {
	if m == nil {
		return next
	}
	return batchSink{metrics: m, next: next}
}

// ObserveEvent records metrics derived from a gateway traffic event.
func (m *PrometheusMetrics) ObserveEvent(event events.Event) {
	if m == nil {
		return
	}
	m.gatewayEvents.WithLabelValues(
		event.RouteID,
		event.ProviderID,
		event.ClientID,
		string(event.Direction),
		string(event.Action),
		event.RiskLevel,
		statusLabel(event.GatewayStatus),
		statusLabel(event.UpstreamStatus),
	).Inc()
	observePositiveDuration(m.gatewayLatency.WithLabelValues(event.RouteID, event.ProviderID), event.GatewayLatency)
	observePositiveDuration(m.detectorLatency.WithLabelValues(event.RouteID, string(event.Direction)), event.DetectorLatency)
	observePositiveDuration(m.verdictLatency.WithLabelValues(event.RouteID, event.VerdictProviderID, string(event.Direction)), event.VerdictLatency)
	if event.StreamingChunkCount > 0 {
		m.streamingChunks.WithLabelValues(event.RouteID, event.TerminationReason).Observe(float64(event.StreamingChunkCount))
	}
}

type eventWriter struct {
	metrics *PrometheusMetrics
	next    events.Writer
}

type batchSink struct {
	metrics *PrometheusMetrics
	next    events.BatchSink
}

func (s batchSink) WriteBatch(ctx context.Context, batch []events.Event) error {
	start := time.Now()
	err := errorForMissingBatchSink
	if s.next != nil {
		err = s.next.WriteBatch(ctx, batch)
	}
	s.metrics.ObserveEventBatch(batch, err, time.Since(start))
	return err
}

func (w eventWriter) Enqueue(ctx context.Context, event events.Event) error {
	w.metrics.ObserveEvent(event)
	if w.next == nil {
		return nil
	}
	return w.next.Enqueue(ctx, event)
}

// ObserveEventBatch records metrics for an event sink batch write.
func (m *PrometheusMetrics) ObserveEventBatch(batch []events.Event, err error, duration time.Duration) {
	if m == nil {
		return
	}
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	sinkStatus, spoolStatus := batchStatuses(batch)
	m.eventBatches.WithLabelValues(outcome, sinkStatus, spoolStatus).Inc()
	m.eventRows.WithLabelValues(outcome, sinkStatus, spoolStatus).Add(float64(len(batch)))
	observePositiveDuration(m.eventBatchDuration.WithLabelValues(outcome), duration)
}

// ObserveSpoolStats records the latest durable event spool state.
func (m *PrometheusMetrics) ObserveSpoolStats(stats events.SpoolStats) {
	if m == nil {
		return
	}
	m.spoolDepth.Set(float64(stats.Depth))
	m.spoolBytes.Set(float64(stats.Bytes))
	m.spoolMaxBytes.Set(float64(stats.MaxBytes))
	m.spoolOldestAge.Set(stats.OldestAge.Seconds())
	m.spoolOverflow.Set(float64(stats.OverflowCount))
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(body)
}

func (r *statusRecorder) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func statusLabel(status uint16) string {
	if status == 0 {
		return "0"
	}
	return strconv.Itoa(int(status))
}

func observePositiveDuration(observer prometheus.Observer, duration time.Duration) {
	if duration <= 0 {
		return
	}
	observer.Observe(duration.Seconds())
}

func routeLabel(r *http.Request) string {
	if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "" {
		return pattern
	}
	if r.Pattern != "" {
		return r.Pattern
	}
	if r.URL == nil || r.URL.Path == "" {
		return "unknown"
	}
	return r.URL.Path
}

func batchStatuses(batch []events.Event) (string, string) {
	if len(batch) == 0 {
		return "", ""
	}
	return batch[0].SinkStatus, batch[0].SpoolStatus
}
