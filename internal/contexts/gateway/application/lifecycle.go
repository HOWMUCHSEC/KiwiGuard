package application

import (
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// LifecycleUseCase owns transport-neutral gateway lifecycle decisions.
type LifecycleUseCase struct{}

// RouteAvailable gates the exchange on whether the requested endpoint resolves to an active route.
func (LifecycleUseCase) RouteAvailable(found bool) LifecycleDecision {
	if !found {
		return LifecycleDecision{Reason: RejectUnsupportedPath}
	}
	return LifecycleDecision{Allowed: true}
}

// ProviderAvailable gates the exchange on whether the resolved route has a usable upstream provider.
func (LifecycleUseCase) ProviderAvailable(found bool) LifecycleDecision {
	if !found {
		return LifecycleDecision{Reason: RejectMissingProvider}
	}
	return LifecycleDecision{Allowed: true}
}

// AuditHealthy gates the exchange on whether fail-closed audit persistence is ready to record traffic.
func (LifecycleUseCase) AuditHealthy(healthy bool) LifecycleDecision {
	if !healthy {
		return LifecycleDecision{Reason: RejectAuditSinkUnhealthy}
	}
	return LifecycleDecision{Allowed: true}
}

// ClassifyExchangeInfrastructure validates infrastructure needed before upstream forwarding.
func (u LifecycleUseCase) ClassifyExchangeInfrastructure(input ExchangeInfrastructureInput) LifecycleDecision {
	if decision := u.ProviderAvailable(input.ProviderFound); !decision.Allowed {
		return decision
	}
	return u.AuditHealthy(input.AuditHealthy)
}

// ResolveModelMapping turns a raw mapping lookup into the normalized exchange model contract.
func (LifecycleUseCase) ResolveModelMapping(input ModelMappingInput) ModelMappingResult {
	if !input.Found {
		return ModelMappingResult{
			LifecycleDecision: LifecycleDecision{Reason: RejectMissingModelMapping},
			RequestedModel:    input.RequestedModel,
		}
	}
	return ModelMappingResult{
		LifecycleDecision: LifecycleDecision{Allowed: true},
		RequestedModel:    input.RequestedModel,
		MappedModel:       input.MappedModel,
		UpstreamModel:     input.UpstreamModel,
	}
}

// ClassifyDecodedRequest combines decode success with model mapping to decide whether processing may continue.
func (u LifecycleUseCase) ClassifyDecodedRequest(input DecodedRequestInput) ModelMappingResult {
	if input.DecodeFailed {
		return ModelMappingResult{
			LifecycleDecision: LifecycleDecision{Reason: RejectInvalidJSON},
		}
	}
	return u.ResolveModelMapping(input.ModelMapping)
}

// ClassifyEnforcement turns a policy decision into lifecycle behavior independent of transport details.
func (LifecycleUseCase) ClassifyEnforcement(direction detection.Direction, decision policy.Decision) EnforcementResult {
	switch decision.Action {
	case policy.ActionBlock:
		return EnforcementResult{Blocked: true, Reason: blockedReason(direction)}
	case policy.ActionRedact:
		return EnforcementResult{RedactionNeeded: true}
	default:
		return EnforcementResult{AllowObservation: true}
	}
}

// ClassifyInputEnforcement applies request-side blocking and redaction semantics.
func (u LifecycleUseCase) ClassifyInputEnforcement(decision policy.Decision) EnforcementResult {
	return u.ClassifyEnforcement(detection.DirectionInput, decision)
}

// ClassifyOutputEnforcement applies response-side blocking and redaction semantics.
func (u LifecycleUseCase) ClassifyOutputEnforcement(decision policy.Decision) EnforcementResult {
	return u.ClassifyEnforcement(detection.DirectionOutput, decision)
}

// ClassifyPolicyEnforcement decides whether traffic may continue after policy evaluation and redaction.
func (u LifecycleUseCase) ClassifyPolicyEnforcement(input PolicyEnforcementInput) PolicyEnforcementResult {
	enforcement := u.ClassifyEnforcement(input.Direction, input.Decision)
	result := PolicyEnforcementResult{EnforcementResult: enforcement}
	if enforcement.Blocked || !enforcement.RedactionNeeded || !input.RedactionAttempted {
		return result
	}

	redaction := u.ClassifyRedaction(RedactionInput{
		Text:       input.RedactionText,
		Redactions: input.RedactionCount,
		Failed:     input.RedactionFailed,
	})
	if redaction.Allowed {
		result.AllowObservation = true
		return result
	}
	result.RedactionRejected = true
	result.Reason = redactionReason(redaction.Reason, input.RedactionFailureTag)
	return result
}

// ClassifyInputPolicyEnforcement classifies input policy and redaction facts.
func (u LifecycleUseCase) ClassifyInputPolicyEnforcement(input PolicyEnforcementInput) PolicyEnforcementResult {
	input.Direction = detection.DirectionInput
	return u.ClassifyPolicyEnforcement(input)
}

// ClassifyOutputPolicyEnforcement classifies output policy and redaction facts.
func (u LifecycleUseCase) ClassifyOutputPolicyEnforcement(input PolicyEnforcementInput) PolicyEnforcementResult {
	input.Direction = detection.DirectionOutput
	return u.ClassifyPolicyEnforcement(input)
}

// ClassifyInputRequest decides whether a request may proceed after input policy enforcement.
func (u LifecycleUseCase) ClassifyInputRequest(input InputRequestLifecycleInput) InputRequestLifecycleResult {
	if !input.PolicyEvaluated {
		return InputRequestLifecycleResult{LifecycleDecision: LifecycleDecision{Allowed: true}}
	}

	enforcement := u.ClassifyInputPolicyEnforcement(PolicyEnforcementInput{
		Decision:            input.Decision,
		RedactionAttempted:  input.RedactionAttempted,
		RedactionText:       input.RedactionText,
		RedactionCount:      input.RedactionCount,
		RedactionFailed:     input.RedactionFailed,
		RedactionFailureTag: input.RedactionFailureTag,
	})
	result := InputRequestLifecycleResult{
		LifecycleDecision:      LifecycleDecision{Allowed: true},
		PolicyDecisionObserved: true,
		RedactionNeeded:        enforcement.RedactionNeeded && !input.RedactionAttempted,
		RedactionRejected:      enforcement.RedactionRejected,
	}
	if enforcement.Blocked || enforcement.RedactionRejected {
		result.Allowed = false
		result.Reason = enforcement.Reason
	}
	return result
}

// ClassifyRequestBody classifies request-body read failures and limit breaches before decoding.
func (LifecycleUseCase) ClassifyRequestBody(input RequestBodyInput) LifecycleDecision {
	switch {
	case input.ReadFailed:
		return LifecycleDecision{Reason: RejectInvalidRequest}
	case input.TooLarge:
		return LifecycleDecision{Reason: RejectRequestTooLarge}
	default:
		return LifecycleDecision{Allowed: true}
	}
}

// ClassifyUpstreamResponse classifies upstream transport and decode failures before output policy runs.
func (LifecycleUseCase) ClassifyUpstreamResponse(input UpstreamResponseInput) LifecycleDecision {
	switch {
	case input.ReadFailed:
		return LifecycleDecision{Reason: RejectUpstreamError}
	case input.TooLarge:
		return LifecycleDecision{Reason: RejectUpstreamResponseTooLarge}
	case input.StatusCode >= 400:
		return LifecycleDecision{Reason: RejectUpstreamError}
	case input.DecodeFailed:
		return LifecycleDecision{Reason: RejectUpstreamError}
	default:
		return LifecycleDecision{Allowed: true}
	}
}

// ClassifyOutputResponse decides whether a non-streaming upstream response may be released downstream.
func (u LifecycleUseCase) ClassifyOutputResponse(input OutputResponseLifecycleInput) OutputResponseLifecycleResult {
	if decision := u.ClassifyUpstreamResponse(input.UpstreamResponseInput); !decision.Allowed {
		return OutputResponseLifecycleResult{LifecycleDecision: decision}
	}
	if !input.PolicyEvaluated {
		return OutputResponseLifecycleResult{LifecycleDecision: LifecycleDecision{Allowed: true}}
	}

	enforcement := u.ClassifyOutputPolicyEnforcement(PolicyEnforcementInput{
		Decision:            input.Decision,
		RedactionAttempted:  input.RedactionAttempted,
		RedactionText:       input.RedactionText,
		RedactionCount:      input.RedactionCount,
		RedactionFailed:     input.RedactionFailed,
		RedactionFailureTag: input.RedactionFailureTag,
	})
	result := OutputResponseLifecycleResult{
		LifecycleDecision:      LifecycleDecision{Allowed: true},
		PolicyDecisionObserved: true,
		RedactionNeeded:        enforcement.RedactionNeeded && !input.RedactionAttempted,
		RedactionRejected:      enforcement.RedactionRejected,
	}
	if enforcement.Blocked || enforcement.RedactionRejected {
		result.Allowed = false
		result.Reason = enforcement.Reason
	}
	return result
}

// ClassifyUpstreamRequest classifies failures while projecting the canonical request into upstream format.
func (LifecycleUseCase) ClassifyUpstreamRequest(input UpstreamRequestInput) LifecycleDecision {
	if input.ProjectionFailed {
		return LifecycleDecision{Reason: RejectInvalidRequest}
	}
	return LifecycleDecision{Allowed: true}
}

// ClassifyUpstreamExchange decides whether upstream transport outcomes may continue into output handling.
func (LifecycleUseCase) ClassifyUpstreamExchange(input UpstreamExchangeInput) UpstreamExchangeResult {
	if input.ForwardFailed {
		return UpstreamExchangeResult{
			LifecycleDecision: LifecycleDecision{Reason: RejectUpstreamError},
		}
	}
	if input.Stream && input.StatusCode >= 400 {
		return UpstreamExchangeResult{
			LifecycleDecision: LifecycleDecision{Reason: RejectUpstreamError},
		}
	}
	return UpstreamExchangeResult{
		LifecycleDecision: LifecycleDecision{Allowed: true},
		Stream:            input.Stream,
	}
}

// ClassifyRedaction decides whether a redaction attempt produced a safe substitute payload.
func (LifecycleUseCase) ClassifyRedaction(input RedactionInput) LifecycleDecision {
	if input.Failed || input.Redactions == 0 && input.Text != "" {
		return LifecycleDecision{Reason: RejectRedactionFailed}
	}
	return LifecycleDecision{Allowed: true}
}

// ClassifyStreamingPolicy turns one incremental policy decision into a stream termination choice.
func (LifecycleUseCase) ClassifyStreamingPolicy(decision policy.Decision) StreamingLifecycleDecision {
	switch decision.Action {
	case policy.ActionBlock:
		return StreamingLifecycleDecision{Terminated: true, TerminationReason: "stream_blocked"}
	case policy.ActionRedact:
		return StreamingLifecycleDecision{Terminated: true, TerminationReason: "unsupported_redaction"}
	default:
		return StreamingLifecycleDecision{}
	}
}

// ClassifyStreamingCollection decides whether buffered output capacity requires terminating the stream.
func (LifecycleUseCase) ClassifyStreamingCollection(appended bool) StreamingLifecycleDecision {
	if !appended {
		return StreamingLifecycleDecision{Terminated: true, TerminationReason: "stream_output_too_large"}
	}
	return StreamingLifecycleDecision{}
}

// ClassifyStreamingDelta combines one streamed delta's policy and buffering outcomes into the next action.
func (u LifecycleUseCase) ClassifyStreamingDelta(input StreamingDeltaInput) StreamingLifecycleDecision {
	if !input.DeltaObserved {
		return StreamingLifecycleDecision{}
	}
	if policyDecision := u.ClassifyStreamingPolicy(input.PolicyDecision); policyDecision.Terminated {
		return policyDecision
	}
	return u.ClassifyStreamingCollection(input.CollectionAccepted)
}

func blockedReason(direction detection.Direction) RejectReason {
	switch direction {
	case detection.DirectionInput:
		return RejectBlockedInput
	case detection.DirectionOutput:
		return RejectBlockedOutput
	default:
		return RejectReason("blocked")
	}
}

func redactionReason(reason RejectReason, fallback string) RejectReason {
	if reason != "" {
		return reason
	}
	if fallback == "" {
		return RejectRedactionFailed
	}
	return RejectReason(fallback)
}
