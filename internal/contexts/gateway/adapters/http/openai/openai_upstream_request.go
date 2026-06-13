package openai

import (
	"net/http"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// projectOpenAIUpstreamRequest applies application-approved request projection for upstream transport.
func (s *server) projectOpenAIUpstreamRequest(w http.ResponseWriter, r *http.Request, exchange openAIExchange) ([]byte, bool) {
	upstreamBody, err := encodeOpenAIRequest(exchange.request, exchange.meta.upstream)
	decision := s.lifecycle.ClassifyUpstreamRequest(appgateway.UpstreamRequestInput{ProjectionFailed: err != nil})
	if !decision.Allowed {
		s.emit(r.Context(), exchange.meta, inputBlockedDecisionResult(), string(decision.Reason))
		writeLifecycleError(w, decision.Reason)
		return nil, false
	}
	return upstreamBody, true
}
