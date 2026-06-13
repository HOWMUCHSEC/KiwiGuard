package application

import (
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestInputRequestUseCasePlansPolicy(t *testing.T) {
	useCase := InputRequestUseCase{}

	blocked := useCase.PlanPolicy(InputPolicyInput{
		Decision: policy.Decision{Action: policy.ActionBlock},
	})
	assertInputPlan(t, blocked, InputRequestActionBlock, RejectBlockedInput, "")

	redact := useCase.PlanPolicy(InputPolicyInput{
		Decision: policy.Decision{Action: policy.ActionRedact},
	})
	assertInputPlan(t, redact, InputRequestActionRedact, "", "")

	allowed := useCase.PlanPolicy(InputPolicyInput{
		Decision: policy.Decision{Action: policy.ActionAllow},
	})
	assertInputPlan(t, allowed, InputRequestActionSend, "", "")
}

func TestInputRequestUseCasePlansRedaction(t *testing.T) {
	useCase := InputRequestUseCase{}

	failed := useCase.PlanRedaction(InputRedactionInput{
		Decision: policy.Decision{Action: policy.ActionRedact},
		Text:     "secret",
		Failed:   true,
	})
	assertInputPlan(t, failed, InputRequestActionBlock, RejectRedactionFailed, "redaction_failed")

	empty := useCase.PlanRedaction(InputRedactionInput{
		Decision: policy.Decision{Action: policy.ActionRedact},
		Text:     "secret",
	})
	assertInputPlan(t, empty, InputRequestActionBlock, RejectRedactionFailed, "redaction_failed")

	allowed := useCase.PlanRedaction(InputRedactionInput{
		Decision:   policy.Decision{Action: policy.ActionRedact},
		Text:       "secret",
		Redactions: 1,
	})
	assertInputPlan(t, allowed, InputRequestActionSend, "", "")
}

func assertInputPlan(t *testing.T, got InputRequestPlan, action InputRequestAction, reason RejectReason, termination string) {
	t.Helper()
	if got.Action != action {
		t.Fatalf("Action = %q, want %q", got.Action, action)
	}
	if got.Reason != reason {
		t.Fatalf("Reason = %q, want %q", got.Reason, reason)
	}
	if got.TerminationReason != termination {
		t.Fatalf("TerminationReason = %q, want %q", got.TerminationReason, termination)
	}
}
