package application

// UpstreamExchangeUseCase plans the transport-neutral lifecycle after upstream forwarding.
type UpstreamExchangeUseCase struct {
	Lifecycle LifecycleUseCase
}

// UpstreamExchangeAction identifies the next operation after upstream forwarding.
type UpstreamExchangeAction string

const (
	// UpstreamExchangeActionBlock means the upstream exchange must be rejected.
	UpstreamExchangeActionBlock UpstreamExchangeAction = "block"
	// UpstreamExchangeActionHandleStream means the adapter should handle a streaming response.
	UpstreamExchangeActionHandleStream UpstreamExchangeAction = "handle_stream"
	// UpstreamExchangeActionHandleResponse means the adapter should handle a non-streaming response.
	UpstreamExchangeActionHandleResponse UpstreamExchangeAction = "handle_response"
)

// UpstreamExchangePlan describes the next application-approved upstream action.
type UpstreamExchangePlan struct {
	Action UpstreamExchangeAction
	Reason RejectReason
}

// UpstreamForwardPlanInput contains upstream forwarding facts.
type UpstreamForwardPlanInput struct {
	ForwardFailed bool
}

// UpstreamResponsePlanInput contains upstream response facts.
type UpstreamResponsePlanInput struct {
	StatusCode int
	Stream     bool
}

// PlanForward decides whether an upstream forwarding attempt can continue to response handling.
func (u UpstreamExchangeUseCase) PlanForward(input UpstreamForwardPlanInput) UpstreamExchangePlan {
	upstream := u.Lifecycle.ClassifyUpstreamExchange(UpstreamExchangeInput{
		ForwardFailed: input.ForwardFailed,
	})
	if !upstream.Allowed {
		return blockUpstreamExchange(upstream.Reason)
	}
	return UpstreamExchangePlan{Action: UpstreamExchangeActionHandleResponse}
}

// PlanResponse decides how an upstream response should be handled.
func (u UpstreamExchangeUseCase) PlanResponse(input UpstreamResponsePlanInput) UpstreamExchangePlan {
	upstream := u.Lifecycle.ClassifyUpstreamExchange(UpstreamExchangeInput{
		StatusCode: input.StatusCode,
		Stream:     input.Stream,
	})
	if !upstream.Allowed {
		return blockUpstreamExchange(upstream.Reason)
	}
	if upstream.Stream {
		return UpstreamExchangePlan{Action: UpstreamExchangeActionHandleStream}
	}
	return UpstreamExchangePlan{Action: UpstreamExchangeActionHandleResponse}
}

func blockUpstreamExchange(reason RejectReason) UpstreamExchangePlan {
	return UpstreamExchangePlan{
		Action: UpstreamExchangeActionBlock,
		Reason: reason,
	}
}
