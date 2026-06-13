package openai

import (
	"net/http"
	"time"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// enforceProtectedRoute authenticates and reserves protected-route capacity before body reads.
func (s *server) enforceProtectedRoute(w http.ResponseWriter, r *http.Request, meta *requestMeta, route Route) (int64, func(), bool) {
	admission := appgateway.AdmissionUseCase{
		Authenticator:      s.clients,
		Limits:             s.limitResolver,
		RateLimiter:        s.limitState,
		ConcurrencyLimiter: s.limitState,
		Audit:              s.auditGate,
	}.AdmitProtectedRoute(appgateway.AdmissionInput{
		ClientKey:           clientKeyFromAuthorization(r.Header.Get("Authorization")),
		RouteKey:            route.Key,
		DefaultMaxBodyBytes: s.maxBodyBytes,
		Now:                 time.Now(),
	})
	if !admission.Allowed {
		meta.clientID = admission.ClientID
		reason := string(admission.Reason)
		s.emit(r.Context(), *meta, inputBlockedDecisionResult(), reason)
		writeProtectedRouteError(w, admission.Reason)
		return 0, func() {}, false
	}
	meta.clientID = admission.ClientID
	return admission.MaxBodyBytes, admission.Release, true
}

// readOpenAIRequestBody reads and classifies one request body under the resolved byte limit.
func (s *server) readOpenAIRequestBody(w http.ResponseWriter, r *http.Request, meta *requestMeta, limit int64) ([]byte, bool) {
	body, tooLarge, err := readLimited(r.Body, limit)
	decision := s.lifecycle.ClassifyRequestBody(appgateway.RequestBodyInput{
		ReadFailed: err != nil,
		TooLarge:   tooLarge,
	})
	if !decision.Allowed {
		reason := decision.Reason
		if err != nil {
			reason = "read_request_error"
		}
		s.emit(r.Context(), *meta, inputBlockedDecisionResult(), string(reason))
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
			return nil, false
		}
		writeLifecycleError(w, decision.Reason)
		return nil, false
	}
	meta.requestBody = body
	meta.requestBytes = int64(len(body))
	return body, true
}

// auditHealthy reports whether the fail-closed audit dependency is ready.
func (s *server) auditHealthy() bool {
	return s.auditGate == nil || s.auditGate.Healthy()
}

// writeProtectedRouteError maps protected-route admission failures to transport errors.
func writeProtectedRouteError(w http.ResponseWriter, reason appgateway.RejectReason) {
	switch reason {
	case appgateway.RejectMissingClientKey, appgateway.RejectInvalidClientKey,
		appgateway.RejectDisabledClientKey, appgateway.RejectRevokedClientKey:
		writeOpenAIError(w, authStatus(string(reason)), string(reason), "gateway client authentication failed")
	case appgateway.RejectMissingLimitPolicy:
		writeOpenAIError(w, http.StatusForbidden, string(reason), "gateway limit policy is required")
	case appgateway.RejectAuditSinkUnhealthy:
		writeOpenAIError(w, http.StatusServiceUnavailable, string(reason), "audit sink is unhealthy")
	case appgateway.RejectRateLimitExceeded:
		writeOpenAIError(w, http.StatusTooManyRequests, string(reason), "rate limit exceeded")
	case appgateway.RejectConcurrencyLimitExceeded:
		writeOpenAIError(w, http.StatusTooManyRequests, string(reason), "concurrency limit exceeded")
	default:
		writeOpenAIError(w, http.StatusForbidden, string(reason), "gateway request rejected")
	}
}
