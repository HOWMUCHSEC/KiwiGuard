package openai

import (
	"context"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// decisionResult keeps transport-facing policy and verdict outcomes for one evaluation.
type decisionResult struct {
	direction       detection.Direction
	text            string
	verdict         verdict.Result
	decision        policy.Decision
	detectorLatency time.Duration
	verdictLatency  time.Duration
}

// verdictProvider resolves the configured verdict provider for a route.
func (s *server) verdictProvider(route Route) verdict.Provider {
	if route.VerdictProviderKey != "" {
		return s.verdictProviders[route.VerdictProviderKey]
	}
	return s.verdict
}

// evaluate runs the full policy and verdict pipeline for one traffic direction.
func (s *server) evaluate(ctx context.Context, meta requestMeta, direction detection.Direction, text string) decisionResult {
	useCase := appgateway.TrafficEvaluationUseCase{
		Snapshot:       s.snapshot,
		VerdictTimeout: s.verdictTimeout,
		ShadowObserver: shadowEvaluationObserver{server: s, meta: meta},
	}
	result := useCase.Evaluate(ctx, appgateway.TrafficEvaluationInput{
		RequestID:       meta.requestID,
		CorrelationID:   meta.correlationID,
		RouteKey:        meta.route.Key,
		ProviderKey:     meta.route.ProviderKey,
		Model:           meta.mapped,
		Direction:       direction,
		Text:            text,
		Execution:       meta.route.Execution,
		FallbackAction:  meta.route.Fallback,
		VerdictProvider: s.verdictProvider(meta.route),
	})
	return decisionResultFromTrafficEvaluation(result)
}

// evaluateInput runs the full evaluation pipeline for request text.
func (s *server) evaluateInput(ctx context.Context, meta requestMeta, text string) decisionResult {
	return s.evaluate(ctx, meta, detection.DirectionInput, text)
}

// shadowEvaluationObserver emits asynchronous shadow verdict results as traffic events.
type shadowEvaluationObserver struct {
	server *server
	meta   requestMeta
}

// ObserveShadowEvaluation records one shadow verdict evaluation without affecting the live response.
func (o shadowEvaluationObserver) ObserveShadowEvaluation(ctx context.Context, result appgateway.TrafficEvaluationResult) {
	o.server.emit(ctx, o.meta, decisionResultFromTrafficEvaluation(result), "async_shadow_verdict")
}

// decisionResultFromTrafficEvaluation adapts an application evaluation result to transport state.
func decisionResultFromTrafficEvaluation(result appgateway.TrafficEvaluationResult) decisionResult {
	return decisionResult{
		direction:       result.Direction,
		text:            result.Text,
		verdict:         result.Verdict,
		decision:        result.Decision,
		detectorLatency: result.DetectorLatency,
		verdictLatency:  result.VerdictLatency,
	}
}

// inputBlockedDecisionResult returns the canonical blocked input decision placeholder.
func inputBlockedDecisionResult() decisionResult {
	return blockedDecisionResult(detection.DirectionInput)
}

// outputBlockedDecisionResult returns the canonical blocked output decision placeholder.
func outputBlockedDecisionResult() decisionResult {
	return blockedDecisionResult(detection.DirectionOutput)
}

// allowedOutputDecisionResult returns the canonical allowed output decision placeholder.
func allowedOutputDecisionResult() decisionResult {
	return decisionResult{
		direction: detection.DirectionOutput,
		decision:  policy.Decision{Action: policy.ActionAllow},
	}
}

// blockedDecisionResult returns a blocked decision placeholder for the supplied direction.
func blockedDecisionResult(direction detection.Direction) decisionResult {
	return decisionResult{
		direction: direction,
		decision:  policy.Decision{Action: policy.ActionBlock},
	}
}

// allowed reports whether the decision allows the response to continue.
func (r decisionResult) allowed() bool {
	return r.decision.Action == policy.ActionAllow
}

// evaluateOutput runs the full evaluation pipeline for response text.
func (s *server) evaluateOutput(ctx context.Context, meta requestMeta, text string) decisionResult {
	return s.evaluate(ctx, meta, detection.DirectionOutput, text)
}

// evaluateStreamingOutput runs policy-only evaluation for incremental streaming text.
func (s *server) evaluateStreamingOutput(ctx context.Context, meta requestMeta, text string) decisionResult {
	useCase := appgateway.TrafficEvaluationUseCase{Snapshot: s.snapshot}
	result := useCase.EvaluatePolicyOnly(appgateway.TrafficEvaluationInput{
		RouteKey:       meta.route.Key,
		ProviderKey:    meta.route.ProviderKey,
		Model:          meta.mapped,
		Direction:      detection.DirectionOutput,
		Text:           text,
		FallbackAction: meta.route.Fallback,
	})
	return decisionResultFromTrafficEvaluation(result)
}

func decisionFromModelSignal(signal policy.ModelSignal) policy.Decision {
	return appgateway.DecisionFromModelSignal(signal)
}

func policyAction(action Action) policy.Action {
	return appgateway.PolicyAction(action)
}

func knownPolicyAction(action policy.Action) bool {
	return appgateway.KnownPolicyAction(action)
}
