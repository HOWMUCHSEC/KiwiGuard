package application

import "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"

// OutputResponseUseCase plans the transport-neutral lifecycle for non-streaming upstream output.
type OutputResponseUseCase struct {
	Lifecycle LifecycleUseCase
}

// OutputResponseAction identifies the next protocol operation for a non-streaming output.
type OutputResponseAction string

const (
	// OutputResponseActionBlock means the response must be rejected.
	OutputResponseActionBlock OutputResponseAction = "block"
	// OutputResponseActionExtractText means the adapter should extract policy text from the payload.
	OutputResponseActionExtractText OutputResponseAction = "extract_text"
	// OutputResponseActionEvaluatePolicy means the adapter should evaluate output policy.
	OutputResponseActionEvaluatePolicy OutputResponseAction = "evaluate_policy"
	// OutputResponseActionRedact means the adapter should apply protocol-specific redaction.
	OutputResponseActionRedact OutputResponseAction = "redact"
	// OutputResponseActionSend means the adapter can send the output to the client.
	OutputResponseActionSend OutputResponseAction = "send"
)

// OutputResponsePlan describes the next application-approved output action.
type OutputResponsePlan struct {
	Action             OutputResponseAction
	Reason             RejectReason
	EmitPolicyDecision bool
	TerminationReason  string
}

// OutputBodyReadInput contains upstream body read facts for a non-streaming response.
type OutputBodyReadInput struct {
	StatusCode int
	ReadFailed bool
	TooLarge   bool
}

// OutputExtractionInput contains protocol extraction facts for a non-streaming response.
type OutputExtractionInput struct {
	StatusCode   int
	DecodeFailed bool
}

// OutputPolicyInput contains output policy facts for a non-streaming response.
type OutputPolicyInput struct {
	StatusCode int
	Decision   policy.Decision
}

// OutputRedactionInput contains output redaction facts for a non-streaming response.
type OutputRedactionInput struct {
	StatusCode int
	Decision   policy.Decision
	Text       string
	Redactions int
	Failed     bool
}

// PlanBodyRead decides whether a read upstream body can move to protocol extraction.
func (u OutputResponseUseCase) PlanBodyRead(input OutputBodyReadInput) OutputResponsePlan {
	lifecycle := u.Lifecycle.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{
			StatusCode: input.StatusCode,
			ReadFailed: input.ReadFailed,
			TooLarge:   input.TooLarge,
		},
	})
	if !lifecycle.Allowed {
		return blockOutputResponse(lifecycle.Reason, false, string(lifecycle.Reason))
	}
	return OutputResponsePlan{Action: OutputResponseActionExtractText}
}

// PlanExtraction decides whether extracted response data can move to policy evaluation.
func (u OutputResponseUseCase) PlanExtraction(input OutputExtractionInput) OutputResponsePlan {
	lifecycle := u.Lifecycle.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{
			StatusCode:   input.StatusCode,
			DecodeFailed: input.DecodeFailed,
		},
	})
	if !lifecycle.Allowed {
		return blockOutputResponse(lifecycle.Reason, false, string(lifecycle.Reason))
	}
	return OutputResponsePlan{Action: OutputResponseActionEvaluatePolicy}
}

// PlanPolicy decides whether a policy-evaluated output should be blocked, redacted, or sent.
func (u OutputResponseUseCase) PlanPolicy(input OutputPolicyInput) OutputResponsePlan {
	lifecycle := u.Lifecycle.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: input.StatusCode},
		PolicyEvaluated:       true,
		Decision:              input.Decision,
	})
	if !lifecycle.Allowed {
		return blockOutputResponse(lifecycle.Reason, true, "")
	}
	if lifecycle.RedactionNeeded {
		return OutputResponsePlan{Action: OutputResponseActionRedact, EmitPolicyDecision: true}
	}
	return OutputResponsePlan{Action: OutputResponseActionSend, EmitPolicyDecision: true}
}

// PlanRedaction decides whether a redacted output can be sent.
func (u OutputResponseUseCase) PlanRedaction(input OutputRedactionInput) OutputResponsePlan {
	lifecycle := u.Lifecycle.ClassifyOutputResponse(OutputResponseLifecycleInput{
		UpstreamResponseInput: UpstreamResponseInput{StatusCode: input.StatusCode},
		PolicyEvaluated:       true,
		Decision:              input.Decision,
		RedactionAttempted:    true,
		RedactionText:         input.Text,
		RedactionCount:        input.Redactions,
		RedactionFailed:       input.Failed,
	})
	if !lifecycle.Allowed {
		return blockOutputResponse(lifecycle.Reason, true, "redaction_failed")
	}
	return OutputResponsePlan{Action: OutputResponseActionSend, EmitPolicyDecision: true}
}

func blockOutputResponse(reason RejectReason, emitPolicyDecision bool, termination string) OutputResponsePlan {
	return OutputResponsePlan{
		Action:             OutputResponseActionBlock,
		Reason:             reason,
		EmitPolicyDecision: emitPolicyDecision,
		TerminationReason:  termination,
	}
}
