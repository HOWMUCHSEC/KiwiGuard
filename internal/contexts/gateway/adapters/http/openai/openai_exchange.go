package openai

import (
	"net/http"
	"time"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// prepareOpenAIExchange validates transport state and builds the exchange context for one request.
func (s *server) prepareOpenAIExchange(w http.ResponseWriter, r *http.Request) (openAIExchange, func(), bool) {
	planner := appgateway.ExchangePreparationUseCase{Lifecycle: s.lifecycle}
	release := func() {}
	cleanupOnFailure := true
	defer func() {
		if cleanupOnFailure {
			release()
		}
	}()

	route, ok := s.routes[routeKey{method: r.Method, path: r.URL.Path}]
	plan := planner.PlanRouteAvailability(appgateway.RouteAvailabilityInput{Found: ok})
	if plan.Action == appgateway.ExchangePreparationActionBlock {
		meta := s.newRequestMeta(r, Route{})
		s.emit(r.Context(), meta, inputBlockedDecisionResult(), string(plan.Reason))
		writeLifecycleError(w, plan.Reason)
		return openAIExchange{}, nil, false
	}

	meta := s.newRequestMeta(r, route)
	maxBodyBytes := s.maxBodyBytes
	var body []byte
	bodyRead := false

	if s.limitResolver.routeProtected(route.Key) {
		var protected bool
		maxBodyBytes, release, protected = s.enforceProtectedRoute(w, r, &meta, route)
		if !protected {
			return openAIExchange{}, nil, false
		}
		var okRead bool
		body, okRead = s.readOpenAIRequestBody(w, r, &meta, maxBodyBytes)
		if !okRead {
			return openAIExchange{}, nil, false
		}
		bodyRead = true
	}

	provider, ok := s.providers[route.ProviderKey]
	plan = planner.PlanInfrastructure(appgateway.ExchangeInfrastructurePlanInput{
		ProviderFound: ok,
		AuditHealthy:  s.auditHealthy(),
	})
	if plan.Action == appgateway.ExchangePreparationActionBlock {
		s.emit(r.Context(), meta, inputBlockedDecisionResult(), string(plan.Reason))
		writeLifecycleError(w, plan.Reason)
		return openAIExchange{}, nil, false
	}

	if !bodyRead {
		var okRead bool
		body, okRead = s.readOpenAIRequestBody(w, r, &meta, maxBodyBytes)
		if !okRead {
			return openAIExchange{}, nil, false
		}
	}

	openAIReq, err := parseOpenAIRequest(r.URL.Path, body)
	var mappingInput appgateway.ModelMappingInput
	if err == nil {
		meta.requested = openAIReq.model
		resolvedMapping := route.ModelMapping.Resolve(openAIReq.model)
		mappingInput = appgateway.ModelMappingInput{
			RequestedModel: openAIReq.model,
			MappedModel:    resolvedMapping.Mapped,
			UpstreamModel:  resolvedMapping.Upstream,
			Found:          resolvedMapping.Found,
		}
	}
	plan = planner.PlanDecodedRequest(appgateway.DecodedRequestPlanInput{
		DecodeFailed: err != nil,
		ModelMapping: mappingInput,
	})
	meta.requested = plan.RequestedModel
	meta.mapped = plan.MappedModel
	meta.upstream = plan.UpstreamModel
	if plan.Action == appgateway.ExchangePreparationActionBlock {
		s.emit(r.Context(), meta, inputBlockedDecisionResult(), string(plan.Reason))
		writeLifecycleError(w, plan.Reason)
		return openAIExchange{}, nil, false
	}

	cleanupOnFailure = false
	return openAIExchange{
		provider: provider,
		meta:     meta,
		request:  openAIReq,
	}, release, true
}

// forwardOpenAIExchange runs the upstream request and response lifecycle for one exchange.
func (s *server) forwardOpenAIExchange(w http.ResponseWriter, r *http.Request, exchange openAIExchange, upstreamBody []byte) {
	planner := appgateway.UpstreamExchangeUseCase{Lifecycle: s.lifecycle}
	upstreamCtx, upstreamCancel := withTimeout(r.Context(), s.upstreamTimeout)
	defer upstreamCancel()

	upstreamResp, upstreamBodyBytes, err := s.forward(upstreamCtx, exchange.provider, r, upstreamBody)
	exchange.meta.upstreamTime = time.Since(exchange.meta.start)
	plan := planner.PlanForward(appgateway.UpstreamForwardPlanInput{
		ForwardFailed: err != nil,
	})
	if plan.Action == appgateway.UpstreamExchangeActionBlock {
		s.emit(r.Context(), exchange.meta, outputBlockedDecisionResult(), string(plan.Reason))
		writeLifecycleError(w, plan.Reason)
		return
	}
	defer func() {
		_ = upstreamResp.Body.Close()
	}()
	exchange.meta.upstreamStatus = uint16(upstreamResp.StatusCode)

	plan = planner.PlanResponse(appgateway.UpstreamResponsePlanInput{
		StatusCode: upstreamResp.StatusCode,
		Stream:     isEventStream(upstreamResp.Header.Get("Content-Type")),
	})
	if plan.Action == appgateway.UpstreamExchangeActionBlock {
		s.emit(r.Context(), exchange.meta, outputBlockedDecisionResult(), string(plan.Reason))
		writeLifecycleError(w, plan.Reason)
		return
	}
	if plan.Action == appgateway.UpstreamExchangeActionHandleStream {
		s.handleStreaming(w, r, upstreamCtx, upstreamResp, upstreamBodyBytes, exchange.meta)
		return
	}

	s.handleOpenAIResponse(w, r, upstreamResp, upstreamBodyBytes, exchange.meta)
}
