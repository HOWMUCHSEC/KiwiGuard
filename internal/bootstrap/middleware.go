package bootstrap

import "net/http"

// gatewayHandler mounts gateway HTTP endpoints behind shared middleware and metrics.
func (f *Factory) gatewayHandler(handler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", f.metrics.Handler())
	mux.Handle("/", f.httpMiddleware("gateway")(handler))
	return mux
}

// controlHandler leaves the control API handler untouched because auth and routing already live upstream.
func (f *Factory) controlHandler(handler http.Handler) http.Handler {
	return handler
}

// httpMiddleware composes metrics, tracing, and panic recovery for one HTTP service.
func (f *Factory) httpMiddleware(service string) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return recoverHTTPPanic(f.metrics.HTTPMiddleware(service, f.telemetry.HTTPMiddleware(service, handler)))
	}
}

// recoverHTTPPanic converts handler panics into a best-effort 500 response when possible.
func recoverHTTPPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &commitRecorder{ResponseWriter: w}
		defer func() {
			if recover() != nil {
				if !recorder.wroteHeader {
					http.Error(recorder, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}
		}()
		next.ServeHTTP(recorder, r)
	})
}

// commitRecorder tracks whether a response has already been committed to the client.
type commitRecorder struct {
	http.ResponseWriter
	wroteHeader bool
}

// WriteHeader records the first committed status code and forwards it once.
func (r *commitRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

// Write marks the response as committed before forwarding body bytes.
func (r *commitRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(body)
}

// Flush preserves streaming semantics when the wrapped writer implements http.Flusher.
func (r *commitRecorder) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

// Unwrap exposes the wrapped response writer to middleware that inspects nested writers.
func (r *commitRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
