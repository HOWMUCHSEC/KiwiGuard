package openai

import (
	"net/http"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// writeLifecycleError maps application rejection reasons to OpenAI-compatible HTTP errors.
func writeLifecycleError(w http.ResponseWriter, reason appgateway.RejectReason) {
	switch reason {
	case appgateway.RejectUnsupportedPath:
		writeOpenAIError(w, http.StatusNotFound, string(reason), "unsupported path")
	case appgateway.RejectMissingProvider:
		writeOpenAIError(w, http.StatusBadGateway, "missing_route", "missing route")
	case appgateway.RejectAuditSinkUnhealthy:
		writeOpenAIError(w, http.StatusServiceUnavailable, string(reason), "audit sink is unhealthy")
	case appgateway.RejectMissingModelMapping:
		writeOpenAIError(w, http.StatusNotFound, string(reason), "missing model mapping")
	case appgateway.RejectInvalidJSON:
		writeOpenAIError(w, http.StatusBadRequest, string(reason), "invalid json")
	case appgateway.RejectInvalidRequest:
		writeOpenAIError(w, http.StatusBadRequest, string(reason), "invalid request")
	case appgateway.RejectRequestTooLarge:
		writeOpenAIError(w, http.StatusRequestEntityTooLarge, string(reason), "request body is too large")
	case appgateway.RejectBlockedInput:
		writeOpenAIError(w, http.StatusForbidden, string(reason), "request blocked by policy")
	case appgateway.RejectBlockedOutput:
		writeOpenAIError(w, http.StatusForbidden, string(reason), "response blocked by policy")
	case appgateway.RejectRedactionFailed:
		writeOpenAIError(w, http.StatusBadGateway, string(reason), "redaction failed")
	case appgateway.RejectUpstreamError:
		writeOpenAIError(w, http.StatusBadGateway, string(reason), "upstream request failed")
	case appgateway.RejectUpstreamResponseTooLarge:
		writeOpenAIError(w, http.StatusBadGateway, string(reason), "upstream response is too large")
	default:
		writeOpenAIError(w, http.StatusBadRequest, string(reason), "request rejected")
	}
}
