package application

// ExchangePreparationUseCase plans transport-neutral gateway exchange preparation.
type ExchangePreparationUseCase struct {
	Lifecycle LifecycleUseCase
}

// ExchangePreparationAction identifies the next operation while preparing an exchange.
type ExchangePreparationAction string

const (
	// ExchangePreparationActionBlock means preparation must reject the request.
	ExchangePreparationActionBlock ExchangePreparationAction = "block"
	// ExchangePreparationActionContinue means preparation can proceed.
	ExchangePreparationActionContinue ExchangePreparationAction = "continue"
)

// ExchangePreparationPlan describes the next application-approved preparation action.
type ExchangePreparationPlan struct {
	Action         ExchangePreparationAction
	Reason         RejectReason
	RequestedModel string
	MappedModel    string
	UpstreamModel  string
}

// RouteAvailabilityInput contains route lookup facts.
type RouteAvailabilityInput struct {
	Found bool
}

// ExchangeInfrastructurePlanInput contains provider and audit readiness facts.
type ExchangeInfrastructurePlanInput struct {
	ProviderFound bool
	AuditHealthy  bool
}

// DecodedRequestPlanInput contains request decoding and model mapping facts.
type DecodedRequestPlanInput struct {
	DecodeFailed bool
	ModelMapping ModelMappingInput
}

// PlanRouteAvailability decides whether a route lookup can proceed.
func (u ExchangePreparationUseCase) PlanRouteAvailability(input RouteAvailabilityInput) ExchangePreparationPlan {
	decision := u.Lifecycle.RouteAvailable(input.Found)
	if !decision.Allowed {
		return blockExchangePreparation(decision.Reason)
	}
	return ExchangePreparationPlan{Action: ExchangePreparationActionContinue}
}

// PlanInfrastructure decides whether provider and audit dependencies can support an exchange.
func (u ExchangePreparationUseCase) PlanInfrastructure(input ExchangeInfrastructurePlanInput) ExchangePreparationPlan {
	decision := u.Lifecycle.ClassifyExchangeInfrastructure(ExchangeInfrastructureInput(input))
	if !decision.Allowed {
		return blockExchangePreparation(decision.Reason)
	}
	return ExchangePreparationPlan{Action: ExchangePreparationActionContinue}
}

// PlanDecodedRequest normalizes request decoding and model mapping outcomes.
func (u ExchangePreparationUseCase) PlanDecodedRequest(input DecodedRequestPlanInput) ExchangePreparationPlan {
	models := u.Lifecycle.ClassifyDecodedRequest(DecodedRequestInput(input))
	plan := ExchangePreparationPlan{
		Action:         ExchangePreparationActionContinue,
		RequestedModel: models.RequestedModel,
		MappedModel:    models.MappedModel,
		UpstreamModel:  models.UpstreamModel,
	}
	if !models.Allowed {
		plan.Action = ExchangePreparationActionBlock
		plan.Reason = models.Reason
	}
	return plan
}

func blockExchangePreparation(reason RejectReason) ExchangePreparationPlan {
	return ExchangePreparationPlan{
		Action: ExchangePreparationActionBlock,
		Reason: reason,
	}
}
