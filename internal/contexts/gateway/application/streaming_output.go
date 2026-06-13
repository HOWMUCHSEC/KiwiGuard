package application

import "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"

// StreamingOutputUseCase plans the transport-neutral lifecycle for streaming output.
type StreamingOutputUseCase struct {
	Lifecycle LifecycleUseCase
	Buffer    *StreamBuffer
}

// StreamingOutputAction identifies the next protocol operation for a streaming output.
type StreamingOutputAction string

const (
	// StreamingOutputActionContinue means no protocol write is needed yet.
	StreamingOutputActionContinue StreamingOutputAction = "continue"
	// StreamingOutputActionEvaluatePolicy means the adapter should evaluate output policy.
	StreamingOutputActionEvaluatePolicy StreamingOutputAction = "evaluate_policy"
	// StreamingOutputActionWritePending means the adapter may write the held stream frame.
	StreamingOutputActionWritePending StreamingOutputAction = "write_pending"
	// StreamingOutputActionTerminate means the stream must be terminated.
	StreamingOutputActionTerminate StreamingOutputAction = "terminate"
	// StreamingOutputActionComplete means the stream completed normally.
	StreamingOutputActionComplete StreamingOutputAction = "complete"
	// StreamingOutputActionEmit means the adapter should emit the final traffic event.
	StreamingOutputActionEmit StreamingOutputAction = "emit"
)

// StreamingOutputPlan describes the next application-approved streaming action.
type StreamingOutputPlan struct {
	Action             StreamingOutputAction
	PolicyText         string
	CollectionAccepted bool
	ForceBlockedOutput bool
	TerminationReason  string
}

// StreamingDeltaPlanInput contains one protocol-decoded streaming output delta.
type StreamingDeltaPlanInput struct {
	Text string
}

// StreamingPolicyPlanInput contains policy facts for the current stream window.
type StreamingPolicyPlanInput struct {
	Decision           policy.Decision
	CollectionAccepted bool
}

// StreamingFramePlanInput contains transport-neutral frame progression facts.
type StreamingFramePlanInput struct {
	HasPending bool
	Done       bool
}

// StreamingFinalPlanInput contains final stream evaluation facts.
type StreamingFinalPlanInput struct {
	PolicyAllowed bool
	CollectedText string
}

// PlanDelta records a decoded stream delta and decides whether policy evaluation is needed.
func (u StreamingOutputUseCase) PlanDelta(input StreamingDeltaPlanInput) StreamingOutputPlan {
	if input.Text == "" {
		return StreamingOutputPlan{Action: StreamingOutputActionContinue}
	}
	observation := u.Buffer.ObserveDelta(input.Text)
	return StreamingOutputPlan{
		Action:             StreamingOutputActionEvaluatePolicy,
		PolicyText:         observation.WindowText,
		CollectionAccepted: observation.CollectionAccepted,
	}
}

// PlanPolicy decides whether an observed stream window may continue.
func (u StreamingOutputUseCase) PlanPolicy(input StreamingPolicyPlanInput) StreamingOutputPlan {
	streaming := u.Lifecycle.ClassifyStreamingDelta(StreamingDeltaInput{
		DeltaObserved:      true,
		PolicyDecision:     input.Decision,
		CollectionAccepted: input.CollectionAccepted,
	})
	if streaming.Terminated {
		return StreamingOutputPlan{
			Action:             StreamingOutputActionTerminate,
			ForceBlockedOutput: !input.CollectionAccepted,
			TerminationReason:  streaming.TerminationReason,
		}
	}
	return StreamingOutputPlan{Action: StreamingOutputActionContinue}
}

// PlanFrame decides whether a held frame can be written or the stream is complete.
func (StreamingOutputUseCase) PlanFrame(input StreamingFramePlanInput) StreamingOutputPlan {
	if input.Done {
		return StreamingOutputPlan{Action: StreamingOutputActionComplete}
	}
	if input.HasPending {
		return StreamingOutputPlan{Action: StreamingOutputActionWritePending}
	}
	return StreamingOutputPlan{Action: StreamingOutputActionContinue}
}

// PlanFinal decides whether a completed stream needs final full-output policy evaluation.
func (StreamingOutputUseCase) PlanFinal(input StreamingFinalPlanInput) StreamingOutputPlan {
	if input.PolicyAllowed && input.CollectedText != "" {
		return StreamingOutputPlan{
			Action:     StreamingOutputActionEvaluatePolicy,
			PolicyText: input.CollectedText,
		}
	}
	return StreamingOutputPlan{Action: StreamingOutputActionEmit}
}
