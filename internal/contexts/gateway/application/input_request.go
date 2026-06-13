package application

import "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"

// InputRequestUseCase plans the transport-neutral lifecycle for gateway input.
type InputRequestUseCase struct {
	Lifecycle LifecycleUseCase
}

// InputRequestAction identifies the next protocol operation for an input request.
type InputRequestAction string

const (
	// InputRequestActionBlock means the request must be rejected.
	InputRequestActionBlock InputRequestAction = "block"
	// InputRequestActionRedact means the adapter should apply protocol-specific redaction.
	InputRequestActionRedact InputRequestAction = "redact"
	// InputRequestActionSend means the request can continue to upstream forwarding.
	InputRequestActionSend InputRequestAction = "send"
)

// InputRequestPlan describes the next application-approved input action.
type InputRequestPlan struct {
	Action            InputRequestAction
	Reason            RejectReason
	TerminationReason string
}

// InputPolicyInput contains input policy facts for a request.
type InputPolicyInput struct {
	Decision policy.Decision
}

// InputRedactionInput contains input redaction facts for a request.
type InputRedactionInput struct {
	Decision   policy.Decision
	Text       string
	Redactions int
	Failed     bool
}

// PlanPolicy decides whether a policy-evaluated input should be blocked, redacted, or sent.
func (u InputRequestUseCase) PlanPolicy(input InputPolicyInput) InputRequestPlan {
	lifecycle := u.Lifecycle.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated: true,
		Decision:        input.Decision,
	})
	if !lifecycle.Allowed {
		return blockInputRequest(lifecycle.Reason, "")
	}
	if lifecycle.RedactionNeeded {
		return InputRequestPlan{Action: InputRequestActionRedact}
	}
	return InputRequestPlan{Action: InputRequestActionSend}
}

// PlanRedaction decides whether a redacted input can continue to upstream forwarding.
func (u InputRequestUseCase) PlanRedaction(input InputRedactionInput) InputRequestPlan {
	lifecycle := u.Lifecycle.ClassifyInputRequest(InputRequestLifecycleInput{
		PolicyEvaluated:    true,
		Decision:           input.Decision,
		RedactionAttempted: true,
		RedactionText:      input.Text,
		RedactionCount:     input.Redactions,
		RedactionFailed:    input.Failed,
	})
	if !lifecycle.Allowed {
		return blockInputRequest(lifecycle.Reason, "redaction_failed")
	}
	return InputRequestPlan{Action: InputRequestActionSend}
}

func blockInputRequest(reason RejectReason, termination string) InputRequestPlan {
	return InputRequestPlan{
		Action:            InputRequestActionBlock,
		Reason:            reason,
		TerminationReason: termination,
	}
}
