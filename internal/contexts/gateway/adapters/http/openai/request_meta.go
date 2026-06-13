package openai

import (
	"net/http"
	"time"
)

// newRequestMeta derives request-scoped metadata for policy, audit, and event emission.
func (s *server) newRequestMeta(r *http.Request, route Route) requestMeta {
	requestID := requestIDFromHeader(r)
	correlationID := r.Header.Get("X-Correlation-ID")
	if correlationID == "" {
		correlationID = requestID
	}
	return requestMeta{
		requestID:            requestID,
		correlationID:        correlationID,
		start:                time.Now(),
		method:               r.Method,
		path:                 r.URL.Path,
		endpointKind:         endpointKind(r.URL.Path),
		route:                route,
		configRevisionNumber: s.configRevisionNumber,
	}
}
