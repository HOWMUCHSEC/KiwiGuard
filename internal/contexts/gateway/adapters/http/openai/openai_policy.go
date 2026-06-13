package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// applyOpenAIInputPolicy enforces input policy and redaction before upstream forwarding.
func (s *server) applyOpenAIInputPolicy(w http.ResponseWriter, r *http.Request, exchange *openAIExchange) bool {
	planner := appgateway.InputRequestUseCase{Lifecycle: s.lifecycle}
	inputDecision := s.evaluateInput(r.Context(), exchange.meta, exchange.request.text)
	plan := planner.PlanPolicy(appgateway.InputPolicyInput{
		Decision: inputDecision.decision,
	})
	if s.rejectInputRequestPlan(r.Context(), w, exchange.meta, inputDecision, plan) {
		return false
	}
	if plan.Action == appgateway.InputRequestActionSend {
		s.emit(r.Context(), exchange.meta, inputDecision, "")
		return true
	}

	redactions, err := redactOpenAIRequest(&exchange.request, inputDecision.decision.Findings, inputDecision.verdict.MatchedSpans)
	plan = planner.PlanRedaction(appgateway.InputRedactionInput{
		Decision:   inputDecision.decision,
		Text:       exchange.request.text,
		Redactions: redactions,
		Failed:     err != nil,
	})
	if s.rejectInputRequestPlan(r.Context(), w, exchange.meta, inputDecision, plan) {
		return false
	}
	exchange.meta.redactions += redactions
	redactedBody, err := json.Marshal(exchange.request.body)
	if err != nil {
		plan = planner.PlanRedaction(appgateway.InputRedactionInput{
			Decision:   inputDecision.decision,
			Text:       exchange.request.text,
			Redactions: redactions,
			Failed:     true,
		})
		s.rejectInputRequestPlan(r.Context(), w, exchange.meta, inputDecision, plan)
		return false
	}
	exchange.meta.requestBody = redactedBody
	s.emit(r.Context(), exchange.meta, inputDecision, "")
	return true
}

// rejectInputRequestPlan emits the observed decision and writes a rejection response when blocked.
func (s *server) rejectInputRequestPlan(ctx context.Context, w http.ResponseWriter, meta requestMeta, observed decisionResult, plan appgateway.InputRequestPlan) bool {
	if plan.Action != appgateway.InputRequestActionBlock {
		return false
	}
	s.emit(ctx, meta, observed, plan.TerminationReason)
	writeLifecycleError(w, plan.Reason)
	return true
}

// handleOpenAIResponse applies output policy and optional redaction to a buffered upstream response.
func (s *server) handleOpenAIResponse(w http.ResponseWriter, r *http.Request, upstreamResp *http.Response, upstreamBody io.Reader, meta requestMeta) {
	planner := appgateway.OutputResponseUseCase{Lifecycle: s.lifecycle}
	responseBody, tooLarge, err := readLimited(upstreamBody, s.maxBodyBytes)
	plan := planner.PlanBodyRead(appgateway.OutputBodyReadInput{
		StatusCode: upstreamResp.StatusCode,
		ReadFailed: err != nil,
		TooLarge:   tooLarge,
	})
	if s.rejectOutputResponsePlan(r.Context(), w, meta, decisionResult{}, plan) {
		return
	}
	meta.responseBody = responseBody
	meta.responseBytes = int64(len(responseBody))

	outputText, err := extractOpenAIOutput(r.URL.Path, responseBody)
	plan = planner.PlanExtraction(appgateway.OutputExtractionInput{
		StatusCode:   upstreamResp.StatusCode,
		DecodeFailed: err != nil,
	})
	if s.rejectOutputResponsePlan(r.Context(), w, meta, decisionResult{}, plan) {
		return
	}
	outputDecision := s.evaluateOutput(r.Context(), meta, outputText)
	plan = planner.PlanPolicy(appgateway.OutputPolicyInput{
		StatusCode: upstreamResp.StatusCode,
		Decision:   outputDecision.decision,
	})
	if s.rejectOutputResponsePlan(r.Context(), w, meta, outputDecision, plan) {
		return
	}
	if plan.Action == appgateway.OutputResponseActionRedact {
		redactedBody, redactions, err := redactOpenAIResponse(r.URL.Path, responseBody, outputDecision.decision.Findings, outputDecision.verdict.MatchedSpans)
		plan = planner.PlanRedaction(appgateway.OutputRedactionInput{
			StatusCode: upstreamResp.StatusCode,
			Decision:   outputDecision.decision,
			Text:       outputText,
			Redactions: redactions,
			Failed:     err != nil,
		})
		if s.rejectOutputResponsePlan(r.Context(), w, meta, outputDecision, plan) {
			return
		}
		responseBody = redactedBody
		meta.responseBody = responseBody
		meta.responseBytes = int64(len(responseBody))
		meta.redactions += redactions
	}
	s.emit(r.Context(), meta, outputDecision, plan.TerminationReason)

	copyResponseHeaders(w.Header(), upstreamResp.Header)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(upstreamResp.StatusCode)
	_, _ = w.Write(responseBody)
}

// rejectOutputResponsePlan emits the correct output decision and writes a rejection response when blocked.
func (s *server) rejectOutputResponsePlan(ctx context.Context, w http.ResponseWriter, meta requestMeta, observed decisionResult, plan appgateway.OutputResponsePlan) bool {
	if plan.Action != appgateway.OutputResponseActionBlock {
		return false
	}
	result := outputBlockedDecisionResult()
	if plan.EmitPolicyDecision {
		result = observed
	}
	s.emit(ctx, meta, result, plan.TerminationReason)
	writeLifecycleError(w, plan.Reason)
	return true
}
